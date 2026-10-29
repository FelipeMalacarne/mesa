package sqlstore

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/google/uuid"

	"github.com/felipemalacarne/mesa/internal/core/account"
	"github.com/felipemalacarne/mesa/internal/core/errs"
)

const userColumns = `id, subject, email, name, role, created_at, updated_at, last_seen_at`

type users struct{ *Store }

func (u users) List(ctx context.Context) ([]account.User, error) {
	out, err := selectAll(ctx, u.Store, scanUser, `SELECT `+userColumns+` FROM users ORDER BY name, id`)
	if err != nil {
		return nil, fail("list users", err)
	}
	return out, nil
}

func (u users) Get(ctx context.Context, id uuid.UUID) (*account.User, error) {
	return u.one(ctx, "id", id)
}

func (u users) GetBySubject(ctx context.Context, subject string) (*account.User, error) {
	return u.one(ctx, "subject", subject)
}

// one finds a user by a unique column; column is one of ours, never input.
func (u users) one(ctx context.Context, column string, value any) (*account.User, error) {
	usr, err := scanUser(u.queryRow(ctx, `SELECT `+userColumns+` FROM users WHERE `+column+` = ?`, value))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, errs.New(errs.NotFound, "user not found")
	}
	if err != nil {
		return nil, fail("get user", err)
	}
	return &usr, nil
}

func (u users) Put(ctx context.Context, usr account.User) error {
	err := u.exec(ctx, `
		INSERT INTO users (`+userColumns+`)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT (id) DO UPDATE SET
			subject = excluded.subject, email = excluded.email, name = excluded.name,
			role = excluded.role, updated_at = excluded.updated_at,
			last_seen_at = excluded.last_seen_at`,
		usr.ID, usr.Subject, usr.Email, usr.Name, string(usr.Role),
		u.dialect.Time(usr.CreatedAt), u.dialect.Time(usr.UpdatedAt), u.nullTime(usr.LastSeenAt))
	if err != nil {
		return fail("put user", err)
	}
	return nil
}

func (u users) TouchSeen(ctx context.Context, id uuid.UUID, at time.Time) error {
	if err := u.exec(ctx, `UPDATE users SET last_seen_at = ? WHERE id = ?`, u.dialect.Time(at), id); err != nil {
		return fail("touch user", err)
	}
	return nil
}

func scanUser(sc scanner) (account.User, error) {
	var usr account.User
	var created, updated, seen scanTime
	err := sc.Scan(&usr.ID, &usr.Subject, &usr.Email, &usr.Name, &usr.Role, &created, &updated, &seen)
	usr.CreatedAt, usr.UpdatedAt, usr.LastSeenAt = created.t, updated.t, seen.ptr()
	return usr, err
}
