// Package connection holds the Connection entity: how Mesa reaches a database
// server. It never holds a secret; secrets live in the vault under the
// connection's ID.
package connection

import (
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/felipemalacarne/mesa/internal/core/errs"
)

// Engine names a database engine Mesa can talk to.
type Engine string

const (
	Postgres Engine = "postgres"
	MySQL    Engine = "mysql"
)

// ParseEngine accepts the engine names Mesa knows.
func ParseEngine(s string) (Engine, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "postgres", "postgresql", "pg":
		return Postgres, nil
	case "mysql":
		return MySQL, nil
	}
	return "", errs.Newf(errs.Invalid, "unknown engine %q (supported: postgres, mysql)", s)
}

// Environment classifies what a connection points at. It drives colour,
// read-only defaults and confirmation rules.
type Environment string

const (
	Local       Environment = "local"
	Development Environment = "development"
	Staging     Environment = "staging"
	Production  Environment = "production"
)

// ParseEnvironment accepts the four environments.
func ParseEnvironment(s string) (Environment, error) {
	switch Environment(strings.ToLower(strings.TrimSpace(s))) {
	case Local, Development, Staging, Production:
		return Environment(strings.ToLower(strings.TrimSpace(s))), nil
	case "":
		return Local, nil
	}
	return "", errs.Newf(errs.Invalid, "unknown environment %q", s)
}

// TLSMode mirrors the common sslmode values.
type TLSMode string

const (
	TLSDisable    TLSMode = "disable"
	TLSPrefer     TLSMode = "prefer"
	TLSRequire    TLSMode = "require"
	TLSVerifyCA   TLSMode = "verify-ca"
	TLSVerifyFull TLSMode = "verify-full"
)

// Connection is a server Mesa can reach. Database is the default database to
// open; the explorer can open any database the role can see.
type Connection struct {
	ID          uuid.UUID
	Name        string
	Engine      Engine
	Environment Environment
	Host        string
	Port        int
	Database    string
	User        string
	TLS         TLSMode
	ReadOnly    bool // true forces read-only regardless of environment
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// Input is what a person fills in; New validates it into a Connection.
type Input struct {
	Name        string
	Engine      string
	Environment string
	Host        string
	Port        int
	Database    string
	User        string
	TLS         string
	ReadOnly    *bool // nil = default for the environment
}

// New validates in and returns a Connection with a fresh ID.
func New(in Input, now time.Time) (Connection, error) {
	c := Connection{CreatedAt: now, UpdatedAt: now}
	id, err := uuid.NewV7()
	if err != nil {
		return c, err
	}
	c.ID = id
	return c, c.apply(in, now)
}

// Update applies in to an existing connection, keeping its ID and CreatedAt.
func (c *Connection) Update(in Input, now time.Time) error {
	return c.apply(in, now)
}

func (c *Connection) apply(in Input, now time.Time) error {
	name := strings.TrimSpace(in.Name)
	if name == "" {
		return errs.New(errs.Invalid, "name is required")
	}
	eng, err := ParseEngine(in.Engine)
	if err != nil {
		return err
	}
	env, err := ParseEnvironment(in.Environment)
	if err != nil {
		return err
	}
	host := strings.TrimSpace(in.Host)
	if host == "" {
		return errs.New(errs.Invalid, "host is required")
	}
	port := in.Port
	if port == 0 {
		port = eng.DefaultPort()
	}
	if port < 1 || port > 65535 {
		return errs.New(errs.Invalid, "port must be between 1 and 65535")
	}
	user := strings.TrimSpace(in.User)
	if user == "" {
		return errs.New(errs.Invalid, "user is required")
	}
	tls := TLSMode(strings.TrimSpace(in.TLS))
	switch tls {
	case "":
		tls = TLSPrefer
	case TLSDisable, TLSPrefer, TLSRequire, TLSVerifyCA, TLSVerifyFull:
	default:
		return errs.Newf(errs.Invalid, "unknown TLS mode %q", in.TLS)
	}
	db := strings.TrimSpace(in.Database)
	if db == "" {
		db = eng.DefaultDatabase()
	}
	readOnly := env == Production
	if in.ReadOnly != nil {
		readOnly = *in.ReadOnly || env == Production
	}
	c.Name, c.Engine, c.Environment, c.Host, c.Port = name, eng, env, host, port
	c.Database, c.User, c.TLS, c.ReadOnly, c.UpdatedAt = db, user, tls, readOnly, now
	return nil
}

// DefaultPort is the engine's conventional port.
func (e Engine) DefaultPort() int {
	switch e {
	case MySQL:
		return 3306
	default:
		return 5432
	}
}

// DefaultDatabase is the database to open when none is given.
func (e Engine) DefaultDatabase() string {
	switch e {
	case MySQL:
		return "mysql"
	default:
		return "postgres"
	}
}

// Label is the environment's display name.
func (e Environment) Label() string {
	switch e {
	case Production:
		return "Production"
	case Staging:
		return "Staging"
	case Development:
		return "Development"
	default:
		return "Local"
	}
}
