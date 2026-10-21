// Package postgres implements the engine ports for PostgreSQL on pgx.
//
// A Session wraps a pgxpool.Pool. Ad-hoc SQL runs through pgconn with
// parameters and results in the text format, so every PostgreSQL type maps
// to a value.Value without per-type negotiation (see values.go). Catalog and
// monitor queries live in the embedded .sql files next to this code.
package postgres

import (
	"context"
	"errors"
	"fmt"
	"math"
	"net"
	"net/url"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/felipemalacarne/mesa/internal/core/connection"
	"github.com/felipemalacarne/mesa/internal/core/errs"
	"github.com/felipemalacarne/mesa/internal/engine"
)

// Engine creates PostgreSQL sessions.
type Engine struct{}

// New returns the PostgreSQL engine.
func New() *Engine { return &Engine{} }

// Name implements engine.Engine.
func (*Engine) Name() connection.Engine { return connection.Postgres }

// Capabilities implements engine.Engine: PostgreSQL supports everything Mesa models.
func (*Engine) Capabilities() engine.Capabilities {
	return engine.Capabilities{
		Schemas:          true,
		Enums:            true,
		Views:            true,
		Functions:        true,
		Sequences:        true,
		Explain:          true,
		KillSession:      true,
		EnginePanel:      true,
		DDL:              true,
		TransactionalDDL: true,
		Databases:        true,
		Roles:            true,
	}
}

// Open connects a pool to the target database and pings it once.
func (*Engine) Open(ctx context.Context, t engine.Target) (engine.Session, error) {
	cfg, err := poolConfig(t)
	if err != nil {
		return nil, errs.Wrap(errs.Invalid, "invalid connection settings for "+endpoint(t), err)
	}
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, errs.Wrap(errs.Unavailable, "cannot connect to "+endpoint(t), err)
	}
	s := &Session{pool: pool, database: t.Database, types: newTypeRegistry()}
	if err := s.Ping(ctx); err != nil {
		pool.Close()
		return nil, errs.Wrap(errs.Unavailable, "cannot connect to "+endpoint(t), err)
	}
	if err := pool.QueryRow(ctx, "select current_setting('server_version_num')::int").Scan(&s.version); err != nil {
		pool.Close()
		return nil, errs.Wrap(errs.Unavailable, "cannot read the server version of "+endpoint(t), err)
	}
	return s, nil
}

// poolConfig builds the pool configuration through pgx's own parser. The
// credentials travel in a URL assembled with net/url so no value is ever
// concatenated into a connection string by hand.
func poolConfig(t engine.Target) (*pgxpool.Config, error) {
	u := url.URL{Scheme: "postgres", Path: "/" + t.Database}
	if t.Secret != "" {
		u.User = url.UserPassword(t.User, t.Secret)
	} else {
		u.User = url.User(t.User)
	}
	q := url.Values{}
	switch {
	case strings.HasPrefix(t.Host, "/"):
		// A unix socket directory cannot sit in the URL authority.
		q.Set("host", t.Host)
		if t.Port > 0 {
			q.Set("port", strconv.Itoa(t.Port))
		}
	case t.Port > 0:
		u.Host = net.JoinHostPort(t.Host, strconv.Itoa(t.Port))
	default:
		u.Host = t.Host
	}
	tls := t.TLS
	if tls == "" {
		tls = connection.TLSPrefer
	}
	q.Set("sslmode", string(tls))
	if t.ConnectTimeout > 0 {
		q.Set("connect_timeout", strconv.Itoa(int(math.Ceil(t.ConnectTimeout.Seconds()))))
	}
	if t.AppName != "" {
		q.Set("application_name", t.AppName)
	}
	u.RawQuery = q.Encode()
	cfg, err := pgxpool.ParseConfig(u.String())
	if err != nil {
		return nil, err
	}
	if t.MaxConns > 0 {
		cfg.MaxConns = int32(t.MaxConns)
	}
	return cfg, nil
}

// endpoint names the target for messages without revealing the secret.
func endpoint(t engine.Target) string {
	host := t.Host
	if t.Port > 0 && !strings.HasPrefix(host, "/") {
		host = net.JoinHostPort(host, strconv.Itoa(t.Port))
	}
	return fmt.Sprintf("%s/%s as %s", host, t.Database, t.User)
}

// Session is one pool against one database.
type Session struct {
	pool     *pgxpool.Pool
	database string
	version  int // server_version_num, e.g. 160004
	types    *typeRegistry
}

// Ping implements engine.Session.
func (s *Session) Ping(ctx context.Context) error {
	return wrap(s.pool.Ping(ctx), "ping failed")
}

// Close implements engine.Session.
func (s *Session) Close() { s.pool.Close() }

// major is the server's major version (16 for 16.4, 9 for 9.6).
func (s *Session) major() int { return s.version / 10000 }

// wrap maps a driver error to an errs kind with a message safe to show.
// PostgreSQL's own message is kept: it names the object, never a secret.
func wrap(err error, msg string) error {
	if err == nil {
		return nil
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return &errs.Error{Kind: kindOfCode(pgErr.Code), Message: pgErr.Message, Cause: err}
	}
	switch {
	case errors.Is(err, context.DeadlineExceeded) || pgconn.Timeout(err):
		return errs.Wrap(errs.Unavailable, msg+": timed out", err)
	case errors.Is(err, context.Canceled):
		return errs.Wrap(errs.Unavailable, msg+": cancelled", err)
	}
	return errs.Wrap(errs.Unavailable, msg, err)
}

// kindOfCode classifies a SQLSTATE.
func kindOfCode(code string) errs.Kind {
	switch code {
	case "42P01", "3F000", "3D000", "42704": // undefined table, schema, database, object
		return errs.NotFound
	case "42501", "25006", "42P04": // insufficient privilege, read-only transaction
		return errs.Forbidden
	case "42P07", "42710", "42723": // duplicate table, object, function
		return errs.Conflict
	case "0A000":
		return errs.Unsupported
	}
	if len(code) < 2 {
		return errs.Unknown
	}
	switch code[:2] {
	case "22", "42", "2B", "2D", "26", "34", "20", "21":
		return errs.Invalid
	case "23":
		return errs.Conflict
	case "28":
		return errs.Forbidden
	case "08", "53", "57", "58", "XX":
		return errs.Unavailable
	}
	return errs.Unknown
}
