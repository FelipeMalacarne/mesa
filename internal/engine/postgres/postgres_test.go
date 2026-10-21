package postgres

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/url"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/felipemalacarne/mesa/internal/core/connection"
	"github.com/felipemalacarne/mesa/internal/engine"
	"github.com/felipemalacarne/mesa/internal/engine/enginetest"
)

// TestConformance runs the engine suite against MESA_TEST_PG, one fresh
// database per subtest.
func TestConformance(t *testing.T) {
	dsn := os.Getenv("MESA_TEST_PG")
	if dsn == "" {
		t.Skip("MESA_TEST_PG is not set")
	}
	enginetest.Run(t, enginetest.Fixture{
		Engine:    New(),
		Open:      func(t *testing.T) engine.Session { return openFresh(t, dsn) },
		Schema:    "public",
		Types:     enginetest.Types{Int: "bigint", Text: "text", Bool: "boolean", Decimal: "numeric(10,2)", WideDecimal: "numeric(12,2)"},
		SelectOne: "SELECT 1 AS one",
	})
}

// openFresh creates a throwaway database and opens a session on it.
func openFresh(t *testing.T, dsn string) engine.Session {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	admin, err := pgx.Connect(ctx, dsn)
	if err != nil {
		t.Fatalf("connect admin: %v", err)
	}
	defer func() { _ = admin.Close(context.Background()) }()
	b := make([]byte, 6)
	_, _ = rand.Read(b)
	name := "mesa_engine_test_" + hex.EncodeToString(b)
	if _, err := admin.Exec(ctx, "CREATE DATABASE "+name); err != nil {
		t.Fatalf("create database: %v", err)
	}
	t.Cleanup(func() {
		c, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		a, err := pgx.Connect(c, dsn)
		if err != nil {
			t.Logf("cleanup connect: %v", err)
			return
		}
		defer func() { _ = a.Close(context.Background()) }()
		if _, err := a.Exec(c, "DROP DATABASE IF EXISTS "+name+" WITH (FORCE)"); err != nil {
			t.Logf("cleanup drop: %v", err)
		}
	})
	target := targetFrom(t, dsn)
	target.Database = name
	s, err := New().Open(ctx, target)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(s.Close)
	return s
}

func targetFrom(t *testing.T, dsn string) engine.Target {
	t.Helper()
	u, err := url.Parse(dsn)
	if err != nil {
		t.Fatalf("MESA_TEST_PG: %v", err)
	}
	port, _ := strconv.Atoi(u.Port())
	pass, _ := u.User.Password()
	tls := connection.TLSMode(u.Query().Get("sslmode"))
	if tls == "" {
		tls = connection.TLSPrefer
	}
	return engine.Target{
		Engine: connection.Postgres, Host: u.Hostname(), Port: port, User: u.User.Username(), Secret: pass,
		TLS: tls, AppName: "mesa-test", ConnectTimeout: 5 * time.Second,
	}
}

func TestOpenUnreachable(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_, err := New().Open(ctx, engine.Target{Host: "127.0.0.1", Port: 1, Database: "x", User: "x", TLS: connection.TLSDisable, ConnectTimeout: time.Second})
	if err == nil {
		t.Fatal("Open on a closed port succeeded")
	}
}
