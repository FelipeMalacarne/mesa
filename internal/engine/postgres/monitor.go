package postgres

import (
	"context"
	_ "embed"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/felipemalacarne/mesa/internal/core/errs"
	"github.com/felipemalacarne/mesa/internal/engine"
)

var (
	//go:embed sql/overview.sql
	overviewSQL string
	//go:embed sql/metrics.sql
	metricsSQL string
	//go:embed sql/sessions.sql
	sessionsSQL string
	//go:embed sql/panel_cache_hit.sql
	panelCacheHitSQL string
	//go:embed sql/panel_replication_lag.sql
	panelReplicationLagSQL string
	//go:embed sql/panel_locks_waiting.sql
	panelLocksWaitingSQL string
	//go:embed sql/panel_long_transactions.sql
	panelLongTransactionsSQL string
	//go:embed sql/panel_wal.sql
	panelWALSQL string
	//go:embed sql/panel_checkpoints_17.sql
	panelCheckpoints17SQL string
	//go:embed sql/panel_checkpoints_legacy.sql
	panelCheckpointsLegacySQL string
	//go:embed sql/panel_xid_age.sql
	panelXIDAgeSQL string
	//go:embed sql/panel_statements_installed.sql
	panelStatementsInstalledSQL string
	//go:embed sql/panel_statements.sql
	panelStatementsSQL string
	//go:embed sql/panel_statements_legacy.sql
	panelStatementsLegacySQL string
	//go:embed sql/panel_dead_tuples.sql
	panelDeadTuplesSQL string
	//go:embed sql/panel_replication.sql
	panelReplicationSQL string
)

// Overview implements engine.Monitor.
func (s *Session) Overview(ctx context.Context) (*engine.Overview, error) {
	var o engine.Overview
	var num int
	err := s.pool.QueryRow(ctx, overviewSQL).Scan(&o.VersionFull, &num, &o.StartedAt, &o.Database, &o.Role, &o.ReadOnly)
	if err != nil {
		return nil, wrap(err, "cannot read the server overview")
	}
	o.Version = versionString(num)
	return &o, nil
}

// versionString renders server_version_num as major.minor.
func versionString(num int) string {
	if num < 100000 {
		return fmt.Sprintf("%d.%d", num/10000, (num/100)%100)
	}
	return fmt.Sprintf("%d.%d", num/10000, num%100)
}

// Metrics implements engine.Monitor.
func (s *Session) Metrics(ctx context.Context) (*engine.Metrics, error) {
	m := engine.Metrics{SampledAt: time.Now()}
	var hit, read int64
	err := s.pool.QueryRow(ctx, metricsSQL).Scan(
		&m.Commits, &m.Rollbacks, &hit, &read,
		&m.ConnectionsActive, &m.ConnectionsIdle, &m.ConnectionsIdleTx, &m.ConnectionsMax,
		&m.DatabaseSizeBytes, &m.LocksWaiting, &m.LongestTxSeconds,
	)
	if err != nil {
		return nil, wrap(err, "cannot read metrics")
	}
	m.CacheHitRatio = ratio(hit, read)
	return &m, nil
}

func ratio(hit, read int64) float64 {
	if hit+read == 0 {
		return 0
	}
	return float64(hit) / float64(hit+read)
}

// Sessions implements engine.Monitor.
func (s *Session) Sessions(ctx context.Context) ([]engine.ActiveSession, error) {
	rows, err := s.pool.Query(ctx, sessionsSQL)
	if err != nil {
		return nil, wrap(err, "cannot read sessions")
	}
	defer rows.Close()
	var out []engine.ActiveSession
	for rows.Next() {
		var a engine.ActiveSession
		var queryStart, xactStart pgtype.Timestamptz
		var now time.Time
		if err := rows.Scan(&a.PID, &a.User, &a.Database, &a.Application, &a.ClientAddr, &a.State,
			&a.WaitEvent, &a.Query, &a.StartedAt, &queryStart, &xactStart, &now, &a.Blocking); err != nil {
			return nil, wrap(err, "cannot read sessions")
		}
		if queryStart.Valid {
			t := queryStart.Time
			a.QueryStart = &t
		}
		switch {
		case a.State == "active" && queryStart.Valid:
			a.Duration = now.Sub(queryStart.Time)
		case xactStart.Valid:
			a.Duration = now.Sub(xactStart.Time)
		}
		out = append(out, a)
	}
	if err := rows.Err(); err != nil {
		return nil, wrap(err, "cannot read sessions")
	}
	return out, nil
}

// Kill implements engine.Monitor.
func (s *Session) Kill(ctx context.Context, pid int, terminate bool) error {
	fn := "pg_cancel_backend"
	if terminate {
		fn = "pg_terminate_backend"
	}
	var ok bool
	if err := s.pool.QueryRow(ctx, "select "+fn+"($1)", pid).Scan(&ok); err != nil {
		return wrap(err, "cannot signal session")
	}
	if !ok {
		return errs.Newf(errs.NotFound, "no session with pid %d", pid)
	}
	return nil
}

// EnginePanel implements engine.Monitor. Every part is optional: a view
// the role cannot read or an extension that is missing leaves its tile
// out instead of failing the panel.
func (s *Session) EnginePanel(ctx context.Context) (*engine.Panel, error) {
	p := &engine.Panel{Title: "PostgreSQL"}
	parts := []func(context.Context, *engine.Panel){
		s.cacheHitTile,
		s.replicationLagTile,
		s.locksWaitingTile,
		s.longTransactionsTile,
		s.walTile,
		s.checkpointsTile,
		s.xidAgeMeter,
		s.statementsTable,
		s.deadTuplesTable,
		s.replicationTable,
	}
	for _, part := range parts {
		part(ctx, p)
		if err := ctx.Err(); err != nil {
			return nil, wrap(err, "panel interrupted")
		}
	}
	return p, nil
}

func (s *Session) cacheHitTile(ctx context.Context, p *engine.Panel) {
	var hit, read int64
	if err := s.pool.QueryRow(ctx, panelCacheHitSQL).Scan(&hit, &read); err != nil {
		return
	}
	t := engine.Tile{Label: "Cache hit ratio", Unit: "%"}
	if hit+read == 0 {
		t.Value, t.Note = "n/a", "no reads yet"
		p.Tiles = append(p.Tiles, t)
		return
	}
	r := ratio(hit, read)
	t.Value = strconv.FormatFloat(r*100, 'f', 1, 64)
	switch {
	case r >= 0.99:
		t.Status = engine.StatusOK
	case r >= 0.95:
		t.Status = engine.StatusWarning
	default:
		t.Status = engine.StatusCritical
	}
	p.Tiles = append(p.Tiles, t)
}

func (s *Session) replicationLagTile(ctx context.Context, p *engine.Panel) {
	var standbys int
	var worst float64
	if err := s.pool.QueryRow(ctx, panelReplicationLagSQL).Scan(&standbys, &worst); err != nil {
		return
	}
	t := engine.Tile{Label: "Replication lag", Status: engine.StatusOK}
	if standbys == 0 {
		t.Value, t.Note = "none", "no standbys connected"
		p.Tiles = append(p.Tiles, t)
		return
	}
	t.Value, t.Unit = strconv.FormatFloat(worst, 'f', 2, 64), "s"
	t.Note = fmt.Sprintf("worst of %d standby(s)", standbys)
	switch {
	case worst >= 60:
		t.Status = engine.StatusCritical
	case worst >= 10:
		t.Status = engine.StatusWarning
	}
	p.Tiles = append(p.Tiles, t)
}

func (s *Session) locksWaitingTile(ctx context.Context, p *engine.Panel) {
	var n int
	if err := s.pool.QueryRow(ctx, panelLocksWaitingSQL).Scan(&n); err != nil {
		return
	}
	t := engine.Tile{Label: "Locks waiting", Value: strconv.Itoa(n), Status: engine.StatusOK}
	if n > 0 {
		t.Status = engine.StatusWarning
	}
	p.Tiles = append(p.Tiles, t)
}

func (s *Session) longTransactionsTile(ctx context.Context, p *engine.Panel) {
	var n int
	if err := s.pool.QueryRow(ctx, panelLongTransactionsSQL).Scan(&n); err != nil {
		return
	}
	t := engine.Tile{Label: "Long transactions", Value: strconv.Itoa(n), Note: "open for more than 5 min", Status: engine.StatusOK}
	if n > 0 {
		t.Status = engine.StatusWarning
	}
	p.Tiles = append(p.Tiles, t)
}

func (s *Session) walTile(ctx context.Context, p *engine.Panel) {
	var bytes int64
	if err := s.pool.QueryRow(ctx, panelWALSQL).Scan(&bytes); err != nil {
		return // pg_stat_wal exists from 14 on
	}
	p.Tiles = append(p.Tiles, engine.Tile{Label: "WAL generated", Value: humanBytes(bytes), Note: "since stats reset"})
}

func (s *Session) checkpointsTile(ctx context.Context, p *engine.Panel) {
	sql := panelCheckpointsLegacySQL
	if s.major() >= 17 {
		sql = panelCheckpoints17SQL
	}
	var timed, requested int64
	if err := s.pool.QueryRow(ctx, sql).Scan(&timed, &requested); err != nil {
		return
	}
	t := engine.Tile{
		Label:  "Checkpoints",
		Value:  strconv.FormatInt(timed+requested, 10),
		Note:   fmt.Sprintf("%d timed, %d requested", timed, requested),
		Status: engine.StatusOK,
	}
	if requested > timed {
		t.Status = engine.StatusWarning
	}
	p.Tiles = append(p.Tiles, t)
}

// xidAgeMeter shows how far the oldest database is from transaction ID
// wraparound (2^31 ≈ 2.1 billion).
func (s *Session) xidAgeMeter(ctx context.Context, p *engine.Panel) {
	var age int64
	if err := s.pool.QueryRow(ctx, panelXIDAgeSQL).Scan(&age); err != nil {
		return
	}
	const limit = 2.1e9
	m := engine.Meter{
		Label:  "Transaction ID age",
		Value:  float64(age),
		Max:    limit,
		Text:   humanCount(float64(age)) + " / " + humanCount(limit),
		Status: engine.StatusOK,
		Note:   "oldest datfrozenxid vs wraparound",
	}
	switch {
	case age > 1.6e9:
		m.Status = engine.StatusCritical
	case age > 1.2e9:
		m.Status = engine.StatusWarning
	}
	p.Meters = append(p.Meters, m)
}

func (s *Session) statementsTable(ctx context.Context, p *engine.Panel) {
	t := engine.PanelTable{
		Title:   "Top statements",
		Source:  "pg_stat_statements",
		Columns: []string{"Query", "Calls", "Mean ms", "Share %"},
		Numeric: []bool{false, true, true, true},
		Rows:    [][]string{},
	}
	var installed bool
	if err := s.pool.QueryRow(ctx, panelStatementsInstalledSQL).Scan(&installed); err != nil || !installed {
		t.Empty = "pg_stat_statements is not installed"
		p.Tables = append(p.Tables, t)
		return
	}
	sql := panelStatementsSQL
	if s.major() < 13 {
		sql = panelStatementsLegacySQL
	}
	rows, err := s.pool.Query(ctx, sql)
	if err != nil {
		t.Empty = "pg_stat_statements could not be read"
		p.Tables = append(p.Tables, t)
		return
	}
	defer rows.Close()
	for rows.Next() {
		var query string
		var calls int64
		var mean, share float64
		if err := rows.Scan(&query, &calls, &mean, &share); err != nil {
			break
		}
		t.Rows = append(t.Rows, []string{
			query,
			strconv.FormatInt(calls, 10),
			strconv.FormatFloat(mean, 'f', 2, 64),
			strconv.FormatFloat(share, 'f', 1, 64),
		})
	}
	if len(t.Rows) == 0 {
		t.Empty = "no statements recorded"
	}
	p.Tables = append(p.Tables, t)
}

func (s *Session) deadTuplesTable(ctx context.Context, p *engine.Panel) {
	t := engine.PanelTable{
		Title:   "Dead tuples",
		Source:  "pg_stat_user_tables",
		Columns: []string{"Table", "Dead", "Dead %", "Last autovacuum"},
		Numeric: []bool{false, true, true, false},
		Rows:    [][]string{},
		Empty:   "no dead tuples",
	}
	rows, err := s.pool.Query(ctx, panelDeadTuplesSQL)
	if err != nil {
		return
	}
	defer rows.Close()
	for rows.Next() {
		var table string
		var dead int64
		var pct float64
		var last pgtype.Timestamptz
		if err := rows.Scan(&table, &dead, &pct, &last); err != nil {
			return
		}
		lastText := "never"
		if last.Valid {
			lastText = last.Time.UTC().Format(time.RFC3339)
		}
		t.Rows = append(t.Rows, []string{table, strconv.FormatInt(dead, 10), strconv.FormatFloat(pct, 'f', 1, 64), lastText})
	}
	p.Tables = append(p.Tables, t)
}

func (s *Session) replicationTable(ctx context.Context, p *engine.Panel) {
	t := engine.PanelTable{
		Title:   "Replication",
		Source:  "pg_stat_replication",
		Columns: []string{"Standby", "Client", "State", "Sync", "Sent LSN", "Replay lag"},
		Rows:    [][]string{},
		Empty:   "no standbys connected",
	}
	rows, err := s.pool.Query(ctx, panelReplicationSQL)
	if err != nil {
		return
	}
	defer rows.Close()
	for rows.Next() {
		row := make([]string, 6)
		if err := rows.Scan(&row[0], &row[1], &row[2], &row[3], &row[4], &row[5]); err != nil {
			return
		}
		t.Rows = append(t.Rows, row)
	}
	p.Tables = append(p.Tables, t)
}

// humanCount renders 1310000000 as "1.31B", 42000 as "42K".
func humanCount(n float64) string {
	units := []struct {
		div    float64
		suffix string
	}{{1e12, "T"}, {1e9, "B"}, {1e6, "M"}, {1e3, "K"}}
	for _, u := range units {
		if n >= u.div {
			return trimFloat(n/u.div, 2) + u.suffix
		}
	}
	return trimFloat(n, 0)
}

// humanBytes renders a byte count with a binary unit.
func humanBytes(n int64) string {
	units := []string{"B", "KiB", "MiB", "GiB", "TiB", "PiB"}
	v := float64(n)
	i := 0
	for v >= 1024 && i < len(units)-1 {
		v /= 1024
		i++
	}
	if i == 0 {
		return strconv.FormatInt(n, 10) + " B"
	}
	return trimFloat(v, 1) + " " + units[i]
}

// trimFloat formats with up to prec decimals and no trailing zeros.
func trimFloat(v float64, prec int) string {
	s := strconv.FormatFloat(v, 'f', prec, 64)
	if strings.Contains(s, ".") {
		s = strings.TrimRight(strings.TrimRight(s, "0"), ".")
	}
	return s
}
