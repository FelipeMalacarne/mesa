package sqlstore

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/google/uuid"

	"github.com/felipemalacarne/mesa/internal/core/errs"
)

type secrets struct{ *Store }

func (s secrets) Get(ctx context.Context, id uuid.UUID) ([]byte, error) {
	var blob []byte
	err := s.queryRow(ctx, `SELECT blob FROM secrets WHERE connection_id = ?`, id).Scan(&blob)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, errs.Newf(errs.NotFound, "no secret for connection %s", id)
	}
	if err != nil {
		return nil, fail("get secret", err)
	}
	return blob, nil
}

func (s secrets) Set(ctx context.Context, id uuid.UUID, blob []byte) error {
	if blob == nil {
		blob = []byte{} // a nil slice would bind NULL
	}
	err := s.exec(ctx, `
		INSERT INTO secrets (connection_id, blob, updated_at) VALUES (?, ?, ?)
		ON CONFLICT (connection_id) DO UPDATE SET
			blob = excluded.blob, updated_at = excluded.updated_at`,
		id, blob, s.dialect.Time(time.Now()))
	if err != nil {
		return fail("set secret", err)
	}
	return nil
}

// Delete removes the secret; deleting one that is already gone is fine.
func (s secrets) Delete(ctx context.Context, id uuid.UUID) error {
	if err := s.exec(ctx, `DELETE FROM secrets WHERE connection_id = ?`, id); err != nil {
		return fail("delete secret", err)
	}
	return nil
}
