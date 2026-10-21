package postgres

import (
	"context"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/felipemalacarne/mesa/internal/core/errs"
	"github.com/felipemalacarne/mesa/internal/core/ident"
	"github.com/felipemalacarne/mesa/internal/core/value"
	"github.com/felipemalacarne/mesa/internal/engine"
)

const (
	// defaultMaxRows bounds a Query that asks for the engine default.
	defaultMaxRows = 1000
	// txMaxRows bounds a Tx.Query, which has no options.
	txMaxRows = 10000
	// defaultPageSize is the TableRows limit when none is given.
	defaultPageSize = 100
)

// Query implements engine.Exec.
func (s *Session) Query(ctx context.Context, sql string, args []value.Value, opts engine.QueryOptions) (*engine.Result, error) {
	maxRows := opts.MaxRows
	if maxRows <= 0 {
		maxRows = defaultMaxRows
	}
	if opts.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, opts.Timeout)
		defer cancel()
	}
	conn, err := s.pool.Acquire(ctx)
	if err != nil {
		return nil, wrap(err, "no connection available")
	}
	defer conn.Release()
	pc := conn.Conn().PgConn()
	if !opts.ReadOnly {
		return s.run(ctx, pc, sql, args, maxRows)
	}
	tx, err := conn.BeginTx(ctx, pgx.TxOptions{AccessMode: pgx.ReadOnly})
	if err != nil {
		return nil, wrap(err, "cannot begin a read-only transaction")
	}
	defer func() { _ = tx.Rollback(context.WithoutCancel(ctx)) }()
	return s.run(ctx, pc, sql, args, maxRows)
}

// Explain implements engine.Exec. With analyze the statement runs inside a
// transaction that is always rolled back, so explaining a write is safe.
func (s *Session) Explain(ctx context.Context, sql string, args []value.Value, analyze bool) ([]string, error) {
	conn, err := s.pool.Acquire(ctx)
	if err != nil {
		return nil, wrap(err, "no connection available")
	}
	defer conn.Release()
	pc := conn.Conn().PgConn()
	prefix := "EXPLAIN (FORMAT TEXT) "
	if analyze {
		prefix = "EXPLAIN (ANALYZE, FORMAT TEXT) "
		tx, err := conn.Begin(ctx)
		if err != nil {
			return nil, wrap(err, "cannot begin a transaction")
		}
		defer func() { _ = tx.Rollback(context.WithoutCancel(ctx)) }()
	}
	res, err := s.run(ctx, pc, prefix+strings.TrimRight(strings.TrimSpace(sql), ";"), args, txMaxRows)
	if err != nil {
		return nil, err
	}
	lines := make([]string, 0, len(res.Rows))
	for _, row := range res.Rows {
		if len(row) > 0 {
			lines = append(lines, row[0].Text())
		}
	}
	return lines, nil
}

// TableRows implements engine.Exec.
func (s *Session) TableRows(ctx context.Context, q engine.RowsQuery) (*engine.RowsPage, error) {
	if q.Table.Name == "" {
		return nil, errs.New(errs.Invalid, "table name is required")
	}
	cols, err := s.columnNames(ctx, q.Table)
	if err != nil {
		return nil, err
	}
	where, args, err := whereClause(q.Filters)
	if err != nil {
		return nil, err
	}
	limit := q.Limit
	if limit <= 0 {
		limit = defaultPageSize
	}
	offset := q.Offset
	if offset < 0 {
		offset = 0
	}

	var b strings.Builder
	b.WriteString("SELECT ")
	for i, c := range cols {
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteString(c.Quote())
	}
	b.WriteString(" FROM " + q.Table.Quote() + where)
	if len(q.Sort) > 0 {
		b.WriteString(" ORDER BY ")
		for i, srt := range q.Sort {
			if i > 0 {
				b.WriteString(", ")
			}
			b.WriteString(srt.Column.Quote())
			if srt.Desc {
				b.WriteString(" DESC")
			}
		}
	}
	pageArgs := append(append([]value.Value{}, args...), value.Int(int64(limit)), value.Int(int64(offset)))
	b.WriteString(" LIMIT $" + strconv.Itoa(len(args)+1) + " OFFSET $" + strconv.Itoa(len(args)+2))

	conn, err := s.pool.Acquire(ctx)
	if err != nil {
		return nil, wrap(err, "no connection available")
	}
	defer conn.Release()
	pc := conn.Conn().PgConn()
	res, err := s.run(ctx, pc, b.String(), pageArgs, limit)
	if err != nil {
		return nil, err
	}
	count, err := s.run(ctx, pc, "SELECT count(*) FROM "+q.Table.Quote()+where, args, 1)
	if err != nil {
		return nil, err
	}
	page := &engine.RowsPage{Result: res}
	if len(count.Rows) == 1 && len(count.Rows[0]) == 1 {
		page.Total = count.Rows[0][0].AsInt()
	}
	return page, nil
}

// whereClause renders filters as " WHERE …" with $n placeholders and the
// matching arguments. Comparisons bind the value in the column's own type;
// pattern matches compare the text form so they work on any column.
func whereClause(filters []engine.Filter) (string, []value.Value, error) {
	if len(filters) == 0 {
		return "", nil, nil
	}
	var parts []string
	var args []value.Value
	for _, f := range filters {
		if f.Column == "" {
			return "", nil, errs.New(errs.Invalid, "filter has no column")
		}
		col := f.Column.Quote()
		bind := func() string {
			args = append(args, f.Value)
			return "$" + strconv.Itoa(len(args))
		}
		switch f.Op {
		case engine.OpEq:
			parts = append(parts, col+" = "+bind())
		case engine.OpNeq:
			parts = append(parts, col+" <> "+bind())
		case engine.OpLt:
			parts = append(parts, col+" < "+bind())
		case engine.OpLte:
			parts = append(parts, col+" <= "+bind())
		case engine.OpGt:
			parts = append(parts, col+" > "+bind())
		case engine.OpGte:
			parts = append(parts, col+" >= "+bind())
		case engine.OpLike:
			parts = append(parts, col+"::text LIKE "+bind())
		case engine.OpILike:
			parts = append(parts, col+"::text ILIKE "+bind())
		case engine.OpIsNull:
			parts = append(parts, col+" IS NULL")
		case engine.OpNotNull:
			parts = append(parts, col+" IS NOT NULL")
		default:
			return "", nil, errs.Newf(errs.Invalid, "unknown filter operator %q", f.Op)
		}
	}
	return " WHERE " + strings.Join(parts, " AND "), args, nil
}

// Tx implements engine.Exec.
func (s *Session) Tx(ctx context.Context, fn func(engine.Tx) error) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return wrap(err, "cannot begin a transaction")
	}
	if err := fn(&transaction{s: s, tx: tx}); err != nil {
		_ = tx.Rollback(context.WithoutCancel(ctx))
		return err
	}
	return wrap(tx.Commit(ctx), "commit failed")
}

// transaction runs statements on the connection of one pgx transaction.
type transaction struct {
	s  *Session
	tx pgx.Tx
}

func (t *transaction) Exec(ctx context.Context, sql string, args []value.Value) (int64, error) {
	res, err := t.s.run(ctx, t.tx.Conn().PgConn(), sql, args, 0)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected, nil
}

func (t *transaction) Query(ctx context.Context, sql string, args []value.Value) (*engine.Result, error) {
	return t.s.run(ctx, t.tx.Conn().PgConn(), sql, args, txMaxRows)
}

// rawResult is one statement's outcome before decoding.
type rawResult struct {
	fields    []pgconn.FieldDescription
	rows      [][][]byte
	truncated bool
	tag       pgconn.CommandTag
}

// run executes sql on pc and keeps up to maxRows rows. Without arguments it
// uses the simple protocol, so a script of several statements works and the
// last result is returned; with arguments it binds them in the text format.
func (s *Session) run(ctx context.Context, pc *pgconn.PgConn, sql string, args []value.Value, maxRows int) (*engine.Result, error) {
	started := time.Now()
	var raw rawResult
	var err error
	if len(args) == 0 {
		raw, err = runSimple(ctx, pc, sql, maxRows)
	} else {
		rr := pc.ExecParams(ctx, sql, encodeArgs(args), nil, nil, nil)
		raw, err = collect(rr, maxRows)
	}
	if err != nil {
		return nil, wrap(err, "query failed")
	}
	res := &engine.Result{
		Duration:     time.Since(started),
		Truncated:    raw.truncated,
		RowsAffected: raw.tag.RowsAffected(),
		Command:      commandOf(raw.tag),
		Columns:      make([]value.Column, len(raw.fields)),
		Rows:         make([]value.Row, len(raw.rows)),
	}
	infos := make([]typeInfo, len(raw.fields))
	for i, f := range raw.fields {
		infos[i] = s.types.lookup(ctx, pc, f.DataTypeOID)
		res.Columns[i] = value.Column{
			Name: f.Name,
			Type: s.types.displayName(ctx, pc, infos[i]),
			Kind: infos[i].kind(),
		}
	}
	for ri, row := range raw.rows {
		out := make(value.Row, len(row))
		for ci, cell := range row {
			out[ci] = s.types.decode(ctx, pc, infos[ci], cell)
		}
		res.Rows[ri] = out
	}
	return res, nil
}

func runSimple(ctx context.Context, pc *pgconn.PgConn, sql string, maxRows int) (rawResult, error) {
	mrr := pc.Exec(ctx, sql)
	var last rawResult
	var seen bool
	for mrr.NextResult() {
		r, err := collect(mrr.ResultReader(), maxRows)
		if err != nil {
			_ = mrr.Close()
			return rawResult{}, err
		}
		// Prefer the last statement that produced columns, else the last one.
		if !seen || len(r.fields) > 0 || len(last.fields) == 0 {
			last = r
		}
		seen = true
	}
	if err := mrr.Close(); err != nil {
		return rawResult{}, err
	}
	return last, nil
}

// collect drains a result reader, keeping the first maxRows rows.
func collect(rr *pgconn.ResultReader, maxRows int) (rawResult, error) {
	var out rawResult
	for rr.NextRow() {
		if len(out.rows) >= maxRows {
			out.truncated = true
			continue
		}
		out.rows = append(out.rows, copyRow(rr.Values()))
	}
	// Field descriptions are only valid until Close, so copy them first.
	out.fields = append([]pgconn.FieldDescription(nil), rr.FieldDescriptions()...)
	tag, err := rr.Close()
	if err != nil {
		return rawResult{}, err
	}
	out.tag = tag
	return out, nil
}

// commandOf strips the counters from a command tag: "INSERT 0 3" → "INSERT".
func commandOf(tag pgconn.CommandTag) string {
	words := strings.Fields(tag.String())
	for len(words) > 0 {
		if _, err := strconv.Atoi(words[len(words)-1]); err != nil {
			break
		}
		words = words[:len(words)-1]
	}
	return strings.Join(words, " ")
}

// columnNames returns the table's columns in their declared order, or
// NotFound when the table does not exist.
func (s *Session) columnNames(ctx context.Context, q ident.Qualified) ([]ident.Name, error) {
	rows, err := s.pool.Query(ctx, columnNamesSQL, string(q.Schema), string(q.Name))
	if err != nil {
		return nil, wrap(err, "cannot read columns of "+q.String())
	}
	defer rows.Close()
	var cols []ident.Name
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, wrap(err, "cannot read columns of "+q.String())
		}
		cols = append(cols, ident.Name(name))
	}
	if err := rows.Err(); err != nil {
		return nil, wrap(err, "cannot read columns of "+q.String())
	}
	if len(cols) == 0 {
		return nil, errs.Newf(errs.NotFound, "table %s not found", q)
	}
	return cols, nil
}
