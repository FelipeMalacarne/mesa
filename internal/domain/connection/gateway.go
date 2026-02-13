package connection

import "context"

// Gateway agrega operações de leitura estrutural, runtime e ações administrativas.
type Gateway interface {
	// Estrutura e conectividade
	GetDatabases(ctx context.Context, conn Connection, password string) ([]Database, error)
	GetTables(ctx context.Context, conn Connection, password string, dbName string) ([]Table, error)
	GetColumns(ctx context.Context, conn Connection, password, dbName, tableName string) ([]Column, error)
	Ping(ctx context.Context, conn Connection, password string) error

	// Runtime e operações administrativas
	GetServerHealth(ctx context.Context, conn Connection, password string) (*ServerHealth, error)
	ListSessions(ctx context.Context, conn Connection, password string) ([]Session, error)
	KillSession(ctx context.Context, conn Connection, password string, pid int) error
	ListUsers(ctx context.Context, conn Connection, password string) ([]DBUser, error)
	CreateUser(ctx context.Context, conn Connection, password string, user DBUser, secret string) error
	DropUser(ctx context.Context, conn Connection, password string, username string) error
	CreateDatabase(ctx context.Context, conn Connection, password string, dbName string, owner string) error
}

// GatewayFactory devolve a implementação adequada para determinado driver.
type GatewayFactory interface {
	ForDriver(driver Driver) (Gateway, error)
}
