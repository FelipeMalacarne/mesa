// Package store holds Mesa's own data behind ports: connections, encrypted
// secrets, the audit log, diagram layouts, query history, saved queries, and
// in server mode the people who signed in and their tokens.
// Implementations: sqlite (desktop, CLI), postgres (server mode).
package store

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/felipemalacarne/mesa/internal/core/account"
	"github.com/felipemalacarne/mesa/internal/core/connection"
)

// Store groups every port and owns the underlying database handle.
type Store interface {
	Connections() Connections
	Secrets() Secrets
	Audit() Audit
	Layouts() Layouts
	History() History
	Queries() SavedQueries
	Metrics() Metrics
	Users() Users
	Tokens() Tokens
	Close() error
}

// Connections persists connection entities.
type Connections interface {
	List(ctx context.Context) ([]connection.Connection, error)
	Get(ctx context.Context, id uuid.UUID) (*connection.Connection, error) // errs.NotFound when missing
	Put(ctx context.Context, c connection.Connection) error                // insert or update by ID
	Delete(ctx context.Context, id uuid.UUID) error
	// TouchOpened records that the connection was opened at the given time.
	TouchOpened(ctx context.Context, id uuid.UUID, at time.Time) error
	// LastOpened returns the last opened time per connection ID (zero when never).
	LastOpened(ctx context.Context) (map[uuid.UUID]time.Time, error)
}

// Secrets persists opaque secret blobs per connection. The vault encrypts
// before Set and decrypts after Get; the store never sees plaintext.
type Secrets interface {
	Get(ctx context.Context, id uuid.UUID) ([]byte, error) // errs.NotFound when missing
	Set(ctx context.Context, id uuid.UUID, blob []byte) error
	Delete(ctx context.Context, id uuid.UUID) error
}

// AuditStatus is the state of an audited write.
type AuditStatus string

const (
	AuditStarted AuditStatus = "started"
	AuditApplied AuditStatus = "applied"
	AuditFailed  AuditStatus = "failed"
)

// AuditEntry is one write Mesa ran (or tried to).
type AuditEntry struct {
	ID           uuid.UUID
	ConnectionID uuid.UUID
	Database     string
	Environment  connection.Environment
	Actor        string // who, as the surface knows them
	Surface      string // http, cli, mcp
	Summary      string
	SQL          string
	DownSQL      string
	Status       AuditStatus
	Error        string
	StartedAt    time.Time
	FinishedAt   *time.Time
}

// Audit records every write.
type Audit interface {
	Begin(ctx context.Context, e AuditEntry) error
	Finish(ctx context.Context, id uuid.UUID, status AuditStatus, errMsg string, at time.Time) error
	List(ctx context.Context, connectionID *uuid.UUID, limit int) ([]AuditEntry, error)
}

// Position is a diagram node position.
type Position struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

// Layouts persists schema-designer positions per (connection, database).
type Layouts interface {
	Get(ctx context.Context, connectionID uuid.UUID, database string) (map[string]Position, error)
	Put(ctx context.Context, connectionID uuid.UUID, database string, positions map[string]Position) error
}

// QueryRun is one executed query.
type QueryRun struct {
	ID           uuid.UUID
	ConnectionID uuid.UUID
	Database     string
	SQL          string
	RowCount     int64
	Duration     time.Duration
	Error        string
	RanAt        time.Time
}

// History persists query runs.
type History interface {
	Add(ctx context.Context, r QueryRun) error
	List(ctx context.Context, connectionID uuid.UUID, limit int) ([]QueryRun, error)
}

// SavedQuery is a named query a person keeps.
type SavedQuery struct {
	ID           uuid.UUID
	ConnectionID *uuid.UUID // nil = any connection
	Name         string
	SQL          string
	UpdatedAt    time.Time
}

// SavedQueries persists saved queries.
type SavedQueries interface {
	List(ctx context.Context) ([]SavedQuery, error)
	Put(ctx context.Context, q SavedQuery) error
	Delete(ctx context.Context, id uuid.UUID) error
}

// MetricPoint is one Monitor sample kept as history.
type MetricPoint struct {
	At        time.Time
	TPS       float64
	Active    int
	Idle      int
	IdleTx    int
	LatencyMs float64
	SizeBytes int64
}

// Metrics keeps Monitor samples per connection and database so the charts can
// show hours and days, not only what was sampled while the page was open.
type Metrics interface {
	Add(ctx context.Context, connectionID uuid.UUID, database string, p MetricPoint) error
	// Range returns the samples in [from, to) averaged into at most buckets
	// points, oldest first.
	Range(ctx context.Context, connectionID uuid.UUID, database string, from, to time.Time, buckets int) ([]MetricPoint, error)
	// Prune drops samples older than before.
	Prune(ctx context.Context, before time.Time) error
}

// Users records the people who signed in, as the identity provider last
// described them.
type Users interface {
	List(ctx context.Context) ([]account.User, error)
	Get(ctx context.Context, id uuid.UUID) (*account.User, error)            // errs.NotFound when missing
	GetBySubject(ctx context.Context, subject string) (*account.User, error) // errs.NotFound when missing
	// Put inserts or updates by ID.
	Put(ctx context.Context, u account.User) error
	TouchSeen(ctx context.Context, id uuid.UUID, at time.Time) error
}

// TokenKind tells a browser session from a token a person made for an agent
// or a script.
type TokenKind string

const (
	SessionToken TokenKind = "session"
	APIToken     TokenKind = "api"
)

// Token is a credential that stands for a user. Only the SHA-256 of its
// secret is kept: whoever holds the store cannot sign in with it.
type Token struct {
	ID         uuid.UUID
	UserID     uuid.UUID
	Kind       TokenKind
	Name       string
	Hash       []byte
	CreatedAt  time.Time
	ExpiresAt  *time.Time
	LastUsedAt *time.Time
}

// Tokens persists sessions and API tokens.
type Tokens interface {
	Create(ctx context.Context, t Token) error
	ByHash(ctx context.Context, hash []byte) (*Token, error) // errs.NotFound when missing
	// List returns a user's tokens of one kind, newest first.
	List(ctx context.Context, userID uuid.UUID, kind TokenKind) ([]Token, error)
	Touch(ctx context.Context, id uuid.UUID, at time.Time) error
	Delete(ctx context.Context, id uuid.UUID) error
	// DeleteForUser removes a user's tokens of one kind, or of every kind
	// when kind is empty.
	DeleteForUser(ctx context.Context, userID uuid.UUID, kind TokenKind) error
	// Prune removes tokens that expired before the given time.
	Prune(ctx context.Context, before time.Time) error
}
