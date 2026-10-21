package sqlstore

import (
	"context"

	"github.com/google/uuid"

	"github.com/felipemalacarne/mesa/internal/store"
)

const savedQueryColumns = `id, connection_id, name, sql, updated_at`

type savedQueries struct{ *Store }

func (q savedQueries) List(ctx context.Context) ([]store.SavedQuery, error) {
	out, err := selectAll(ctx, q.Store, scanSavedQuery,
		`SELECT `+savedQueryColumns+` FROM saved_queries ORDER BY name, id`)
	if err != nil {
		return nil, fail("list saved queries", err)
	}
	return out, nil
}

func (q savedQueries) Put(ctx context.Context, sq store.SavedQuery) error {
	err := q.exec(ctx, `
		INSERT INTO saved_queries (`+savedQueryColumns+`)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT (id) DO UPDATE SET
			connection_id = excluded.connection_id, name = excluded.name,
			sql = excluded.sql, updated_at = excluded.updated_at`,
		sq.ID, nullUUID(sq.ConnectionID), sq.Name, sq.SQL, q.dialect.Time(sq.UpdatedAt))
	if err != nil {
		return fail("put saved query", err)
	}
	return nil
}

// Delete removes the query; deleting one that is already gone is fine.
func (q savedQueries) Delete(ctx context.Context, id uuid.UUID) error {
	if err := q.exec(ctx, `DELETE FROM saved_queries WHERE id = ?`, id); err != nil {
		return fail("delete saved query", err)
	}
	return nil
}

func scanSavedQuery(sc scanner) (store.SavedQuery, error) {
	var q store.SavedQuery
	var conn uuid.NullUUID
	var updated scanTime
	err := sc.Scan(&q.ID, &conn, &q.Name, &q.SQL, &updated)
	if conn.Valid {
		id := conn.UUID
		q.ConnectionID = &id
	}
	q.UpdatedAt = updated.t
	return q, err
}

// nullUUID binds an optional id, NULL when nil.
func nullUUID(id *uuid.UUID) any {
	if id == nil {
		return nil
	}
	return *id
}
