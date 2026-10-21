package sqlstore

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/felipemalacarne/mesa/internal/store"
)

const historyColumns = `id, connection_id, database, sql, row_count, duration_ms, error, ran_at`

type history struct{ *Store }

func (h history) Add(ctx context.Context, r store.QueryRun) error {
	err := h.exec(ctx, `
		INSERT INTO history (`+historyColumns+`)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		r.ID, r.ConnectionID, r.Database, r.SQL, r.RowCount, r.Duration.Milliseconds(),
		r.Error, h.dialect.Time(r.RanAt))
	if err != nil {
		return fail("add history entry", err)
	}
	return nil
}

// List returns the newest runs of a connection first.
func (h history) List(ctx context.Context, connectionID uuid.UUID, limit int) ([]store.QueryRun, error) {
	out, err := selectAll(ctx, h.Store, scanQueryRun, `
		SELECT `+historyColumns+` FROM history
		WHERE connection_id = ?
		ORDER BY ran_at DESC, id DESC
		LIMIT ?`, connectionID, limitOrDefault(limit))
	if err != nil {
		return nil, fail("list history", err)
	}
	return out, nil
}

func scanQueryRun(sc scanner) (store.QueryRun, error) {
	var r store.QueryRun
	var ms int64
	var ran scanTime
	err := sc.Scan(&r.ID, &r.ConnectionID, &r.Database, &r.SQL, &r.RowCount, &ms, &r.Error, &ran)
	r.Duration = time.Duration(ms) * time.Millisecond
	r.RanAt = ran.t
	return r, err
}
