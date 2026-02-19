// Package persistence provides a factory for creating repositories and managing database connections for both SQLite and PostgreSQL, including running migrations on startup.
package persistence

import (
	"context"
	"database/sql"
	"fmt"
	"log"

	"github.com/felipemalacarne/mesa/internal/config"
	"github.com/felipemalacarne/mesa/internal/domain/connection"
	"github.com/felipemalacarne/mesa/internal/infrastructure/postgres"
	"github.com/felipemalacarne/mesa/internal/infrastructure/sqlite"
	"github.com/golang-migrate/migrate/v4"
	mig_postgres "github.com/golang-migrate/migrate/v4/database/pgx/v5"
	mig_sqlite "github.com/golang-migrate/migrate/v4/database/sqlite"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/jackc/pgx/v5/stdlib"
)

type Store struct {
	ConnectionRepo connection.Repository
	Close          func()
}

func New(ctx context.Context, cfg config.Config) (*Store, error) {
	if cfg.DBDriver == "sqlite" {
		return newSQLiteStore(cfg.DatabaseURL)
	}
	return newPostgresStore(ctx, cfg.DatabaseURL)
}

func newSQLiteStore(dsn string) (*Store, error) {
	db, err := sqlite.NewDB(dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to sqlite: %w", err)
	}

	if err := runSQLiteMigrations(db); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to run sqlite migrations: %w", err)
	}

	log.Println("Connected to SQLite and migrations applied.")

	return &Store{
		ConnectionRepo: sqlite.NewConnectionRepository(db),
		Close:          func() { db.Close() },
	}, nil
}

func newPostgresStore(ctx context.Context, dsn string) (*Store, error) {
	pool, err := postgres.NewPool(ctx, dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to postgres: %w", err)
	}

	// For migration purposes, we need a *sql.DB from the pool
	migrationDB := stdlib.OpenDBFromPool(pool)

	if err := runPostgresMigrations(migrationDB); err != nil {
		pool.Close()
		return nil, fmt.Errorf("failed to run postgres migrations: %w", err)
	}

	log.Println("Connected to Postgres and migrations applied.")

	return &Store{
		ConnectionRepo: postgres.NewConnectionRepository(pool),
		Close:          func() { pool.Close() },
	}, nil
}

func runSQLiteMigrations(db *sql.DB) error {
	driver, err := mig_sqlite.WithInstance(db, &mig_sqlite.Config{})
	if err != nil {
		return fmt.Errorf("failed to create sqlite migration driver: %w", err)
	}

	m, err := migrate.NewWithDatabaseInstance(
		"file://internal/infrastructure/sqlite/migrations",
		"sqlite",
		driver,
	)
	if err != nil {
		return fmt.Errorf("failed to create sqlite migration instance: %w", err)
	}

	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		return fmt.Errorf("failed to run sqlite up migrations: %w", err)
	}

	return nil
}

func runPostgresMigrations(db *sql.DB) error {
	driver, err := mig_postgres.WithInstance(db, &mig_postgres.Config{})
	if err != nil {
		return fmt.Errorf("failed to create postgres migration driver: %w", err)
	}

	m, err := migrate.NewWithDatabaseInstance(
		"file://internal/infrastructure/postgres/migrations",
		"pgx",
		driver,
	)
	if err != nil {
		return fmt.Errorf("failed to create postgres migration instance: %w", err)
	}

	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		return fmt.Errorf("failed to run postgres up migrations: %w", err)
	}

	return nil
}
