package sqlstore

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"

	"github.com/google/uuid"

	"github.com/felipemalacarne/mesa/internal/core/errs"
	"github.com/felipemalacarne/mesa/internal/store"
)

type layouts struct{ *Store }

func (l layouts) Get(ctx context.Context, connectionID uuid.UUID, database string) (map[string]store.Position, error) {
	var raw []byte
	err := l.queryRow(ctx,
		`SELECT positions_json FROM layouts WHERE connection_id = ? AND database = ?`,
		connectionID, database).Scan(&raw)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, errs.Newf(errs.NotFound, "no layout for database %q of connection %s", database, connectionID)
	}
	if err != nil {
		return nil, fail("get layout", err)
	}
	positions := map[string]store.Position{}
	if err := json.Unmarshal(raw, &positions); err != nil {
		return nil, fail("decode layout", err)
	}
	if positions == nil { // a stored JSON null
		positions = map[string]store.Position{}
	}
	return positions, nil
}

// Put replaces the whole layout of (connection, database).
func (l layouts) Put(ctx context.Context, connectionID uuid.UUID, database string, positions map[string]store.Position) error {
	if positions == nil {
		positions = map[string]store.Position{}
	}
	raw, err := json.Marshal(positions)
	if err != nil {
		return fail("encode layout", err)
	}
	err = l.exec(ctx, `
		INSERT INTO layouts (connection_id, database, positions_json) VALUES (?, ?, ?)
		ON CONFLICT (connection_id, database) DO UPDATE SET
			positions_json = excluded.positions_json`,
		connectionID, database, string(raw))
	if err != nil {
		return fail("put layout", err)
	}
	return nil
}
