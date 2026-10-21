package mysql

import (
	"context"
	"database/sql"
	"os"
	"strconv"
	"testing"
	"time"

	driver "github.com/go-sql-driver/mysql"

	"github.com/felipemalacarne/mesa/internal/core/connection"
	"github.com/felipemalacarne/mesa/internal/engine"
	"github.com/felipemalacarne/mesa/internal/engine/enginetest"
)

// testDatabase is recreated for every subtest. MySQL has no schemas inside a
// database, so the fixture's schema is the database itself.
const testDatabase = "mesa_engine_test"

// TestConformance runs the engine suite against MESA_TEST_MYSQL, a
// go-sql-driver DSN such as "mesa:mesa@tcp(localhost:3306)/".
func TestConformance(t *testing.T) {
	dsn := os.Getenv("MESA_TEST_MYSQL")
	if dsn == "" {
		t.Skip("MESA_TEST_MYSQL is not set")
	}
	enginetest.Run(t, enginetest.Fixture{
		Engine:     New(),
		Open:       func(t *testing.T) engine.Session { return openFresh(t, dsn) },
		Schema:     testDatabase,
		Types:      enginetest.Types{Int: "bigint", Text: "varchar(200)", Bool: "boolean", Decimal: "decimal(10,2)", WideDecimal: "decimal(12,2)"},
		SelectOne:  "SELECT 1 AS one",
		QuoteTable: table,
	})
}

// openFresh drops and recreates the test database and opens a session on it.
func openFresh(t *testing.T, dsn string) engine.Session {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	admin, err := sql.Open("mysql", dsn)
	if err != nil {
		t.Fatalf("MESA_TEST_MYSQL: %v", err)
	}
	defer func() { _ = admin.Close() }()
	for _, q := range []string{"DROP DATABASE IF EXISTS " + testDatabase, "CREATE DATABASE " + testDatabase} {
		if _, err := admin.ExecContext(ctx, q); err != nil {
			t.Fatalf("%s: %v", q, err)
		}
	}
	t.Cleanup(func() {
		a, err := sql.Open("mysql", dsn)
		if err == nil {
			_, _ = a.Exec("DROP DATABASE IF EXISTS " + testDatabase)
			_ = a.Close()
		}
	})
	target := targetFrom(t, dsn)
	target.Database = testDatabase
	s, err := New().Open(ctx, target)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(s.Close)
	return s
}

func targetFrom(t *testing.T, dsn string) engine.Target {
	t.Helper()
	cfg, err := driver.ParseDSN(dsn)
	if err != nil {
		t.Fatalf("MESA_TEST_MYSQL: %v", err)
	}
	host, port := cfg.Addr, 3306
	if h, p, ok := cutLast(cfg.Addr, ':'); ok {
		host = h
		port, _ = strconv.Atoi(p)
	}
	return engine.Target{
		Engine: connection.MySQL, Host: host, Port: port, User: cfg.User, Secret: cfg.Passwd,
		TLS: connection.TLSPrefer, AppName: "mesa-test", ConnectTimeout: 5 * time.Second,
	}
}

func cutLast(s string, sep byte) (string, string, bool) {
	for i := len(s) - 1; i >= 0; i-- {
		if s[i] == sep {
			return s[:i], s[i+1:], true
		}
	}
	return s, "", false
}

func TestOpenUnreachable(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_, err := New().Open(ctx, engine.Target{Host: "127.0.0.1", Port: 1, Database: "x", User: "x", TLS: connection.TLSDisable, ConnectTimeout: time.Second})
	if err == nil {
		t.Fatal("Open on a closed port succeeded")
	}
}
