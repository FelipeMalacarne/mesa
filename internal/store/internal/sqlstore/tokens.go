package sqlstore

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/google/uuid"

	"github.com/felipemalacarne/mesa/internal/core/errs"
	"github.com/felipemalacarne/mesa/internal/store"
)

const tokenColumns = `id, user_id, kind, name, hash, created_at, expires_at, last_used_at`

type tokens struct{ *Store }

func (t tokens) Create(ctx context.Context, tok store.Token) error {
	err := t.exec(ctx, `INSERT INTO tokens (`+tokenColumns+`) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		tok.ID, tok.UserID, string(tok.Kind), tok.Name, tok.Hash,
		t.dialect.Time(tok.CreatedAt), t.nullTime(tok.ExpiresAt), t.nullTime(tok.LastUsedAt))
	if err != nil {
		return fail("create token", err)
	}
	return nil
}

func (t tokens) ByHash(ctx context.Context, hash []byte) (*store.Token, error) {
	tok, err := scanToken(t.queryRow(ctx, `SELECT `+tokenColumns+` FROM tokens WHERE hash = ?`, hash))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, errs.New(errs.NotFound, "token not found")
	}
	if err != nil {
		return nil, fail("get token", err)
	}
	return &tok, nil
}

func (t tokens) List(ctx context.Context, userID uuid.UUID, kind store.TokenKind) ([]store.Token, error) {
	out, err := selectAll(ctx, t.Store, scanToken,
		`SELECT `+tokenColumns+` FROM tokens WHERE user_id = ? AND kind = ? ORDER BY created_at DESC, id DESC`,
		userID, string(kind))
	if err != nil {
		return nil, fail("list tokens", err)
	}
	return out, nil
}

func (t tokens) Touch(ctx context.Context, id uuid.UUID, at time.Time) error {
	if err := t.exec(ctx, `UPDATE tokens SET last_used_at = ? WHERE id = ?`, t.dialect.Time(at), id); err != nil {
		return fail("touch token", err)
	}
	return nil
}

func (t tokens) Delete(ctx context.Context, id uuid.UUID) error {
	if err := t.exec(ctx, `DELETE FROM tokens WHERE id = ?`, id); err != nil {
		return fail("delete token", err)
	}
	return nil
}

func (t tokens) DeleteForUser(ctx context.Context, userID uuid.UUID, kind store.TokenKind) error {
	var err error
	if kind == "" {
		err = t.exec(ctx, `DELETE FROM tokens WHERE user_id = ?`, userID)
	} else {
		err = t.exec(ctx, `DELETE FROM tokens WHERE user_id = ? AND kind = ?`, userID, string(kind))
	}
	if err != nil {
		return fail("delete tokens", err)
	}
	return nil
}

func (t tokens) Prune(ctx context.Context, before time.Time) error {
	if err := t.exec(ctx, `DELETE FROM tokens WHERE expires_at IS NOT NULL AND expires_at < ?`, t.dialect.Time(before)); err != nil {
		return fail("prune tokens", err)
	}
	return nil
}

func scanToken(sc scanner) (store.Token, error) {
	var tok store.Token
	var created, expires, used scanTime
	err := sc.Scan(&tok.ID, &tok.UserID, &tok.Kind, &tok.Name, &tok.Hash, &created, &expires, &used)
	tok.CreatedAt, tok.ExpiresAt, tok.LastUsedAt = created.t, expires.ptr(), used.ptr()
	return tok, err
}
