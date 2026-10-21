// Package mcp exposes Mesa to agents over the Model Context Protocol. Tools
// are read-only unless writes are allowed; even then every write returns its
// plan first and runs only through the same policy as every other surface.
package mcp

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/felipemalacarne/mesa/internal/app"
	"github.com/felipemalacarne/mesa/internal/app/changes"
	"github.com/felipemalacarne/mesa/internal/app/query"
	"github.com/felipemalacarne/mesa/internal/core/changeset"
	"github.com/felipemalacarne/mesa/internal/core/errs"
	"github.com/felipemalacarne/mesa/internal/core/ident"
	"github.com/felipemalacarne/mesa/internal/engine"
	"github.com/felipemalacarne/mesa/internal/platform"
)

const surface = "mcp"

// Options configure the server.
type Options struct {
	AllowWrites bool
}

// NewServer builds the MCP server with Mesa's tools.
func NewServer(a *app.App, o Options) *mcp.Server {
	s := mcp.NewServer(&mcp.Implementation{Name: "mesa", Version: platform.Version}, nil)
	t := &tools{app: a, allowWrites: o.AllowWrites}

	mcp.AddTool(s, &mcp.Tool{Name: "list_connections", Description: "Connections Mesa knows: name, environment, engine, host, database, read-only state."}, t.listConnections)
	mcp.AddTool(s, &mcp.Tool{Name: "list_databases", Description: "Databases on a connection's server."}, t.listDatabases)
	mcp.AddTool(s, &mcp.Tool{Name: "get_schema", Description: "The schema of a database as DBML (tables, columns, keys, relations)."}, t.getSchema)
	mcp.AddTool(s, &mcp.Tool{Name: "query", Description: "Run SQL and return rows. Read-only connections run in a read-only transaction. Results are capped."}, t.query)
	mcp.AddTool(s, &mcp.Tool{Name: "table_rows", Description: "Page through a table with optional sort."}, t.tableRows)
	mcp.AddTool(s, &mcp.Tool{Name: "explain", Description: "EXPLAIN a query (no ANALYZE)."}, t.explain)
	mcp.AddTool(s, &mcp.Tool{Name: "monitor", Description: "Server overview and current metrics for a database."}, t.monitor)
	mcp.AddTool(s, &mcp.Tool{Name: "sessions", Description: "Active sessions on a database."}, t.sessions)
	mcp.AddTool(s, &mcp.Tool{Name: "plan_sql", Description: "Plan a SQL script as a reviewed change: returns the statements, risks and the policy decision without running anything."}, t.planSQL)
	if o.AllowWrites {
		mcp.AddTool(s, &mcp.Tool{Name: "apply_sql", Description: "Run a SQL script through Mesa's policy and audit. Production and risky changes need `confirmation` = the database name."}, t.applySQL)
	}
	return s
}

// Serve runs the server on stdio until the client disconnects.
func Serve(ctx context.Context, s *mcp.Server) error {
	return s.Run(ctx, &mcp.StdioTransport{})
}

// Handler serves the same tools over streamable HTTP, for mesa-server.
// Stateless: every request stands alone, which suits replicas behind a
// load balancer.
func Handler(a *app.App, o Options) http.Handler {
	s := NewServer(a, o)
	return mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server { return s }, &mcp.StreamableHTTPOptions{Stateless: true})
}

type tools struct {
	app         *app.App
	allowWrites bool
}

type connRef struct {
	Connection string `json:"connection" jsonschema:"connection name, or name@environment when the name is used in several environments"`
	Database   string `json:"database,omitempty" jsonschema:"database to open; the connection's default when empty"`
}

func (t *tools) resolve(ctx context.Context, ref string) (uuid.UUID, error) {
	name, env, _ := strings.Cut(ref, "@")
	items, err := t.app.Connections.List(ctx)
	if err != nil {
		return uuid.Nil, err
	}
	var found []uuid.UUID
	for _, it := range items {
		if it.Name == name && (env == "" || string(it.Environment) == strings.ToLower(env)) {
			found = append(found, it.ID)
		}
	}
	switch len(found) {
	case 0:
		return uuid.Nil, errs.Newf(errs.NotFound, "no connection %q", ref)
	case 1:
		return found[0], nil
	}
	return uuid.Nil, errs.Newf(errs.Invalid, "%q names several connections; use name@environment", ref)
}

func text(s string) *mcp.CallToolResult {
	return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: s}}}
}

func fail(err error) (*mcp.CallToolResult, any, error) {
	return &mcp.CallToolResult{IsError: true, Content: []mcp.Content{&mcp.TextContent{Text: errs.Message(err)}}}, nil, nil
}

// --- tools ----------------------------------------------------------------

type connectionOut struct {
	Name        string `json:"name"`
	Environment string `json:"environment"`
	Engine      string `json:"engine"`
	Host        string `json:"host"`
	Database    string `json:"database"`
	ReadOnly    bool   `json:"readOnly"`
}

func (t *tools) listConnections(ctx context.Context, _ *mcp.CallToolRequest, _ struct{}) (*mcp.CallToolResult, []connectionOut, error) {
	items, err := t.app.Connections.List(ctx)
	if err != nil {
		r, _, e := fail(err)
		return r, nil, e
	}
	out := make([]connectionOut, 0, len(items))
	for _, it := range items {
		out = append(out, connectionOut{Name: it.Name, Environment: string(it.Environment), Engine: string(it.Engine), Host: fmt.Sprintf("%s:%d", it.Host, it.Port), Database: it.Database, ReadOnly: it.ReadOnly && it.UnlockedUntil == nil})
	}
	return nil, out, nil
}

func (t *tools) listDatabases(ctx context.Context, _ *mcp.CallToolRequest, in connRef) (*mcp.CallToolResult, []engine.DatabaseInfo, error) {
	id, err := t.resolve(ctx, in.Connection)
	if err != nil {
		r, _, e := fail(err)
		return r, nil, e
	}
	dbs, err := t.app.Explore.Databases(ctx, id)
	if err != nil {
		r, _, e := fail(err)
		return r, nil, e
	}
	return nil, dbs, nil
}

func (t *tools) getSchema(ctx context.Context, _ *mcp.CallToolRequest, in connRef) (*mcp.CallToolResult, any, error) {
	id, err := t.resolve(ctx, in.Connection)
	if err != nil {
		return fail(err)
	}
	m, err := t.app.Designer.Model(ctx, id, in.Database)
	if err != nil {
		return fail(err)
	}
	return text(m.DBML), nil, nil
}

type queryIn struct {
	connRef
	SQL     string `json:"sql"`
	MaxRows int    `json:"maxRows,omitempty" jsonschema:"default 200, max 2000"`
}

func (t *tools) query(ctx context.Context, _ *mcp.CallToolRequest, in queryIn) (*mcp.CallToolResult, *engine.Result, error) {
	id, err := t.resolve(ctx, in.Connection)
	if err != nil {
		r, _, e := fail(err)
		return r, nil, e
	}
	max := in.MaxRows
	if max <= 0 {
		max = 200
	}
	if max > 2000 {
		max = 2000
	}
	res, err := t.app.Query.Run(ctx, query.RunInput{ConnectionID: id, Database: in.Database, SQL: in.SQL, MaxRows: max, ReadOnly: !t.allowWrites})
	if err != nil {
		r, _, e := fail(err)
		return r, nil, e
	}
	return nil, res, nil
}

type rowsIn struct {
	connRef
	Schema string `json:"schema,omitempty" jsonschema:"default public"`
	Table  string `json:"table"`
	Limit  int    `json:"limit,omitempty"`
	Offset int    `json:"offset,omitempty"`
	SortBy string `json:"sortBy,omitempty"`
	Desc   bool   `json:"desc,omitempty"`
}

func (t *tools) tableRows(ctx context.Context, _ *mcp.CallToolRequest, in rowsIn) (*mcp.CallToolResult, *engine.RowsPage, error) {
	id, err := t.resolve(ctx, in.Connection)
	if err != nil {
		r, _, e := fail(err)
		return r, nil, e
	}
	schemaName := in.Schema
	if schemaName == "" {
		schemaName = "public"
	}
	q, err := ident.ParseQualified(schemaName+"."+in.Table, "public")
	if err != nil {
		r, _, e := fail(err)
		return r, nil, e
	}
	rq := engine.RowsQuery{Table: q, Limit: in.Limit, Offset: in.Offset}
	if in.SortBy != "" {
		col, err := ident.Parse(in.SortBy)
		if err != nil {
			r, _, e := fail(err)
			return r, nil, e
		}
		rq.Sort = []engine.Sort{{Column: col, Desc: in.Desc}}
	}
	page, err := t.app.Explore.Rows(ctx, id, in.Database, rq)
	if err != nil {
		r, _, e := fail(err)
		return r, nil, e
	}
	return nil, page, nil
}

type explainIn struct {
	connRef
	SQL string `json:"sql"`
}

func (t *tools) explain(ctx context.Context, _ *mcp.CallToolRequest, in explainIn) (*mcp.CallToolResult, any, error) {
	id, err := t.resolve(ctx, in.Connection)
	if err != nil {
		return fail(err)
	}
	lines, err := t.app.Query.Explain(ctx, id, in.Database, in.SQL, false)
	if err != nil {
		return fail(err)
	}
	return text(strings.Join(lines, "\n")), nil, nil
}

type monitorOut struct {
	Overview *engine.Overview `json:"overview"`
	Metrics  *engine.Metrics  `json:"metrics"`
	TPS      float64          `json:"tps"`
	Panel    *engine.Panel    `json:"panel,omitempty"`
}

func (t *tools) monitor(ctx context.Context, _ *mcp.CallToolRequest, in connRef) (*mcp.CallToolResult, *monitorOut, error) {
	id, err := t.resolve(ctx, in.Connection)
	if err != nil {
		r, _, e := fail(err)
		return r, nil, e
	}
	ov, err := t.app.Monitor.Overview(ctx, id, in.Database)
	if err != nil {
		r, _, e := fail(err)
		return r, nil, e
	}
	snap, err := t.app.Monitor.Snapshot(ctx, id, in.Database)
	if err != nil {
		r, _, e := fail(err)
		return r, nil, e
	}
	out := &monitorOut{Overview: ov, Metrics: snap.Metrics, TPS: snap.TPS}
	if p, err := t.app.Monitor.Panel(ctx, id, in.Database); err == nil {
		out.Panel = p
	}
	return nil, out, nil
}

func (t *tools) sessions(ctx context.Context, _ *mcp.CallToolRequest, in connRef) (*mcp.CallToolResult, []engine.ActiveSession, error) {
	id, err := t.resolve(ctx, in.Connection)
	if err != nil {
		r, _, e := fail(err)
		return r, nil, e
	}
	list, err := t.app.Monitor.Sessions(ctx, id, in.Database)
	if err != nil {
		r, _, e := fail(err)
		return r, nil, e
	}
	return nil, list, nil
}

type sqlIn struct {
	connRef
	SQL          string `json:"sql" jsonschema:"one or more statements separated by semicolons"`
	Confirmation string `json:"confirmation,omitempty" jsonschema:"the database name, when the policy asks for it"`
}

type planOut struct {
	Statements  []string         `json:"statements"`
	Risks       []changeset.Risk `json:"risks,omitempty"`
	Decision    string           `json:"decision"`
	Reason      string           `json:"reason,omitempty"`
	ConfirmWith string           `json:"confirmWith,omitempty"`
	Applied     bool             `json:"applied"`
	DurationMs  int64            `json:"durationMs,omitempty"`
}

func (t *tools) plan(ctx context.Context, in sqlIn, dryRun bool) (*planOut, error) {
	id, err := t.resolve(ctx, in.Connection)
	if err != nil {
		return nil, err
	}
	p := &changeset.Plan{Database: ident.Name(in.Database), Statements: split(in.SQL)}
	if len(p.Statements) == 0 {
		return nil, errs.New(errs.Invalid, "no statements")
	}
	res, err := t.app.Changes.Apply(ctx, changes.ApplyInput{
		ConnectionID: id, Database: in.Database, Plan: p, Confirmation: in.Confirmation,
		Transaction: true, DryRun: dryRun, Actor: "agent", Surface: surface,
	})
	if res == nil && err != nil {
		return nil, err
	}
	out := &planOut{Decision: string(res.Decision.Outcome), Reason: res.Decision.Reason, ConfirmWith: res.Decision.ConfirmWith, Risks: res.Plan.Risks, Applied: res.Applied, DurationMs: res.Duration.Milliseconds()}
	for _, s := range res.Plan.Statements {
		out.Statements = append(out.Statements, s.Display())
	}
	return out, err
}

func (t *tools) planSQL(ctx context.Context, _ *mcp.CallToolRequest, in sqlIn) (*mcp.CallToolResult, *planOut, error) {
	out, err := t.plan(ctx, in, true)
	if err != nil && out == nil {
		r, _, e := fail(err)
		return r, nil, e
	}
	return nil, out, nil
}

func (t *tools) applySQL(ctx context.Context, _ *mcp.CallToolRequest, in sqlIn) (*mcp.CallToolResult, *planOut, error) {
	out, err := t.plan(ctx, in, false)
	if err != nil {
		if out == nil {
			r, _, e := fail(err)
			return r, nil, e
		}
		return &mcp.CallToolResult{IsError: true, Content: []mcp.Content{&mcp.TextContent{Text: errs.Message(err)}}}, out, nil
	}
	return nil, out, nil
}

func split(src string) []changeset.Statement {
	var out []changeset.Statement
	for _, part := range strings.Split(src, ";") {
		s := strings.TrimSpace(part)
		if s == "" {
			continue
		}
		first, _, _ := strings.Cut(s, "\n")
		out = append(out, changeset.Statement{Kind: changeset.DDL, SQL: s, Describe: first})
	}
	return out
}
