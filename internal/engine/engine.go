// Package engine defines the ports a database driver implements. Use cases
// talk to these interfaces only; engine-specific behaviour stays behind them
// and is declared through Capabilities.
package engine

import (
	"context"
	"time"

	"github.com/felipemalacarne/mesa/internal/core/changeset"
	"github.com/felipemalacarne/mesa/internal/core/connection"
	"github.com/felipemalacarne/mesa/internal/core/ident"
	"github.com/felipemalacarne/mesa/internal/core/schema"
	"github.com/felipemalacarne/mesa/internal/core/value"
)

// Target is everything needed to open a session. It is built by the session
// manager from a Connection plus its secret and never stored.
type Target struct {
	Engine         connection.Engine
	Host           string
	Port           int
	Database       string
	User           string
	Secret         string
	TLS            connection.TLSMode
	AppName        string
	ConnectTimeout time.Duration
	// MaxConns bounds the pool behind the session.
	MaxConns int
}

// Capabilities declares what an engine supports so the UI shows only that.
type Capabilities struct {
	Schemas          bool `json:"schemas"` // schemas inside a database
	Enums            bool `json:"enums"`
	Views            bool `json:"views"`
	Functions        bool `json:"functions"`
	Sequences        bool `json:"sequences"`
	Explain          bool `json:"explain"`
	KillSession      bool `json:"killSession"`
	EnginePanel      bool `json:"enginePanel"` // engine-specific monitor panel
	DDL              bool `json:"ddl"`         // schema changes through Dialect
	TransactionalDDL bool `json:"transactionalDdl"`
	Databases        bool `json:"databases"` // create/drop databases
	Roles            bool `json:"roles"`
	// Types are the column types the designer offers, most used first. Any
	// other spelling the engine accepts can still be typed.
	Types []ColumnType `json:"types"`
}

// ColumnType is one column type as the designer offers it.
type ColumnType struct {
	Name  string `json:"name"`  // "varchar", "bigint unsigned"
	Group string `json:"group"` // "Numeric", "Text", "Date & time"…
	// Args names what goes in parentheses: "length", "precision" (p,s),
	// "values" (MySQL enum/set), or "" when the type takes none.
	Args string `json:"args,omitempty"`
	// Spelling is what is inserted when the type is picked: "varchar(255)".
	Spelling string `json:"spelling"`
	// Integer types can be auto-incremented (identity / AUTO_INCREMENT).
	Integer bool `json:"integer,omitempty"`
}

// Engine creates sessions for one database engine.
type Engine interface {
	Name() connection.Engine
	Capabilities() Capabilities
	Open(ctx context.Context, t Target) (Session, error)
}

// Session is an open, pooled connection to one database of one server.
type Session interface {
	Catalog
	Exec
	Monitor
	Admin
	Dialect
	Ping(ctx context.Context) error
	Close()
}

// --- Catalog --------------------------------------------------------------

// DatabaseInfo is one database on the server.
type DatabaseInfo struct {
	Name      ident.Name `json:"name"`
	Owner     string     `json:"owner,omitempty"`
	Encoding  string     `json:"encoding,omitempty"`
	SizeBytes int64      `json:"sizeBytes,omitempty"`
}

// Catalog reads structure.
type Catalog interface {
	// Databases lists the databases this role can see.
	Databases(ctx context.Context) ([]DatabaseInfo, error)
	// Database returns the full model of the session's database, with stats.
	Database(ctx context.Context) (*schema.Database, error)
	// Table returns one table with fresh stats.
	Table(ctx context.Context, q ident.Qualified) (*schema.Table, error)
}

// --- Exec -----------------------------------------------------------------

// QueryOptions bounds an ad-hoc query.
type QueryOptions struct {
	MaxRows  int           // 0 = engine default
	ReadOnly bool          // run in a read-only transaction
	Timeout  time.Duration // 0 = none
}

// Result is the outcome of a statement.
type Result struct {
	Columns      []value.Column `json:"columns"`
	Rows         []value.Row    `json:"rows"`
	RowsAffected int64          `json:"rowsAffected"`
	Truncated    bool           `json:"truncated"` // MaxRows reached
	Duration     time.Duration  `json:"duration"`
	Command      string         `json:"command,omitempty"` // SELECT, UPDATE…
}

// Filter is one WHERE condition on a table column.
type Filter struct {
	Column ident.Name  `json:"column"`
	Op     FilterOp    `json:"op"`
	Value  value.Value `json:"value"`
}

// FilterOp is a comparison the grid filter bar offers.
type FilterOp string

const (
	OpEq      FilterOp = "="
	OpNeq     FilterOp = "!="
	OpLt      FilterOp = "<"
	OpLte     FilterOp = "<="
	OpGt      FilterOp = ">"
	OpGte     FilterOp = ">="
	OpLike    FilterOp = "like"
	OpILike   FilterOp = "ilike"
	OpIsNull  FilterOp = "is null"
	OpNotNull FilterOp = "is not null"
)

// Sort orders rows by one column.
type Sort struct {
	Column ident.Name `json:"column"`
	Desc   bool       `json:"desc"`
}

// RowsQuery pages through a table.
type RowsQuery struct {
	Table   ident.Qualified
	Filters []Filter
	Sort    []Sort
	Limit   int
	Offset  int
}

// RowsPage is one page of a table plus the count behind the filter.
type RowsPage struct {
	Result *Result
	Total  int64
}

// Tx runs statements inside one transaction.
type Tx interface {
	Exec(ctx context.Context, sql string, args []value.Value) (rowsAffected int64, err error)
	Query(ctx context.Context, sql string, args []value.Value) (*Result, error)
}

// Exec runs SQL.
type Exec interface {
	Query(ctx context.Context, sql string, args []value.Value, opts QueryOptions) (*Result, error)
	Explain(ctx context.Context, sql string, args []value.Value, analyze bool) ([]string, error)
	TableRows(ctx context.Context, q RowsQuery) (*RowsPage, error)
	Tx(ctx context.Context, fn func(Tx) error) error
}

// --- Monitor --------------------------------------------------------------

// Overview is the header of the Monitor screen and the connection list.
type Overview struct {
	Version     string    `json:"version"`
	VersionFull string    `json:"versionFull,omitempty"`
	StartedAt   time.Time `json:"startedAt"`
	Database    string    `json:"database"`
	Role        string    `json:"role"`
	ReadOnly    bool      `json:"readOnly"` // server-side (hot standby)
}

// Metrics are the general counters every engine fills in. They are sampled
// by the app layer; deltas between samples give rates.
type Metrics struct {
	SampledAt         time.Time `json:"sampledAt"`
	Commits           int64     `json:"commits"`
	Rollbacks         int64     `json:"rollbacks"`
	ConnectionsActive int       `json:"connectionsActive"`
	ConnectionsIdle   int       `json:"connectionsIdle"`
	ConnectionsIdleTx int       `json:"connectionsIdleTx"`
	ConnectionsMax    int       `json:"connectionsMax"`
	DatabaseSizeBytes int64     `json:"databaseSizeBytes"`
	CacheHitRatio     float64   `json:"cacheHitRatio"` // 0..1
	LocksWaiting      int       `json:"locksWaiting"`
	LongestTxSeconds  float64   `json:"longestTxSeconds"`
}

// ActiveSession is one backend.
type ActiveSession struct {
	PID         int           `json:"pid"`
	User        string        `json:"user"`
	Database    string        `json:"database"`
	Application string        `json:"application,omitempty"`
	ClientAddr  string        `json:"clientAddr,omitempty"`
	State       string        `json:"state"`
	WaitEvent   string        `json:"waitEvent,omitempty"`
	Query       string        `json:"query,omitempty"`
	StartedAt   time.Time     `json:"startedAt"`
	QueryStart  *time.Time    `json:"queryStart,omitempty"`
	Duration    time.Duration `json:"duration"` // of the current query or transaction
	Blocking    bool          `json:"blocking"`
}

// Panel is an engine-specific set of metrics rendered generically.
type Panel struct {
	Title  string       `json:"title"`
	Tiles  []Tile       `json:"tiles,omitempty"`
	Meters []Meter      `json:"meters,omitempty"`
	Tables []PanelTable `json:"tables,omitempty"`
}

// Status grades a tile or meter.
type Status string

const (
	StatusOK       Status = "ok"
	StatusWarning  Status = "warning"
	StatusCritical Status = "critical"
	StatusNone     Status = ""
)

// Tile is one stat.
type Tile struct {
	Label  string `json:"label"`
	Value  string `json:"value"`
	Unit   string `json:"unit,omitempty"`
	Status Status `json:"status,omitempty"`
	Note   string `json:"note,omitempty"`
}

// Meter is a value against a ceiling.
type Meter struct {
	Label  string  `json:"label"`
	Value  float64 `json:"value"`
	Max    float64 `json:"max"`
	Text   string  `json:"text"` // "1.31B / 2.1B"
	Status Status  `json:"status,omitempty"`
	Note   string  `json:"note,omitempty"`
}

// PanelTable is a small table of facts.
type PanelTable struct {
	Title   string     `json:"title"`
	Source  string     `json:"source,omitempty"` // pg_stat_statements…
	Columns []string   `json:"columns"`
	Numeric []bool     `json:"numeric,omitempty"` // per column, right-align
	Rows    [][]string `json:"rows"`
	Empty   string     `json:"empty,omitempty"` // shown when Rows is empty
}

// Monitor reads runtime state.
type Monitor interface {
	Overview(ctx context.Context) (*Overview, error)
	Metrics(ctx context.Context) (*Metrics, error)
	Sessions(ctx context.Context) ([]ActiveSession, error)
	Kill(ctx context.Context, pid int, terminate bool) error
	EnginePanel(ctx context.Context) (*Panel, error)
}

// --- Admin ----------------------------------------------------------------

// Role is a login or group role.
type Role struct {
	Name       ident.Name `json:"name"`
	Superuser  bool       `json:"superuser"`
	CanLogin   bool       `json:"canLogin"`
	CreateDB   bool       `json:"createDb"`
	CreateRole bool       `json:"createRole"`
	Inherit    bool       `json:"inherit"`
	ConnLimit  int        `json:"connLimit"` // -1 = unlimited
	ValidUntil *time.Time `json:"validUntil,omitempty"`
	MemberOf   []string   `json:"memberOf,omitempty"`
}

// RoleInput creates or alters a role.
type RoleInput struct {
	Name       ident.Name
	Password   string // empty = no change
	CanLogin   bool
	CreateDB   bool
	CreateRole bool
	Superuser  bool
	ConnLimit  int
	MemberOf   []ident.Name
}

// Admin manages server-level objects. Each method builds the statements it
// runs and returns them so the app layer can show them in review first.
type Admin interface {
	Roles(ctx context.Context) ([]Role, error)
	CreateDatabasePlan(name, owner ident.Name) *changeset.Plan
	CreateRolePlan(in RoleInput) *changeset.Plan
	DropRolePlan(name ident.Name) *changeset.Plan
}

// --- Dialect --------------------------------------------------------------

// Dialect turns a change set into SQL for this engine. Current may be nil
// when the database is new; with it the dialect can compute risks (row
// counts, locks) and validate names.
type Dialect interface {
	Compile(ctx context.Context, current *schema.Database, cs *changeset.ChangeSet) (*changeset.Plan, error)
	// SameType reports whether two type spellings name the same type
	// ("varchar(20)" and "character varying(20)").
	SameType(a, b string) bool
	// SameDefault reports whether two default expressions are equivalent
	// ("'new'" and "'new'::order_status").
	SameDefault(a, b string) bool
}
