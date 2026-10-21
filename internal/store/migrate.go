package store

import (
	"cmp"
	"context"
	"database/sql"
	"fmt"
	"io/fs"
	"path"
	"slices"
	"strconv"
	"strings"
)

// Migrate applies the SQL files in fsys that mesa_migrations does not record
// yet, lowest version first, each file in its own transaction. Files are
// named like 0001_init.sql: the leading number is the version, the rest the
// name. dialect is "sqlite" or "postgres". Running Migrate again is a no-op;
// on PostgreSQL concurrent runs serialise on an advisory lock.
func Migrate(ctx context.Context, db *sql.DB, fsys fs.FS, dialect string) error {
	d, err := migrationDialect(dialect)
	if err != nil {
		return err
	}
	migrations, err := readMigrations(fsys)
	if err != nil {
		return err
	}
	err = inTx(ctx, db, d.lock, func(tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, d.createTable)
		return err
	})
	if err != nil {
		return fmt.Errorf("store: create mesa_migrations: %w", err)
	}
	for _, m := range migrations {
		if err := applyMigration(ctx, db, d, m); err != nil {
			return fmt.Errorf("store: migration %04d_%s: %w", m.version, m.name, err)
		}
	}
	return nil
}

type migration struct {
	version int
	name    string
	sql     string
}

// dialectSQL is what differs between backends in the bookkeeping table.
type dialectSQL struct {
	createTable string
	applied     string // counts rows for one version
	insert      string // records one version
	lock        string // run first in every transaction; empty for none
}

func migrationDialect(name string) (dialectSQL, error) {
	switch name {
	case "sqlite":
		return dialectSQL{
			createTable: `CREATE TABLE IF NOT EXISTS mesa_migrations (
				version    INTEGER PRIMARY KEY,
				name       TEXT NOT NULL,
				applied_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now'))
			)`,
			applied: `SELECT count(*) FROM mesa_migrations WHERE version = ?`,
			insert:  `INSERT INTO mesa_migrations (version, name) VALUES (?, ?)`,
		}, nil
	case "postgres":
		return dialectSQL{
			createTable: `CREATE TABLE IF NOT EXISTS mesa_migrations (
				version    integer PRIMARY KEY,
				name       text NOT NULL,
				applied_at timestamptz NOT NULL DEFAULT now()
			)`,
			applied: `SELECT count(*) FROM mesa_migrations WHERE version = $1`,
			insert:  `INSERT INTO mesa_migrations (version, name) VALUES ($1, $2)`,
			// One lock for all of Mesa's migrations; the key itself is arbitrary.
			lock: `SELECT pg_advisory_xact_lock(727310)`,
		}, nil
	}
	return dialectSQL{}, fmt.Errorf("store: unknown migration dialect %q", name)
}

// readMigrations collects every .sql file in fsys, sorted by version.
func readMigrations(fsys fs.FS) ([]migration, error) {
	var out []migration
	seen := map[int]string{}
	err := fs.WalkDir(fsys, ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(p, ".sql") {
			return nil
		}
		version, name, err := parseMigrationName(path.Base(p))
		if err != nil {
			return err
		}
		if prev, dup := seen[version]; dup {
			return fmt.Errorf("store: migrations %s and %s share version %d", prev, p, version)
		}
		seen[version] = p
		body, err := fs.ReadFile(fsys, p)
		if err != nil {
			return err
		}
		out = append(out, migration{version: version, name: name, sql: string(body)})
		return nil
	})
	if err != nil {
		return nil, err
	}
	slices.SortFunc(out, func(a, b migration) int { return cmp.Compare(a.version, b.version) })
	return out, nil
}

// parseMigrationName splits "0001_init.sql" into 1 and "init".
func parseMigrationName(base string) (int, string, error) {
	num, name, ok := strings.Cut(strings.TrimSuffix(base, ".sql"), "_")
	version, err := strconv.Atoi(num)
	if !ok || name == "" || err != nil || version <= 0 {
		return 0, "", fmt.Errorf("store: migration %q is not named NNNN_name.sql", base)
	}
	return version, name, nil
}

func applyMigration(ctx context.Context, db *sql.DB, d dialectSQL, m migration) error {
	return inTx(ctx, db, d.lock, func(tx *sql.Tx) error {
		var n int
		if err := tx.QueryRowContext(ctx, d.applied, m.version).Scan(&n); err != nil {
			return err
		}
		if n > 0 {
			return nil
		}
		if _, err := tx.ExecContext(ctx, m.sql); err != nil {
			return err
		}
		_, err := tx.ExecContext(ctx, d.insert, m.version, m.name)
		return err
	})
}

// inTx runs fn in a transaction, taking lock first when there is one.
func inTx(ctx context.Context, db *sql.DB, lock string, fn func(*sql.Tx) error) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }() // no-op once committed
	if lock != "" {
		if _, err := tx.ExecContext(ctx, lock); err != nil {
			return err
		}
	}
	if err := fn(tx); err != nil {
		return err
	}
	return tx.Commit()
}
