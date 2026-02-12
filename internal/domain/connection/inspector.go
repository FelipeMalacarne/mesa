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
	Name        string
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
