// Package mysql implements the engine ports for MySQL 8 on database/sql.
//
// MySQL has no schemas inside a database: the catalog reports one schema
// named after the database, and DDL qualifies tables with it. Results are read
// as text (interpolated parameters keep the driver on the text protocol), so
// every type maps to a value.Value the same way. DDL commits implicitly in
// MySQL; plans say so as a risk.
package mysql

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"

	driver "github.com/go-sql-driver/mysql"

	"github.com/felipemalacarne/mesa/internal/core/connection"
	"github.com/felipemalacarne/mesa/internal/core/errs"
	"github.com/felipemalacarne/mesa/internal/core/ident"
	"github.com/felipemalacarne/mesa/internal/engine"
)

// Engine creates MySQL sessions.
type Engine struct{}

// New returns the MySQL engine.
func New() *Engine { return &Engine{} }

// Name implements engine.Engine.
func (*Engine) Name() connection.Engine { return connection.MySQL }

// Capabilities implements engine.Engine.
func (*Engine) Capabilities() engine.Capabilities {
	return engine.Capabilities{
		Schemas:          false,
		Enums:            false, // enums are column types in MySQL
		Views:            true,
		Functions:        true,
		Sequences:        false,
		Explain:          true,
		KillSession:      true,
		EnginePanel:      true,
		DDL:              true,
		TransactionalDDL: false,
		Databases:        true,
		Roles:            true,
		Types:            columnTypes,
	}
}

// Open connects a pool to the target database and pings it once.
func (*Engine) Open(ctx context.Context, t engine.Target) (engine.Session, error) {
	cfg := config(t)
	connector, err := driver.NewConnector(cfg)
	if err != nil {
		return nil, errs.Wrap(errs.Invalid, "invalid connection settings for "+endpoint(t), err)
	}
	db := sql.OpenDB(connector)
	max := t.MaxConns
	if max <= 0 {
		max = 8
	}
	db.SetMaxOpenConns(max)
	db.SetMaxIdleConns(2)
	db.SetConnMaxIdleTime(5 * time.Minute)
	s := &Session{db: db, database: t.Database}
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, wrap(err, "cannot connect to "+endpoint(t))
	}
	if err := db.QueryRowContext(ctx, "SELECT VERSION()").Scan(&s.version); err != nil {
		_ = db.Close()
		return nil, wrap(err, "cannot read the server version of "+endpoint(t))
	}
	return s, nil
}

func config(t engine.Target) *driver.Config {
	cfg := driver.NewConfig()
	cfg.User = t.User
	cfg.Passwd = t.Secret
	cfg.DBName = t.Database
	if strings.HasPrefix(t.Host, "/") {
		cfg.Net = "unix"
		cfg.Addr = t.Host
	} else {
		cfg.Net = "tcp"
		port := t.Port
		if port == 0 {
			port = 3306
		}
		cfg.Addr = net.JoinHostPort(t.Host, strconv.Itoa(port))
	}
	switch t.TLS {
	case connection.TLSDisable:
		cfg.TLSConfig = "false"
	case connection.TLSRequire:
		cfg.TLSConfig = "skip-verify"
	case connection.TLSVerifyCA, connection.TLSVerifyFull:
		cfg.TLSConfig = "true"
	default:
		cfg.TLSConfig = "preferred"
	}
	cfg.InterpolateParams = true // text protocol for every query
	cfg.ClientFoundRows = true   // rows matched, not rows changed: row edits count one
	cfg.ParseTime = false
	cfg.MultiStatements = false
	cfg.Timeout = t.ConnectTimeout
	if cfg.Timeout == 0 {
		cfg.Timeout = 10 * time.Second
	}
	cfg.Params = map[string]string{"time_zone": "'+00:00'"}
	if t.AppName != "" {
		cfg.ConnectionAttributes = "program_name:" + t.AppName
	}
	return cfg
}

func endpoint(t engine.Target) string {
	return fmt.Sprintf("%s:%d/%s as %s", t.Host, t.Port, t.Database, t.User)
}

var _ engine.Session = (*Session)(nil)

// Session is one pool against one database.
type Session struct {
	db       *sql.DB
	database string
	version  string
}

// Ping implements engine.Session.
func (s *Session) Ping(ctx context.Context) error { return wrap(s.db.PingContext(ctx), "ping failed") }

// Close implements engine.Session.
func (s *Session) Close() { _ = s.db.Close() }

// quote renders a MySQL identifier.
func quote(n ident.Name) string {
	return "`" + strings.ReplaceAll(string(n), "`", "``") + "`"
}

// table renders `schema`.`name`; the schema is the database.
func table(q ident.Qualified) string {
	if q.Schema == "" {
		return quote(q.Name)
	}
	return quote(q.Schema) + "." + quote(q.Name)
}

// quoteLiteral renders s as a string literal, escaping what MySQL's default
// sql_mode treats specially.
func quoteLiteral(s string) string {
	r := strings.NewReplacer(`\`, `\\`, `'`, `''`, "\x00", `\0`, "\n", `\n`, "\r", `\r`, "\x1a", `\Z`)
	return "'" + r.Replace(s) + "'"
}

// wrap maps a driver error to an errs kind with a message safe to show.
func wrap(err error, msg string) error {
	if err == nil {
		return nil
	}
	var me *driver.MySQLError
	if errors.As(err, &me) {
		return &errs.Error{Kind: kindOf(me), Message: me.Message, Cause: err}
	}
	switch {
	case errors.Is(err, context.DeadlineExceeded):
		return errs.Wrap(errs.Unavailable, msg+": timed out", err)
	case errors.Is(err, context.Canceled):
		return errs.Wrap(errs.Unavailable, msg+": cancelled", err)
	case errors.Is(err, sql.ErrNoRows):
		return errs.Wrap(errs.NotFound, msg, err)
	}
	return errs.Wrap(errs.Unavailable, msg, err)
}

func kindOf(e *driver.MySQLError) errs.Kind {
	switch e.Number {
	case 1146, 1049, 1051, 1091, 1305, 1396: // no such table, database, drop target, key, routine, user
		return errs.NotFound
	case 1792, 1044, 1045, 1142, 1143, 1227, 1290: // read-only transaction, access denied, server read-only
		return errs.Forbidden
	case 1050, 1060, 1061, 1062, 1007, 1451, 1452, 1022, 3819: // exists, duplicates, foreign keys, check violated
		return errs.Conflict
	case 1064, 1054, 1149, 1292, 1366, 1406, 1048, 1265, 1264: // syntax, unknown column, bad values
		return errs.Invalid
	case 1235:
		return errs.Unsupported
	}
	state := string(e.SQLState[:])
	switch {
	case strings.HasPrefix(state, "42"), strings.HasPrefix(state, "22"):
		return errs.Invalid
	case strings.HasPrefix(state, "23"):
		return errs.Conflict
	case strings.HasPrefix(state, "28"), strings.HasPrefix(state, "25"):
		return errs.Forbidden
	case strings.HasPrefix(state, "08"):
		return errs.Unavailable
	}
	return errs.Unknown
}
