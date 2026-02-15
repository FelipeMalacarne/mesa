package connection

import "time"

type Database struct {
	Name       string
	Owner      string
	Encoding   string
	Size       int64
	TableCount int
}

type Table struct {
	Name     string
	Type     string // "TABLE", "VIEW", "MATERIALIZED VIEW"
	Size     int64
	RowCount int64
}

type Column struct {
	Name         string
	DataType     string
	IsNullable   bool
	IsPrimary    bool
	DefaultValue *string
}

type ServerHealth struct {
	Version        string
	Uptime         time.Duration
	ActiveSessions int
	MaxConnections int
	Status         string
}

type DBUser struct {
	Name        Identifier
	IsSuperUser bool
	CanLogin    bool
	ConnLimit   int
}

type Session struct {
	PID       int
	User      string
	Database  string
	State     string
	Query     string
	Duration  time.Duration
	StartedAt time.Time
}

// type TableColumnDefinition struct {
// 	Name         string
// 	Type         ColumnType
// 	Length       *int
// 	Precision    *int
// 	Scale        *int
// 	Nullable     bool
// 	PrimaryKey   bool
// 	DefaultValue *string
// }
//
// type TableIndexDefinition struct {
// 	Name    string
// 	Columns []string
// 	Method  string
// 	Unique  bool
// }
//
// type TableDefinition struct {
// 	Name    string
// 	Columns []TableColumnDefinition
// }
