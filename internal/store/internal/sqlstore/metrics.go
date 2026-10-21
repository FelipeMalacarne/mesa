package sqlstore

import (
	"context"
	"math"
	"time"

	"github.com/google/uuid"

	"github.com/felipemalacarne/mesa/internal/store"
)

// Samples are keyed by Unix milliseconds so both backends can bucket them
// with integer arithmetic.
type metrics struct{ *Store }

func (m metrics) Add(ctx context.Context, id uuid.UUID, database string, p store.MetricPoint) error {
	err := m.exec(ctx, `
		INSERT INTO metric_points (connection_id, database, at_ms, tps, active, idle, idle_tx, latency_ms, size_bytes)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		id, database, p.At.UnixMilli(), p.TPS, p.Active, p.Idle, p.IdleTx, p.LatencyMs, p.SizeBytes)
	if err != nil {
		return fail("add metric point", err)
	}
	return nil
}

func (m metrics) Range(ctx context.Context, id uuid.UUID, database string, from, to time.Time, buckets int) ([]store.MetricPoint, error) {
	if buckets <= 0 {
		buckets = 240
	}
	step := to.Sub(from).Milliseconds() / int64(buckets)
	if step < 1 {
		step = 1
	}
	rows, err := m.db.QueryContext(ctx, m.dialect.Rebind(`
		SELECT (at_ms / ?) * ? AS bucket, AVG(tps), AVG(active), AVG(idle), AVG(idle_tx), AVG(latency_ms), MAX(size_bytes)
		FROM metric_points
		WHERE connection_id = ? AND database = ? AND at_ms >= ? AND at_ms < ?
		GROUP BY bucket
		ORDER BY bucket`), step, step, id, database, from.UnixMilli(), to.UnixMilli())
	if err != nil {
		return nil, fail("read metric points", err)
	}
	defer func() { _ = rows.Close() }()
	var out []store.MetricPoint
	for rows.Next() {
		var bucket int64
		var tps, active, idle, idleTx, latency float64
		var size int64
		if err := rows.Scan(&bucket, &tps, &active, &idle, &idleTx, &latency, &size); err != nil {
			return nil, fail("read metric points", err)
		}
		out = append(out, store.MetricPoint{
			At: time.UnixMilli(bucket).UTC(), TPS: tps, LatencyMs: latency, SizeBytes: size,
			Active: int(math.Round(active)), Idle: int(math.Round(idle)), IdleTx: int(math.Round(idleTx)),
		})
	}
	if err := rows.Err(); err != nil {
		return nil, fail("read metric points", err)
	}
	return out, nil
}

func (m metrics) Prune(ctx context.Context, before time.Time) error {
	if err := m.exec(ctx, `DELETE FROM metric_points WHERE at_ms < ?`, before.UnixMilli()); err != nil {
		return fail("prune metric points", err)
	}
	return nil
}
