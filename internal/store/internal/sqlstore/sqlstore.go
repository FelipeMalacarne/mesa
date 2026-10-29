// Package sqlstore implements the store ports over database/sql. The SQL is
// shared by the SQLite and PostgreSQL backends; a Dialect supplies what
// differs between them: placeholder syntax and how a timestamp is bound.
// Everything else round-trips through values both drivers understand: ids as
// text, blobs and JSON as bytes, booleans and integers as themselves.
package sqlstore

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/felipemalacarne/mesa/internal/core/errs"
	"github.com/felipemalacarne/mesa/internal/store"
)

// Dialect is what a backend tells the shared SQL about itself.
type Dialect interface {
	// Rebind rewrites the ? placeholders of a query into the dialect's own
	// form. Queries in this package never contain a literal question mark.
	Rebind(query string) string
	// Time converts a timestamp into the value bound for a timestamp column.
	Time(t time.Time) any
}

// DefaultLimit bounds List calls given a limit of zero or less.
const DefaultLimit = 100

// Store implements store.Store over an open database handle, which it owns.
type Store struct {
	db      *sql.DB
	dialect Dialect
}

// New wraps db, whose schema must already be migrated.
func New(db *sql.DB, dialect Dialect) *Store {
	return &Store{db: db, dialect: dialect}
}

func (s *Store) Connections() store.Connections { return connections{s} }
func (s *Store) Secrets() store.Secrets         { return secrets{s} }
func (s *Store) Audit() store.Audit             { return audit{s} }
func (s *Store) Layouts() store.Layouts         { return layouts{s} }
func (s *Store) History() store.History         { return history{s} }
func (s *Store) Queries() store.SavedQueries    { return savedQueries{s} }
func (s *Store) Metrics() store.Metrics         { return metrics{s} }
func (s *Store) Users() store.Users             { return users{s} }
func (s *Store) Tokens() store.Tokens           { return tokens{s} }

// Close closes the database handle.
func (s *Store) Close() error { return s.db.Close() }

func (s *Store) exec(ctx context.Context, query string, args ...any) error {
	_, err := s.db.ExecContext(ctx, s.dialect.Rebind(query), args...)
	return err
}

// affected runs a statement and reports whether it touched any row.
func (s *Store) affected(ctx context.Context, query string, args ...any) (bool, error) {
	res, err := s.db.ExecContext(ctx, s.dialect.Rebind(query), args...)
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	return n > 0, err
}

func (s *Store) queryRow(ctx context.Context, query string, args ...any) *sql.Row {
	return s.db.QueryRowContext(ctx, s.dialect.Rebind(query), args...)
}

// nullTime binds an optional timestamp, NULL when nil.
func (s *Store) nullTime(t *time.Time) any {
	if t == nil {
		return nil
	}
	return s.dialect.Time(*t)
}

// scanner is what *sql.Row and *sql.Rows have in common.
type scanner interface {
	Scan(dest ...any) error
}

// selectAll runs query and collects one T per row.
func selectAll[T any](ctx context.Context, s *Store, scan func(scanner) (T, error), query string, args ...any) ([]T, error) {
	rows, err := s.db.QueryContext(ctx, s.dialect.Rebind(query), args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []T
	for rows.Next() {
		v, err := scan(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

func limitOrDefault(limit int) int {
	if limit <= 0 {
		return DefaultLimit
	}
	return limit
}

// fail wraps a driver error so a client sees a safe message; the cause stays
// attached for logs and errors.Is.
func fail(op string, err error) error {
	return errs.Wrap(errs.Unavailable, "store: "+op, err)
}

// scanTime receives a timestamp column from either backend: a time.Time
// from PostgreSQL, RFC 3339 text from SQLite. NULL leaves it invalid.
type scanTime struct {
	t     time.Time
	valid bool
}

func (s *scanTime) Scan(src any) error {
	s.t, s.valid = time.Time{}, false
	switch v := src.(type) {
	case nil:
		return nil
	case time.Time:
		s.t = v
	case string:
		return s.parse(v)
	case []byte:
		return s.parse(string(v))
	default:
		return fmt.Errorf("cannot scan %T into time", src)
	}
	s.valid = true
	return nil
}

func (s *scanTime) parse(text string) error {
	t, err := time.Parse(time.RFC3339Nano, text)
	if err != nil {
		return err
	}
	s.t, s.valid = t, true
	return nil
}

// ptr is the value for an optional field: nil when the column was NULL.
func (s *scanTime) ptr() *time.Time {
	if !s.valid {
		return nil
	}
	t := s.t
	return &t
}
