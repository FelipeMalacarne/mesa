package connection

import "context"

type Database struct {
	Name string
}

type Table struct {
	Name      string
	Type      string // "TABLE" ou "VIEW"
	SizeBytes int64  // tamanho total em bytes (0 para views)
	RowCount  int64  // estimativa do Postgres (0 para views)
}

type Column struct {
	Name         string
	Type         string
	IsNullable   bool
	IsPrimary    bool
	DefaultValue *string
}

// Inspector é a nossa "Port" (Interface)
type Inspector interface {
	GetDatabases(ctx context.Context, conn Connection, password string) ([]Database, error)
	GetTables(ctx context.Context, conn Connection, password string, dbName string) ([]Table, error)
	GetColumns(ctx context.Context, conn Connection, password, dbName, tableName string) ([]Column, error)
}
