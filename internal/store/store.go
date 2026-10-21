// Package store holds Mesa's own data behind ports: connections, encrypted
// secrets, the audit log, diagram layouts, query history and saved queries.
// Implementations: sqlite (desktop, CLI), postgres (server mode).
package store

import (
	"context"
	"time"

	"github.com/google/uuid"

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
