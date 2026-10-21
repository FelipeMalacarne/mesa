// Package sqlite is Mesa's own store on SQLite, for the desktop app and the
// CLI: one file under the config directory, or ":memory:" for tests.
package sqlite

import (
	"context"
	"database/sql"
	"embed"
	"net/url"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite" // registers the "sqlite" driver

	"github.com/felipemalacarne/mesa/internal/core/errs"
	"github.com/felipemalacarne/mesa/internal/store"
	"github.com/felipemalacarne/mesa/internal/store/internal/sqlstore"
)

//go:embed migrations/*.sql
var migrations embed.FS

// Memory is the path of a private in-memory database.
const Memory = ":memory:"

// Open opens the database at path, creating it and its parent directories
// when needed, applies pending migrations and returns the store. Every
// pooled connection runs in WAL mode with foreign keys on and waits up to
// five seconds for a locked database.
func Open(ctx context.Context, path string) (store.Store, error) {
	if path == "" {
		return nil, errs.New(errs.Invalid, "sqlite store path is empty")
	}
	if path != Memory {
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			return nil, errs.Wrap(errs.Unavailable, "create store directory", err)
		}
	}
	db, err := sql.Open("sqlite", dsn(path))
	if err != nil {
		return nil, errs.Wrap(errs.Unavailable, "open sqlite store", err)
	}
	if path == Memory {
		// Each pooled connection would otherwise get its own empty database.
		db.SetMaxOpenConns(1)
	}
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, errs.Wrap(errs.Unavailable, "open sqlite store", err)
	}
	if err := store.Migrate(ctx, db, migrations, "sqlite"); err != nil {
		_ = db.Close()
		return nil, errs.Wrap(errs.Unavailable, "migrate sqlite store", err)
	}
	return sqlstore.New(db, dialect{}), nil
}

// dsn appends the per-connection settings; the driver strips them before
// handing the path to SQLite and applies them to every new connection.
func dsn(path string) string {
	q := url.Values{
		"_pragma": {"busy_timeout(5000)", "journal_mode(WAL)", "foreign_keys(ON)"},
		"_txlock": {"immediate"}, // writers take the lock at BEGIN, so they queue instead of deadlocking
	}
	return path + "?" + q.Encode()
}

// timeLayout is RFC 3339 in UTC with a fixed nine fractional digits, so the
// text order of a timestamp column is its time order.
const timeLayout = "2006-01-02T15:04:05.000000000Z07:00"

// dialect binds values the way the SQLite schema stores them.
type dialect struct{}

func (dialect) Rebind(query string) string { return query }

func (dialect) Time(t time.Time) any { return t.UTC().Format(timeLayout) }
