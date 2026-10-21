// Package postgres is Mesa's own store on PostgreSQL, for server mode where
// several replicas share one database.
package postgres

import (
	"context"
	"embed"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"

	"github.com/felipemalacarne/mesa/internal/core/errs"
	"github.com/felipemalacarne/mesa/internal/store"
	"github.com/felipemalacarne/mesa/internal/store/internal/sqlstore"
)

//go:embed migrations/*.sql
var migrations embed.FS

// Open connects with dsn (a URL or key=value string, as pgx parses it),
// applies pending migrations and returns the store. Point the DSN at a
// dedicated database or schema (options=-c search_path=...): the tables
// carry plain names.
func Open(ctx context.Context, dsn string) (store.Store, error) {
	cfg, err := pgx.ParseConfig(dsn)
	if err != nil {
		return nil, errs.Wrap(errs.Invalid, "invalid postgres store DSN", err)
	}
	if cfg.RuntimeParams["application_name"] == "" {
		cfg.RuntimeParams["application_name"] = "mesa-store"
	}
	db := stdlib.OpenDB(*cfg)
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, errs.Wrap(errs.Unavailable, "connect to postgres store", err)
	}
	if err := store.Migrate(ctx, db, migrations, "postgres"); err != nil {
		_ = db.Close()
		return nil, errs.Wrap(errs.Unavailable, "migrate postgres store", err)
	}
	return sqlstore.New(db, dialect{}), nil
}

// dialect binds values the way the PostgreSQL schema stores them.
type dialect struct{}

// Rebind numbers the placeholders: each ? becomes $1, $2, ...
func (dialect) Rebind(query string) string {
	var b strings.Builder
	b.Grow(len(query) + 16)
	n := 0
	for i := 0; i < len(query); i++ {
		if query[i] != '?' {
			b.WriteByte(query[i])
			continue
		}
		n++
		b.WriteByte('$')
		b.WriteString(strconv.Itoa(n))
	}
	return b.String()
}

func (dialect) Time(t time.Time) any { return t }
