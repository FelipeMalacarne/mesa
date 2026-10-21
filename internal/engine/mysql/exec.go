package mysql

import (
	"context"
	"database/sql"
	sqldriver "database/sql/driver"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/felipemalacarne/mesa/internal/core/changeset"
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

// queryer is what a pooled connection and a transaction have in common.
type queryer interface {
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}

// Query implements engine.Exec. The driver runs one statement per call, so
// a script is split and run statement by statement on one connection; the
// last result with columns is returned.
//
// Read-only is enforced three ways, because MySQL lets DDL commit a
// read-only transaction implicitly: only reading statements are accepted,
// the session is marked transaction_read_only, and the statements run in a
// read-only transaction.
func (s *Session) Query(ctx context.Context, script string, args []value.Value, opts engine.QueryOptions) (*engine.Result, error) {
	maxRows := opts.MaxRows
	if maxRows <= 0 {
		maxRows = defaultMaxRows
	}
	stmts := []string{script}
	if len(args) == 0 {
		stmts = statements(script)
	}
	if len(stmts) == 0 {
		return nil, errs.New(errs.Invalid, "nothing to run")
	}
	if opts.ReadOnly {
		for _, st := range stmts {
			if err := readingStatement(st); err != nil {
				return nil, err
			}
		}
	}
	if opts.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, opts.Timeout)
		defer cancel()
	}
	conn, err := s.db.Conn(ctx)
	if err != nil {
		return nil, wrap(err, "no connection available")
	}
	defer func() { _ = conn.Close() }()
	if !opts.ReadOnly {
		return s.runAll(ctx, conn, stmts, args, maxRows)
	}

	if _, err := conn.ExecContext(ctx, "SET SESSION transaction_read_only = ON"); err != nil {
		return nil, wrap(err, "cannot make the session read-only")
	}
	if opts.Timeout > 0 {
		_, _ = conn.ExecContext(ctx, "SET SESSION max_execution_time = "+strconv.FormatInt(opts.Timeout.Milliseconds(), 10))
	}
	defer func() {
		// Put the connection back as it was, or drop it from the pool.
		reset := context.WithoutCancel(ctx)
		if _, err := conn.ExecContext(reset, "SET SESSION transaction_read_only = OFF, max_execution_time = 0"); err != nil {
			_ = conn.Raw(func(any) error { return sqldriver.ErrBadConn })
		}
	}()
	tx, err := conn.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return nil, wrap(err, "cannot begin a read-only transaction")
	}
	defer func() { _ = tx.Rollback() }()
	return s.runAll(ctx, tx, stmts, args, maxRows)
}

func (s *Session) runAll(ctx context.Context, q queryer, stmts []string, args []value.Value, maxRows int) (*engine.Result, error) {
	var last *engine.Result
	for _, st := range stmts {
		res, err := s.run(ctx, q, st, args, maxRows)
		if err != nil {
			return nil, err
		}
		if last == nil || len(res.Columns) > 0 || len(last.Columns) == 0 {
			last = res
		}
	}
	return last, nil
}

// statements splits a script into statements, dropping empty ones.
func statements(script string) []string {
	var out []string
	for _, st := range changeset.SplitScript(script) {
		if st = strings.TrimSpace(st); st != "" {
			out = append(out, st)
		}
	}
	return out
}

// readingKeywords start the statements a read-only query may run.
var readingKeywords = map[string]bool{
	"SELECT": true, "WITH": true, "SHOW": true, "DESCRIBE": true, "DESC": true,
	"EXPLAIN": true, "TABLE": true, "VALUES": true, "HELP": true,
}

var writesFile = regexp.MustCompile(`(?i)\binto\s+(outfile|dumpfile)\b`)

// readingStatement rejects anything but a read.
func readingStatement(st string) error {
	kw := strings.ToUpper(firstKeyword(st))
	if !readingKeywords[kw] {
		return errs.Newf(errs.Forbidden, "%s is not allowed in a read-only query: make changes through a reviewed change set", kw)
	}
	if writesFile.MatchString(st) {
		return errs.New(errs.Forbidden, "writing query results to a file is not allowed")
	}
	if kw == "EXPLAIN" && strings.Contains(strings.ToUpper(st), "ANALYZE") {
		return errs.New(errs.Forbidden, "EXPLAIN ANALYZE runs the statement; use the explain action instead")
	}
	return nil
}

// firstKeyword returns the first word of a statement after comments and
// opening parentheses.
func firstKeyword(st string) string {
	s := strings.TrimSpace(st)
	for {
		switch {
		case strings.HasPrefix(s, "--"), strings.HasPrefix(s, "#"):
			if i := strings.IndexByte(s, '\n'); i >= 0 {
				s = strings.TrimSpace(s[i+1:])
				continue
			}
			return ""
		case strings.HasPrefix(s, "/*"):
			if i := strings.Index(s, "*/"); i >= 0 {
				s = strings.TrimSpace(s[i+2:])
				continue
			}
			return ""
		case strings.HasPrefix(s, "("):
			s = strings.TrimSpace(s[1:])
			continue
		}
		break
	}
	end := strings.IndexFunc(s, func(r rune) bool {
		return (r < 'a' || r > 'z') && (r < 'A' || r > 'Z') && r != '_'
	})
	if end < 0 {
		return s
	}
	return s[:end]
}

// ddlKeywords start statements that MySQL commits implicitly.
var ddlKeywords = map[string]bool{"CREATE": true, "ALTER": true, "DROP": true, "RENAME": true, "TRUNCATE": true, "GRANT": true, "REVOKE": true}

// Explain implements engine.Exec. EXPLAIN ANALYZE runs the statement, so it
// runs inside a transaction that is rolled back.
func (s *Session) Explain(ctx context.Context, query string, args []value.Value, analyze bool) ([]string, error) {
	query = strings.TrimRight(strings.TrimSpace(query), ";")
	prefix := "EXPLAIN FORMAT=TREE "
	if analyze {
		prefix = "EXPLAIN ANALYZE "
	}
	var res *engine.Result
	err := s.rollbackTx(ctx, func(tx *sql.Tx) error {
		var err error
		res, err = s.run(ctx, tx, prefix+query, args, txMaxRows)
		if err != nil && !analyze {
			// Servers before 8.0.16 have no tree format.
			res, err = s.run(ctx, tx, "EXPLAIN "+query, args, txMaxRows)
		}
		return err
	})
	if err != nil {
		return nil, err
	}
	var lines []string
	if len(res.Columns) == 1 {
		for _, row := range res.Rows {
			lines = append(lines, strings.Split(strings.TrimRight(row[0].Text(), "\n"), "\n")...)
		}
		return lines, nil
	}
	// Traditional EXPLAIN: one line per row, "column=value" pairs.
	for _, row := range res.Rows {
		var parts []string
		for i, c := range res.Columns {
			if !row[i].IsNull() {
				parts = append(parts, c.Name+"="+row[i].Text())
			}
		}
		lines = append(lines, strings.Join(parts, " "))
	}
	return lines, nil
}

func (s *Session) rollbackTx(ctx context.Context, fn func(*sql.Tx) error) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return wrap(err, "cannot begin a transaction")
	}
	defer func() { _ = tx.Rollback() }()
	return fn(tx)
}

// TableRows implements engine.Exec.
func (s *Session) TableRows(ctx context.Context, q engine.RowsQuery) (*engine.RowsPage, error) {
	if q.Table.Name == "" {
		return nil, errs.New(errs.Invalid, "table name is required")
	}
	q.Table = s.qualify(q.Table)
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
	offset := max(q.Offset, 0)

	var b strings.Builder
	b.WriteString("SELECT ")
	for i, c := range cols {
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteString(quote(c))
	}
	b.WriteString(" FROM " + table(q.Table) + where)
	if len(q.Sort) > 0 {
		b.WriteString(" ORDER BY ")
		for i, srt := range q.Sort {
			if i > 0 {
				b.WriteString(", ")
			}
			b.WriteString(quote(srt.Column))
			if srt.Desc {
				b.WriteString(" DESC")
			}
		}
	}
	b.WriteString(" LIMIT " + strconv.Itoa(limit) + " OFFSET " + strconv.Itoa(offset))

	res, err := s.run(ctx, s.db, b.String(), args, limit)
	if err != nil {
		return nil, err
	}
	count, err := s.run(ctx, s.db, "SELECT count(*) FROM "+table(q.Table)+where, args, 1)
	if err != nil {
		return nil, err
	}
	page := &engine.RowsPage{Result: res}
	if len(count.Rows) == 1 && len(count.Rows[0]) == 1 {
		page.Total = count.Rows[0][0].AsInt()
	}
	return page, nil
}

// whereClause renders filters as " WHERE …" with ? placeholders and the
// matching arguments. Pattern matches compare the text form of the column.
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
		col := quote(f.Column)
		bind := func() string {
			args = append(args, f.Value)
			return "?"
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
			parts = append(parts, "CAST("+col+" AS CHAR) LIKE BINARY "+bind())
		case engine.OpILike:
			parts = append(parts, "LOWER(CAST("+col+" AS CHAR)) LIKE LOWER("+bind()+")")
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

// Tx implements engine.Exec. MySQL commits DDL implicitly; after each DDL
// statement a new transaction is started so the statements that follow can
// still be rolled back together.
func (s *Session) Tx(ctx context.Context, fn func(engine.Tx) error) error {
	conn, err := s.db.Conn(ctx)
	if err != nil {
		return wrap(err, "no connection available")
	}
	defer func() { _ = conn.Close() }()
	if _, err := conn.ExecContext(ctx, "START TRANSACTION"); err != nil {
		return wrap(err, "cannot begin a transaction")
	}
	if err := fn(&transaction{s: s, conn: conn}); err != nil {
		_, _ = conn.ExecContext(context.WithoutCancel(ctx), "ROLLBACK")
		return err
	}
	_, err = conn.ExecContext(ctx, "COMMIT")
	return wrap(err, "commit failed")
}

// transaction runs statements on one connection inside START TRANSACTION.
type transaction struct {
	s    *Session
	conn *sql.Conn
}

func (t *transaction) Exec(ctx context.Context, query string, args []value.Value) (int64, error) {
	res, err := t.s.run(ctx, t.conn, query, args, 0)
	if err != nil {
		return 0, err
	}
	if ddlKeywords[strings.ToUpper(firstKeyword(query))] {
		if _, err := t.conn.ExecContext(ctx, "START TRANSACTION"); err != nil {
			return 0, wrap(err, "cannot begin a transaction")
		}
	}
	return res.RowsAffected, nil
}

func (t *transaction) Query(ctx context.Context, query string, args []value.Value) (*engine.Result, error) {
	return t.s.run(ctx, t.conn, query, args, txMaxRows)
}

// run executes one statement. Statements that return rows are read with
// QueryContext; others with ExecContext for the affected-row count.
func (s *Session) run(ctx context.Context, q queryer, query string, args []value.Value, maxRows int) (*engine.Result, error) {
	started := time.Now()
	kw := strings.ToUpper(firstKeyword(query))
	if !returnsRows(kw) {
		r, err := q.ExecContext(ctx, query, encodeArgs(args)...)
		if err != nil {
			return nil, wrap(err, "query failed")
		}
		n, _ := r.RowsAffected()
		return &engine.Result{Columns: []value.Column{}, Rows: []value.Row{}, RowsAffected: n, Command: kw, Duration: time.Since(started)}, nil
	}
	rows, err := q.QueryContext(ctx, query, encodeArgs(args)...)
	if err != nil {
		return nil, wrap(err, "query failed")
	}
	defer func() { _ = rows.Close() }()
	if maxRows <= 0 {
		maxRows = txMaxRows
	}
	res, err := readRows(rows, maxRows)
	if err != nil {
		return nil, wrap(err, "query failed")
	}
	res.Command = kw
	res.RowsAffected = int64(len(res.Rows))
	res.Duration = time.Since(started)
	return res, nil
}

func returnsRows(kw string) bool {
	switch kw {
	case "SELECT", "WITH", "SHOW", "DESCRIBE", "DESC", "EXPLAIN", "TABLE", "VALUES", "HELP", "CALL", "CHECK", "ANALYZE", "OPTIMIZE", "REPAIR", "CHECKSUM":
		return true
	}
	return false
}

// qualify fills in the session's database as the schema.
func (s *Session) qualify(q ident.Qualified) ident.Qualified {
	if q.Schema == "" {
		q.Schema = ident.Name(s.database)
	}
	return q
}

// columnNames returns the table's columns in their declared order, or
// NotFound when the table does not exist.
func (s *Session) columnNames(ctx context.Context, q ident.Qualified) ([]ident.Name, error) {
	rows, err := s.db.QueryContext(ctx, columnNamesSQL, string(q.Schema), string(q.Name))
	if err != nil {
		return nil, wrap(err, "cannot read columns of "+q.String())
	}
	defer func() { _ = rows.Close() }()
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
