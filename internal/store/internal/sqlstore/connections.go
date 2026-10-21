package sqlstore

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/google/uuid"

	"github.com/felipemalacarne/mesa/internal/core/connection"
	"github.com/felipemalacarne/mesa/internal/core/errs"
)

// "user" is quoted because PostgreSQL reserves it.
const connectionColumns = `id, name, engine, environment, host, port, database, "user", tls, read_only, created_at, updated_at`

type connections struct{ *Store }

func (c connections) List(ctx context.Context) ([]connection.Connection, error) {
	out, err := selectAll(ctx, c.Store, scanConnection,
		`SELECT `+connectionColumns+` FROM connections ORDER BY name, id`)
	if err != nil {
		return nil, fail("list connections", err)
	}
	return out, nil
}

func (c connections) Get(ctx context.Context, id uuid.UUID) (*connection.Connection, error) {
	conn, err := scanConnection(c.queryRow(ctx,
		`SELECT `+connectionColumns+` FROM connections WHERE id = ?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, errs.Newf(errs.NotFound, "connection %s not found", id)
	}
	if err != nil {
		return nil, fail("get connection", err)
	}
	return &conn, nil
}

func (c connections) Put(ctx context.Context, conn connection.Connection) error {
	err := c.exec(ctx, `
		INSERT INTO connections (`+connectionColumns+`)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT (id) DO UPDATE SET
			name = excluded.name, engine = excluded.engine,
			environment = excluded.environment, host = excluded.host,
			port = excluded.port, database = excluded.database,
			"user" = excluded."user", tls = excluded.tls,
			read_only = excluded.read_only, created_at = excluded.created_at,
			updated_at = excluded.updated_at`,
		conn.ID, conn.Name, string(conn.Engine), string(conn.Environment), conn.Host, conn.Port,
		conn.Database, conn.User, string(conn.TLS), conn.ReadOnly,
		c.dialect.Time(conn.CreatedAt), c.dialect.Time(conn.UpdatedAt))
	if err != nil {
		return fail("put connection", err)
	}
	return nil
}

// Delete removes the connection; deleting one that is already gone is fine.
func (c connections) Delete(ctx context.Context, id uuid.UUID) error {
	if err := c.exec(ctx, `DELETE FROM connections WHERE id = ?`, id); err != nil {
		return fail("delete connection", err)
	}
	return nil
}

func (c connections) TouchOpened(ctx context.Context, id uuid.UUID, at time.Time) error {
	ok, err := c.affected(ctx, `UPDATE connections SET last_opened_at = ? WHERE id = ?`,
		c.dialect.Time(at), id)
	if err != nil {
		return fail("touch connection", err)
	}
	if !ok {
		return errs.Newf(errs.NotFound, "connection %s not found", id)
	}
	return nil
}

func (c connections) LastOpened(ctx context.Context) (map[uuid.UUID]time.Time, error) {
	type opened struct {
		id uuid.UUID
		at time.Time
	}
	rows, err := selectAll(ctx, c.Store, func(sc scanner) (opened, error) {
		var o opened
		var at scanTime
		err := sc.Scan(&o.id, &at)
		o.at = at.t
		return o, err
	}, `SELECT id, last_opened_at FROM connections WHERE last_opened_at IS NOT NULL`)
	if err != nil {
		return nil, fail("list last opened", err)
	}
	out := make(map[uuid.UUID]time.Time, len(rows))
	for _, o := range rows {
		out[o.id] = o.at
	}
	return out, nil
}

func scanConnection(sc scanner) (connection.Connection, error) {
	var c connection.Connection
	var created, updated scanTime
	err := sc.Scan(&c.ID, &c.Name, &c.Engine, &c.Environment, &c.Host, &c.Port,
		&c.Database, &c.User, &c.TLS, &c.ReadOnly, &created, &updated)
	c.CreatedAt, c.UpdatedAt = created.t, updated.t
	return c, err
}
