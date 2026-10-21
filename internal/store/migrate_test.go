package store_test

import (
	"context"
	"database/sql"
	"strings"
	"testing"
	"testing/fstest"

	_ "modernc.org/sqlite"

	"github.com/felipemalacarne/mesa/internal/store"
)

func openMemory(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func TestMigrateAppliesInOrderOnce(t *testing.T) {
	ctx := context.Background()
	db := openMemory(t)
	fsys := fstest.MapFS{
		// Listed out of order on purpose: the version decides.
		"migrations/0002_seed.sql":  {Data: []byte(`INSERT INTO things (name) VALUES ('one');`)},
		"migrations/0001_init.sql":  {Data: []byte(`CREATE TABLE things (id INTEGER PRIMARY KEY, name TEXT NOT NULL);`)},
		"migrations/0010_later.sql": {Data: []byte(`ALTER TABLE things ADD COLUMN note TEXT;`)},
		"migrations/README.md":      {Data: []byte("not a migration")},
	}

	for range 2 { // the second run must change nothing
		if err := store.Migrate(ctx, db, fsys, "sqlite"); err != nil {
			t.Fatalf("Migrate: %v", err)
		}
	}

	var things int
	if err := db.QueryRowContext(ctx, `SELECT count(*) FROM things`).Scan(&things); err != nil {
		t.Fatal(err)
	}
	if things != 1 {
		t.Fatalf("seed ran %d times, want once", things)
	}
	rows, err := db.QueryContext(ctx, `SELECT version, name FROM mesa_migrations ORDER BY version`)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = rows.Close() }()
	var applied []string
	for rows.Next() {
		var version int
		var name string
		if err := rows.Scan(&version, &name); err != nil {
			t.Fatal(err)
		}
		applied = append(applied, name)
	}
	if got := strings.Join(applied, ","); got != "init,seed,later" {
		t.Fatalf("applied = %s, want init,seed,later", got)
	}
}

func TestMigrateRollsBackAFailedFile(t *testing.T) {
	ctx := context.Background()
	db := openMemory(t)
	fsys := fstest.MapFS{
		"0001_init.sql": {Data: []byte(`CREATE TABLE things (id INTEGER PRIMARY KEY);`)},
		"0002_bad.sql":  {Data: []byte(`CREATE TABLE other (id INTEGER PRIMARY KEY); INSERT INTO missing VALUES (1);`)},
	}
	err := store.Migrate(ctx, db, fsys, "sqlite")
	if err == nil || !strings.Contains(err.Error(), "0002_bad") {
		t.Fatalf("Migrate = %v, want an error naming 0002_bad", err)
	}
	var n int
	if err := db.QueryRowContext(ctx, `SELECT count(*) FROM mesa_migrations`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("%d migrations recorded, want only the first", n)
	}
	if err := db.QueryRowContext(ctx, `SELECT count(*) FROM other`).Scan(&n); err == nil {
		t.Fatal("table from the failed migration exists; want it rolled back")
	}
}

func TestMigrateRejectsBadInput(t *testing.T) {
	ctx := context.Background()
	db := openMemory(t)
	cases := map[string]struct {
		fsys    fstest.MapFS
		dialect string
	}{
		"unknown dialect": {fstest.MapFS{"0001_init.sql": {Data: []byte(`SELECT 1;`)}}, "oracle"},
		"unnumbered file": {fstest.MapFS{"init.sql": {Data: []byte(`SELECT 1;`)}}, "sqlite"},
		"nameless file":   {fstest.MapFS{"0001_.sql": {Data: []byte(`SELECT 1;`)}}, "sqlite"},
		"duplicate version": {fstest.MapFS{
			"0001_a.sql": {Data: []byte(`SELECT 1;`)},
			"0001_b.sql": {Data: []byte(`SELECT 1;`)},
		}, "sqlite"},
	}
	for name, tc := range cases {
		if err := store.Migrate(ctx, db, tc.fsys, tc.dialect); err == nil {
			t.Errorf("%s: Migrate succeeded, want an error", name)
		}
	}
}
