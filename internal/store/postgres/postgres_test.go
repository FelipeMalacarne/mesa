package postgres_test

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/url"
	"os"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"

	"github.com/felipemalacarne/mesa/internal/core/errs"
	"github.com/felipemalacarne/mesa/internal/store"
	"github.com/felipemalacarne/mesa/internal/store/postgres"
	"github.com/felipemalacarne/mesa/internal/store/storetest"
)

// TestStore runs the conformance suite against a scratch database created
// on the server MESA_TEST_PG points at; it skips when the variable is unset.
func TestStore(t *testing.T) {
	dsn := os.Getenv("MESA_TEST_PG")
	if dsn == "" {
		t.Skip("MESA_TEST_PG is not set")
	}
	storetest.Run(t, func(t *testing.T) store.Store {
		return openScratch(t, dsn)
	})
}

func TestOpenRejectsBadDSN(t *testing.T) {
	_, err := postgres.Open(context.Background(), "this is not a dsn")
	if !errs.Is(err, errs.Invalid) {
		t.Fatalf("open with a bad DSN: got %v, want an Invalid error", err)
	}
}

// openScratch creates a database named mesa_store_test_<random> next to the
// one in dsn, opens the store on it and drops it when the test ends.
func openScratch(t *testing.T, dsn string) store.Store {
	t.Helper()
	ctx := context.Background()

	cfg, err := pgx.ParseConfig(dsn)
	if err != nil {
		t.Fatalf("MESA_TEST_PG: %v", err)
	}
	admin := stdlib.OpenDB(*cfg)
	name := "mesa_store_test_" + randomSuffix(t)
	quoted := pgx.Identifier{name}.Sanitize()
	if _, err := admin.ExecContext(ctx, "CREATE DATABASE "+quoted); err != nil {
		_ = admin.Close()
		t.Fatalf("create scratch database: %v", err)
	}
	t.Cleanup(func() {
		if _, err := admin.ExecContext(ctx, "DROP DATABASE "+quoted+" WITH (FORCE)"); err != nil {
			t.Errorf("drop scratch database %s: %v", name, err)
		}
		_ = admin.Close()
	})

	st, err := postgres.Open(ctx, withDatabase(dsn, name))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() }) // runs before the drop
	return st
}

// withDatabase points dsn at another database on the same server.
func withDatabase(dsn, name string) string {
	if u, err := url.Parse(dsn); err == nil && strings.HasPrefix(u.Scheme, "postgres") {
		u.Path = "/" + name
		return u.String()
	}
	return dsn + " dbname=" + name // key=value form: the last dbname wins
}

func randomSuffix(t *testing.T) string {
	t.Helper()
	var b [4]byte
	if _, err := rand.Read(b[:]); err != nil {
		t.Fatal(err)
	}
	return hex.EncodeToString(b[:])
}
