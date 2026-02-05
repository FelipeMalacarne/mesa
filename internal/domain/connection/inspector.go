package connection

import "context"

type Database struct {
	Name   string
	Tables []Table
}

type Table struct {
	Name     string
	Type     string // "TABLE" ou "VIEW"
	Size     int64  // em bytes
	RowCount int64
	Columns  []Column
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
}
