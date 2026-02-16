package connection

import "context"

// Inspector reads structural metadata (Databases, Tables, Columns).
type Inspector interface {
	GetDatabases(ctx context.Context, conn Connection, password string) ([]Database, error)
	GetTables(ctx context.Context, conn Connection, password string, dbName Identifier) ([]Table, error)
	GetColumns(ctx context.Context, conn Connection, password string, dbName, tableName Identifier) ([]Column, error)
}

// Monitor checks runtime health and active sessions.
type Monitor interface {
	Ping(ctx context.Context, conn Connection, password string) error
	GetServerHealth(ctx context.Context, conn Connection, password string) (*ServerHealth, error)
	ListSessions(ctx context.Context, conn Connection, password string) ([]Session, error)
}

// Administrator handles user management and database creation.
type Administrator interface {
	KillSession(ctx context.Context, conn Connection, password string, pid int) error
	ListUsers(ctx context.Context, conn Connection, password string) ([]DBUser, error)
	CreateUser(ctx context.Context, conn Connection, password string, user DBUser, secret string) error
	DropUser(ctx context.Context, conn Connection, password string, username Identifier) error
	CreateDatabase(ctx context.Context, conn Connection, password string, dbName, owner Identifier) error
}

// Gateway aggregates all operations (kept for backward compatibility during refactor).
type Gateway interface {
	Inspector
	Monitor
	Administrator
	SchemaManager
}

// GatewayFactory devolve a implementação adequada para determinado driver.
type GatewayFactory interface {
	ForDriver(driver Driver) (Gateway, error)
}
