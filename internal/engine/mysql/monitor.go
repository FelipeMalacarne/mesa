package mysql

import (
	"context"
	"database/sql"
	"strconv"
	"strings"
	"time"

	"github.com/felipemalacarne/mesa/internal/core/errs"
	"github.com/felipemalacarne/mesa/internal/engine"
)

// status reads SHOW GLOBAL STATUS (or VARIABLES) into a map.
func (s *Session) status(ctx context.Context, what string) (map[string]string, error) {
	rows, err := s.db.QueryContext(ctx, "SHOW GLOBAL "+what)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	out := map[string]string{}
	for rows.Next() {
		var k, v sql.NullString
		if err := rows.Scan(&k, &v); err != nil {
			return nil, err
		}
		out[strings.ToLower(k.String)] = v.String
	}
	return out, rows.Err()
}

func num(m map[string]string, k string) int64 {
	n, _ := strconv.ParseInt(m[k], 10, 64)
	return n
}

// Overview implements engine.Monitor.
func (s *Session) Overview(ctx context.Context) (*engine.Overview, error) {
	var user string
	var uptime int64
	var readOnly, superReadOnly int
	err := s.db.QueryRowContext(ctx, "SELECT CURRENT_USER(), @@global.read_only, COALESCE(@@global.super_read_only, 0)").Scan(&user, &readOnly, &superReadOnly)
	if err != nil {
		// MariaDB has no super_read_only.
		err = s.db.QueryRowContext(ctx, "SELECT CURRENT_USER(), @@global.read_only, 0").Scan(&user, &readOnly, &superReadOnly)
	}
	if err != nil {
		return nil, wrap(err, "cannot read the overview")
	}
	st, err := s.status(ctx, "STATUS LIKE 'Uptime'")
	if err != nil {
		return nil, wrap(err, "cannot read the overview")
	}
	uptime = num(st, "uptime")
	if i := strings.LastIndexByte(user, '@'); i > 0 && user[i+1:] == "%" {
		user = user[:i]
	}
	product := "MySQL"
	if strings.Contains(strings.ToLower(s.version), "mariadb") {
		product = "MariaDB"
	}
	return &engine.Overview{
		Version:     shortVersion(s.version),
		VersionFull: product + " " + s.version,
		StartedAt:   time.Now().Add(-time.Duration(uptime) * time.Second).UTC().Truncate(time.Second),
		Database:    s.database,
		Role:        user,
		ReadOnly:    readOnly == 1 || superReadOnly == 1,
	}, nil
}

// shortVersion keeps major.minor: "8.0.36-0ubuntu…" → "8.0".
func shortVersion(v string) string {
	parts := strings.SplitN(v, ".", 3)
	if len(parts) < 2 {
		return v
	}
	return parts[0] + "." + strings.TrimFunc(parts[1], func(r rune) bool { return r < '0' || r > '9' })
}

// Metrics implements engine.Monitor.
func (s *Session) Metrics(ctx context.Context) (*engine.Metrics, error) {
	st, err := s.status(ctx, "STATUS")
	if err != nil {
		return nil, wrap(err, "cannot read status counters")
	}
	vars, err := s.status(ctx, "VARIABLES LIKE 'max_connections'")
	if err != nil {
		return nil, wrap(err, "cannot read max_connections")
	}
	m := &engine.Metrics{
		SampledAt:      time.Now().UTC(),
		Commits:        num(st, "handler_commit"),
		Rollbacks:      num(st, "handler_rollback"),
		ConnectionsMax: int(num(vars, "max_connections")),
	}
	reads, requests := num(st, "innodb_buffer_pool_reads"), num(st, "innodb_buffer_pool_read_requests")
	if requests > 0 {
		m.CacheHitRatio = 1 - float64(reads)/float64(requests)
	}
	_ = s.db.QueryRowContext(ctx, `SELECT COALESCE(SUM(data_length + index_length), 0) FROM information_schema.tables WHERE table_schema = ?`, s.database).Scan(&m.DatabaseSizeBytes)
	_ = s.db.QueryRowContext(ctx, `SELECT
	  COALESCE(SUM(p.command <> 'Sleep'), 0),
	  COALESCE(SUM(p.command = 'Sleep' AND t.trx_id IS NULL), 0),
	  COALESCE(SUM(p.command = 'Sleep' AND t.trx_id IS NOT NULL), 0)
	FROM information_schema.processlist p
	LEFT JOIN information_schema.innodb_trx t ON t.trx_mysql_thread_id = p.id
	WHERE p.command NOT IN ('Daemon', 'Binlog Dump', 'Binlog Dump GTID')`).Scan(&m.ConnectionsActive, &m.ConnectionsIdle, &m.ConnectionsIdleTx)
	_ = s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM information_schema.innodb_trx WHERE trx_state = 'LOCK WAIT'`).Scan(&m.LocksWaiting)
	var longest sql.NullFloat64
	_ = s.db.QueryRowContext(ctx, `SELECT MAX(TIMESTAMPDIFF(MICROSECOND, trx_started, NOW())) / 1e6 FROM information_schema.innodb_trx`).Scan(&longest)
	m.LongestTxSeconds = longest.Float64
	return m, nil
}

// Sessions implements engine.Monitor: every client thread but this one.
func (s *Session) Sessions(ctx context.Context) ([]engine.ActiveSession, error) {
	blocking := s.blockingThreads(ctx)
	rows, err := s.db.QueryContext(ctx, `SELECT p.id, COALESCE(p.user, ''), COALESCE(p.db, ''), COALESCE(p.host, ''),
	       p.command, COALESCE(p.state, ''), COALESCE(p.info, ''), COALESCE(p.time, 0),
	       t.trx_started
	FROM information_schema.processlist p
	LEFT JOIN information_schema.innodb_trx t ON t.trx_mysql_thread_id = p.id
	WHERE p.id <> CONNECTION_ID()
	  AND p.command NOT IN ('Daemon', 'Binlog Dump', 'Binlog Dump GTID')
	  AND p.user NOT IN ('system user', 'event_scheduler')
	ORDER BY p.time DESC`)
	if err != nil {
		return nil, wrap(err, "cannot list sessions")
	}
	defer func() { _ = rows.Close() }()
	now := time.Now().UTC()
	var out []engine.ActiveSession
	for rows.Next() {
		var a engine.ActiveSession
		var command, state, query string
		var secs int64
		var trxStarted sql.NullString
		if err := rows.Scan(&a.PID, &a.User, &a.Database, &a.ClientAddr, &command, &state, &query, &secs, &trxStarted); err != nil {
			return nil, wrap(err, "cannot list sessions")
		}
		if i := strings.LastIndexByte(a.ClientAddr, ':'); i > 0 {
			a.ClientAddr = a.ClientAddr[:i]
		}
		switch {
		case command == "Sleep" && trxStarted.Valid:
			a.State = "idle in transaction"
		case command == "Sleep":
			a.State = "idle"
		default:
			a.State = "active"
			a.WaitEvent = state
			a.Query = query
			qs := now.Add(-time.Duration(secs) * time.Second)
			a.QueryStart = &qs
		}
		a.Duration = time.Duration(secs) * time.Second
		if trxStarted.Valid {
			if t, err := time.Parse("2006-01-02 15:04:05", trxStarted.String); err == nil {
				a.StartedAt = t.UTC()
				if a.State == "idle in transaction" {
					a.Duration = now.Sub(a.StartedAt)
				}
			}
		}
		if a.StartedAt.IsZero() {
			a.StartedAt = now.Add(-a.Duration)
		}
		a.Blocking = blocking[a.PID]
		out = append(out, a)
	}
	return out, wrap(rows.Err(), "cannot list sessions")
}

// blockingThreads lists the threads other transactions wait on, through
// sys.innodb_lock_waits when the sys schema is readable.
func (s *Session) blockingThreads(ctx context.Context) map[int]bool {
	out := map[int]bool{}
	rows, err := s.db.QueryContext(ctx, `SELECT DISTINCT blocking_pid FROM sys.innodb_lock_waits`)
	if err != nil {
		return out
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var pid int
		if rows.Scan(&pid) == nil {
			out[pid] = true
		}
	}
	return out
}

// Kill implements engine.Monitor: KILL QUERY cancels the statement, KILL
// ends the connection.
func (s *Session) Kill(ctx context.Context, pid int, terminate bool) error {
	if pid <= 0 {
		return errs.New(errs.Invalid, "invalid session id")
	}
	sql := "KILL QUERY " + strconv.Itoa(pid)
	if terminate {
		sql = "KILL " + strconv.Itoa(pid)
	}
	if _, err := s.db.ExecContext(ctx, sql); err != nil {
		return wrap(err, "cannot stop session "+strconv.Itoa(pid))
	}
	return nil
}

// EnginePanel implements engine.Monitor: InnoDB and server counters, the
// busiest statement digests and the largest tables.
func (s *Session) EnginePanel(ctx context.Context) (*engine.Panel, error) {
	st, err := s.status(ctx, "STATUS")
	if err != nil {
		return nil, wrap(err, "cannot read status counters")
	}
	vars, _ := s.status(ctx, "VARIABLES")
	p := &engine.Panel{Title: "MySQL · InnoDB"}
	uptime := max(num(st, "uptime"), 1)

	reads, requests := num(st, "innodb_buffer_pool_reads"), num(st, "innodb_buffer_pool_read_requests")
	if requests > 0 {
		hit := 1 - float64(reads)/float64(requests)
		status := engine.StatusOK
		if hit < 0.95 {
			status = engine.StatusWarning
		}
		if hit < 0.80 {
			status = engine.StatusCritical
		}
		p.Tiles = append(p.Tiles, engine.Tile{Label: "Buffer pool hit", Value: trimFloat(hit*100, 2), Unit: "%", Status: status, Note: "reads served from memory"})
	}
	p.Tiles = append(p.Tiles,
		engine.Tile{Label: "Queries / s", Value: trimFloat(float64(num(st, "questions"))/float64(uptime), 1), Note: "average since start"},
		engine.Tile{Label: "Threads running", Value: strconv.FormatInt(num(st, "threads_running"), 10), Note: strconv.FormatInt(num(st, "threads_connected"), 10) + " connected"},
	)
	slow := num(st, "slow_queries")
	slowStatus := engine.StatusNone
	if slow > 0 {
		slowStatus = engine.StatusWarning
	}
	p.Tiles = append(p.Tiles, engine.Tile{Label: "Slow queries", Value: humanCount(float64(slow)), Status: slowStatus, Note: "over long_query_time (" + trimFloat(parseFloat(vars["long_query_time"]), 1) + " s)"})
	waits := num(st, "innodb_row_lock_waits")
	p.Tiles = append(p.Tiles, engine.Tile{Label: "Row lock waits", Value: humanCount(float64(waits)), Note: "avg " + strconv.FormatInt(num(st, "innodb_row_lock_time_avg"), 10) + " ms"})
	if aborted := num(st, "aborted_connects"); aborted > 0 {
		p.Tiles = append(p.Tiles, engine.Tile{Label: "Aborted connects", Value: humanCount(float64(aborted)), Status: engine.StatusWarning, Note: "failed logins and handshakes"})
	}

	if total := num(st, "innodb_buffer_pool_pages_total"); total > 0 {
		data := num(st, "innodb_buffer_pool_pages_data")
		ratio := float64(data) / float64(total)
		status := engine.StatusOK
		if ratio > 0.95 {
			status = engine.StatusWarning
		}
		pageSize := max(num(st, "innodb_page_size"), 16384)
		p.Meters = append(p.Meters, engine.Meter{
			Label: "Buffer pool used", Value: float64(data), Max: float64(total),
			Text:   humanBytes(data*pageSize) + " / " + humanBytes(total*pageSize),
			Status: status, Note: strconv.FormatInt(num(st, "innodb_buffer_pool_pages_dirty"), 10) + " dirty pages",
		})
	}
	if maxConn := num(vars, "max_connections"); maxConn > 0 {
		used := num(st, "max_used_connections")
		status := engine.StatusOK
		if float64(used) > 0.8*float64(maxConn) {
			status = engine.StatusWarning
		}
		p.Meters = append(p.Meters, engine.Meter{
			Label: "Peak connections", Value: float64(used), Max: float64(maxConn),
			Text: strconv.FormatInt(used, 10) + " / " + strconv.FormatInt(maxConn, 10), Status: status, Note: "since start",
		})
	}

	s.digestTable(ctx, p)
	s.largestTables(ctx, p)
	return p, nil
}

func parseFloat(s string) float64 {
	f, _ := strconv.ParseFloat(s, 64)
	return f
}

func (s *Session) digestTable(ctx context.Context, p *engine.Panel) {
	t := engine.PanelTable{
		Title: "Top statements", Source: "performance_schema",
		Columns: []string{"Statement", "Calls", "Total", "Mean", "Rows"},
		Numeric: []bool{false, true, true, true, true},
		Rows:    [][]string{},
		Empty:   "performance_schema statement digests are not available",
	}
	rows, err := s.db.QueryContext(ctx, `SELECT COALESCE(digest_text, ''), count_star, sum_timer_wait / 1e12, avg_timer_wait / 1e12, sum_rows_examined
	FROM performance_schema.events_statements_summary_by_digest
	WHERE schema_name = ? AND digest_text IS NOT NULL
	ORDER BY sum_timer_wait DESC LIMIT 10`, s.database)
	if err == nil {
		defer func() { _ = rows.Close() }()
		for rows.Next() {
			var text string
			var calls, rowsExamined int64
			var total, mean float64
			if rows.Scan(&text, &calls, &total, &mean, &rowsExamined) != nil {
				break
			}
			if len(text) > 160 {
				text = text[:157] + "…"
			}
			t.Rows = append(t.Rows, []string{text, humanCount(float64(calls)), trimFloat(total, 2) + " s", trimFloat(mean*1000, 2) + " ms", humanCount(float64(rowsExamined))})
		}
	}
	p.Tables = append(p.Tables, t)
}

func (s *Session) largestTables(ctx context.Context, p *engine.Panel) {
	t := engine.PanelTable{
		Title: "Largest tables", Source: "information_schema.tables",
		Columns: []string{"Table", "Rows (est.)", "Data", "Indexes", "Free"},
		Numeric: []bool{false, true, true, true, true},
		Rows:    [][]string{},
		Empty:   "no tables yet",
	}
	rows, err := s.db.QueryContext(ctx, `SELECT table_name, COALESCE(table_rows, 0), COALESCE(data_length, 0), COALESCE(index_length, 0), COALESCE(data_free, 0)
	FROM information_schema.tables
	WHERE table_schema = ? AND table_type = 'BASE TABLE'
	ORDER BY COALESCE(data_length, 0) + COALESCE(index_length, 0) DESC LIMIT 10`, s.database)
	if err == nil {
		defer func() { _ = rows.Close() }()
		for rows.Next() {
			var name string
			var n, data, idx, free int64
			if rows.Scan(&name, &n, &data, &idx, &free) != nil {
				break
			}
			t.Rows = append(t.Rows, []string{name, humanCount(float64(n)), humanBytes(data), humanBytes(idx), humanBytes(free)})
		}
	}
	p.Tables = append(p.Tables, t)
}
