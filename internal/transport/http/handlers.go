package http

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/felipemalacarne/mesa/internal/app"
	"github.com/felipemalacarne/mesa/internal/app/changes"
	"github.com/felipemalacarne/mesa/internal/app/connections"
	"github.com/felipemalacarne/mesa/internal/app/designer"
	"github.com/felipemalacarne/mesa/internal/app/monitor"
	"github.com/felipemalacarne/mesa/internal/app/query"
	"github.com/felipemalacarne/mesa/internal/app/roles"
	"github.com/felipemalacarne/mesa/internal/core/changeset"
	"github.com/felipemalacarne/mesa/internal/core/connection"
	"github.com/felipemalacarne/mesa/internal/core/errs"
	"github.com/felipemalacarne/mesa/internal/core/ident"
	"github.com/felipemalacarne/mesa/internal/core/schema"
	"github.com/felipemalacarne/mesa/internal/dbml"
	"github.com/felipemalacarne/mesa/internal/engine"
	"github.com/felipemalacarne/mesa/internal/store"
	"github.com/felipemalacarne/mesa/internal/transport/http/gen"
)

const surface = "http"

type handlers struct {
	app *app.App
}

var _ gen.StrictServerInterface = (*handlers)(nil)

// actor names the person behind a request; without authentication it is the
// surface itself. Authentication, when added, sets it from the session.
func actor(ctx context.Context) string {
	if a, ok := ctx.Value(actorKey{}).(string); ok && a != "" {
		return a
	}
	return "web"
}

type actorKey struct{}

// --- conversions ----------------------------------------------------------

func deref[T any](p *T, def T) T {
	if p == nil {
		return def
	}
	return *p
}

func ptr[T any](v T) *T { return &v }

func toConnection(it connections.Item) gen.Connection {
	c := gen.Connection{
		Id:          it.ID,
		Name:        it.Name,
		Engine:      gen.Engine(it.Engine),
		Environment: gen.Environment(it.Environment),
		Host:        it.Host,
		Port:        it.Port,
		Database:    it.Database,
		User:        it.User,
		Tls:         gen.TLSMode(it.TLS),
		ReadOnly:    it.ReadOnly,
		CreatedAt:   it.CreatedAt,
		UpdatedAt:   it.UpdatedAt,
	}
	c.LastOpenedAt = it.LastOpenedAt
	c.UnlockedUntil = it.UnlockedUntil
	if it.DefinedIn != "" {
		c.DefinedIn = ptr(it.DefinedIn)
	}
	return c
}

func fromConnectionInput(in gen.ConnectionInput) connections.Input {
	out := connections.Input{Input: connection.Input{
		Name:     in.Name,
		Engine:   string(in.Engine),
		Host:     in.Host,
		Port:     deref(in.Port, 0),
		Database: deref(in.Database, ""),
		User:     in.User,
		ReadOnly: in.ReadOnly,
	}}
	if in.Environment != nil {
		out.Environment = string(*in.Environment)
	}
	if in.Tls != nil {
		out.TLS = string(*in.Tls)
	}
	out.Secret = deref(in.Secret, "")
	return out
}

func toTestResult(t *connections.TestResult) gen.TestResult {
	return gen.TestResult{LatencyMs: float64(t.Latency) / float64(time.Millisecond), Overview: *t.Overview}
}

func toAudit(e store.AuditEntry) gen.AuditEntry {
	out := gen.AuditEntry{
		Id:           e.ID,
		ConnectionId: e.ConnectionID,
		Database:     e.Database,
		Environment:  gen.Environment(e.Environment),
		Surface:      e.Surface,
		Summary:      e.Summary,
		Sql:          e.SQL,
		Status:       gen.AuditEntryStatus(e.Status),
		StartedAt:    e.StartedAt,
		FinishedAt:   e.FinishedAt,
	}
	if e.Actor != "" {
		out.Actor = ptr(e.Actor)
	}
	if e.DownSQL != "" {
		out.DownSql = ptr(e.DownSQL)
	}
	if e.Error != "" {
		out.Error = ptr(e.Error)
	}
	return out
}

func toApplyResult(r *changes.ApplyResult) gen.ApplyResult {
	out := gen.ApplyResult{
		Plan:     *r.Plan,
		Decision: r.Decision,
		Applied:  r.Applied,
		UpSql:    r.Plan.SQL(),
		DownSql:  r.Plan.DownSQL(),
	}
	if r.AuditID != uuid.Nil {
		out.AuditId = ptr(r.AuditID)
	}
	if r.Duration > 0 {
		out.DurationMs = ptr(float64(r.Duration) / float64(time.Millisecond))
	}
	return out
}

func toPlanResult(r *changes.PlanResult) gen.PlanResult {
	return gen.PlanResult{Plan: *r.Plan, Decision: r.Decision, UpSql: r.Plan.SQL(), DownSql: r.Plan.DownSQL()}
}

func toQueryRun(r store.QueryRun) gen.QueryRun {
	out := gen.QueryRun{
		Id:           r.ID,
		ConnectionId: r.ConnectionID,
		Database:     r.Database,
		Sql:          r.SQL,
		RowCount:     r.RowCount,
		DurationMs:   float64(r.Duration) / float64(time.Millisecond),
		RanAt:        r.RanAt,
	}
	if r.Error != "" {
		out.Error = ptr(r.Error)
	}
	return out
}

func toSavedQuery(q store.SavedQuery) gen.SavedQuery {
	out := gen.SavedQuery{Id: ptr(q.ID), Name: q.Name, Sql: q.SQL, UpdatedAt: ptr(q.UpdatedAt)}
	if q.ConnectionID != nil {
		out.ConnectionId = ptr(*q.ConnectionID)
	}
	return out
}

func toSnapshot(s *monitor.Snapshot) gen.Snapshot {
	out := gen.Snapshot{Tps: s.TPS, LatencyMs: s.LatencyMs, Series: make([]gen.Point, 0, len(s.Series))}
	if s.Metrics != nil {
		out.Metrics = *s.Metrics
	}
	if s.SizeDelta7d != 0 {
		out.SizeDelta7d = ptr(s.SizeDelta7d)
	}
	for _, p := range s.Series {
		out.Series = append(out.Series, gen.Point{At: p.At, Tps: p.TPS, Active: p.Active, Idle: p.Idle, IdleTx: p.IdleTx, LatencyMs: p.LatencyMs})
	}
	return out
}

func toProblems(ps []dbml.Problem) []gen.Problem {
	out := make([]gen.Problem, 0, len(ps))
	for _, p := range ps {
		gp := gen.Problem{Level: gen.ProblemLevel(p.Level), Message: p.Message}
		if p.Line > 0 {
			gp.Line = ptr(p.Line)
		}
		out = append(out, gp)
	}
	return out
}

func toLayout(m map[string]store.Position) gen.Layout {
	out := gen.Layout{Positions: map[string]struct {
		X float64 `json:"x"`
		Y float64 `json:"y"`
	}{}}
	for k, p := range m {
		out.Positions[k] = struct {
			X float64 `json:"x"`
			Y float64 `json:"y"`
		}{X: p.X, Y: p.Y}
	}
	return out
}

func fromLayout(l gen.Layout) map[string]store.Position {
	out := make(map[string]store.Position, len(l.Positions))
	for k, p := range l.Positions {
		out[k] = store.Position{X: p.X, Y: p.Y}
	}
	return out
}

func toDiff(d *designer.Diff) gen.DesignerDiff {
	return gen.DesignerDiff{Changes: orEmpty(d.Changes), Plan: *d.Plan, Decision: d.Decision, UpSql: d.Plan.SQL(), DownSql: d.Plan.DownSQL()}
}

func qualified(q gen.Qualified) (ident.Qualified, error) {
	s, err := ident.Parse(q.Schema)
	if err != nil {
		return ident.Qualified{}, err
	}
	n, err := ident.Parse(q.Name)
	if err != nil {
		return ident.Qualified{}, err
	}
	return ident.Q(s, n), nil
}

// --- System ---------------------------------------------------------------

func (h *handlers) GetSystemInfo(_ context.Context, _ gen.GetSystemInfoRequestObject) (gen.GetSystemInfoResponseObject, error) {
	info := gen.SystemInfo{Version: h.app.Version, Mode: gen.SystemInfoMode(h.app.Mode)}
	for _, e := range h.app.Engines {
		info.Engines = append(info.Engines, struct {
			Capabilities engine.Capabilities `json:"capabilities"`
			Name         string              `json:"name"`
		}{Capabilities: e.Capabilities(), Name: string(e.Name())})
	}
	return gen.GetSystemInfo200JSONResponse(info), nil
}

// --- Connections ----------------------------------------------------------

func (h *handlers) ListConnections(ctx context.Context, _ gen.ListConnectionsRequestObject) (gen.ListConnectionsResponseObject, error) {
	items, err := h.app.Connections.List(ctx)
	if err != nil {
		return nil, err
	}
	out := make(gen.ListConnections200JSONResponse, 0, len(items))
	for _, it := range items {
		out = append(out, toConnection(it))
	}
	return out, nil
}

func (h *handlers) CreateConnection(ctx context.Context, r gen.CreateConnectionRequestObject) (gen.CreateConnectionResponseObject, error) {
	it, err := h.app.Connections.Create(ctx, fromConnectionInput(*r.Body))
	if err != nil {
		return nil, err
	}
	return gen.CreateConnection201JSONResponse(toConnection(*it)), nil
}

func (h *handlers) TestConnection(ctx context.Context, r gen.TestConnectionRequestObject) (gen.TestConnectionResponseObject, error) {
	t, err := h.app.Connections.Test(ctx, fromConnectionInput(*r.Body))
	if err != nil {
		return nil, err
	}
	return gen.TestConnection200JSONResponse(toTestResult(t)), nil
}

func (h *handlers) GetConnection(ctx context.Context, r gen.GetConnectionRequestObject) (gen.GetConnectionResponseObject, error) {
	it, err := h.app.Connections.Get(ctx, r.ConnectionId)
	if err != nil {
		return nil, err
	}
	return gen.GetConnection200JSONResponse(toConnection(*it)), nil
}

func (h *handlers) UpdateConnection(ctx context.Context, r gen.UpdateConnectionRequestObject) (gen.UpdateConnectionResponseObject, error) {
	it, err := h.app.Connections.Update(ctx, r.ConnectionId, fromConnectionInput(*r.Body))
	if err != nil {
		return nil, err
	}
	return gen.UpdateConnection200JSONResponse(toConnection(*it)), nil
}

func (h *handlers) DeleteConnection(ctx context.Context, r gen.DeleteConnectionRequestObject) (gen.DeleteConnectionResponseObject, error) {
	if err := h.app.Connections.Delete(ctx, r.ConnectionId); err != nil {
		return nil, err
	}
	return gen.DeleteConnection204Response{}, nil
}

func (h *handlers) PingConnection(ctx context.Context, r gen.PingConnectionRequestObject) (gen.PingConnectionResponseObject, error) {
	t, err := h.app.Connections.Ping(ctx, r.ConnectionId)
	if err != nil {
		return nil, err
	}
	_ = h.app.Connections.TouchOpened(ctx, r.ConnectionId)
	return gen.PingConnection200JSONResponse(toTestResult(t)), nil
}

func (h *handlers) UnlockConnection(ctx context.Context, r gen.UnlockConnectionRequestObject) (gen.UnlockConnectionResponseObject, error) {
	ttl := time.Hour
	if r.Body != nil && r.Body.TtlSeconds != nil {
		ttl = time.Duration(*r.Body.TtlSeconds) * time.Second
	}
	it, err := h.app.Connections.Unlock(ctx, r.ConnectionId, ttl)
	if err != nil {
		return nil, err
	}
	return gen.UnlockConnection200JSONResponse(toConnection(*it)), nil
}

func (h *handlers) LockConnection(ctx context.Context, r gen.LockConnectionRequestObject) (gen.LockConnectionResponseObject, error) {
	it, err := h.app.Connections.Lock(ctx, r.ConnectionId)
	if err != nil {
		return nil, err
	}
	return gen.LockConnection200JSONResponse(toConnection(*it)), nil
}

func (h *handlers) ListAudit(ctx context.Context, r gen.ListAuditRequestObject) (gen.ListAuditResponseObject, error) {
	list, err := h.app.Changes.Audit(ctx, r.ConnectionId, deref(r.Params.Limit, 50))
	if err != nil {
		return nil, err
	}
	out := make(gen.ListAudit200JSONResponse, 0, len(list))
	for _, e := range list {
		out = append(out, toAudit(e))
	}
	return out, nil
}

// --- Explore --------------------------------------------------------------

func (h *handlers) ListDatabases(ctx context.Context, r gen.ListDatabasesRequestObject) (gen.ListDatabasesResponseObject, error) {
	dbs, err := h.app.Explore.Databases(ctx, r.ConnectionId)
	if err != nil {
		return nil, err
	}
	return gen.ListDatabases200JSONResponse(orEmpty(dbs)), nil
}

func (h *handlers) CreateDatabase(ctx context.Context, r gen.CreateDatabaseRequestObject) (gen.CreateDatabaseResponseObject, error) {
	name, err := ident.Parse(r.Body.Name)
	if err != nil {
		return nil, err
	}
	var owner ident.Name
	if o := deref(r.Body.Owner, ""); o != "" {
		if owner, err = ident.Parse(o); err != nil {
			return nil, err
		}
	}
	res, err := h.app.Roles.CreateDatabase(ctx, r.ConnectionId, name, owner, deref(r.Body.Confirmation, ""), deref(r.Body.DryRun, false), actor(ctx), surface)
	if err != nil {
		return nil, err
	}
	return gen.CreateDatabase200JSONResponse(toApplyResult(res)), nil
}

func (h *handlers) GetSchema(ctx context.Context, r gen.GetSchemaRequestObject) (gen.GetSchemaResponseObject, error) {
	db, err := h.app.Explore.Schema(ctx, r.ConnectionId, r.DatabaseName, deref(r.Params.Refresh, false))
	if err != nil {
		return nil, err
	}
	return gen.GetSchema200JSONResponse(*db.Fill()), nil
}

func (h *handlers) GetTable(ctx context.Context, r gen.GetTableRequestObject) (gen.GetTableResponseObject, error) {
	q, err := qualified(gen.Qualified{Schema: r.Schema, Name: r.Table})
	if err != nil {
		return nil, err
	}
	t, err := h.app.Explore.Table(ctx, r.ConnectionId, r.DatabaseName, q)
	if err != nil {
		return nil, err
	}
	return gen.GetTable200JSONResponse(*t.Fill()), nil
}

func (h *handlers) QueryRows(ctx context.Context, r gen.QueryRowsRequestObject) (gen.QueryRowsResponseObject, error) {
	q, err := qualified(r.Body.Table)
	if err != nil {
		return nil, err
	}
	rq := engine.RowsQuery{Table: q, Limit: deref(r.Body.Limit, 100), Offset: deref(r.Body.Offset, 0)}
	if r.Body.Filters != nil {
		for _, f := range *r.Body.Filters {
			col, err := ident.Parse(f.Column)
			if err != nil {
				return nil, err
			}
			flt := engine.Filter{Column: col, Op: engine.FilterOp(f.Op)}
			if f.Value != nil {
				flt.Value = *f.Value
			}
			rq.Filters = append(rq.Filters, flt)
		}
	}
	if r.Body.Sort != nil {
		for _, s := range *r.Body.Sort {
			col, err := ident.Parse(s.Column)
			if err != nil {
				return nil, err
			}
			rq.Sort = append(rq.Sort, engine.Sort{Column: col, Desc: deref(s.Desc, false)})
		}
	}
	page, err := h.app.Explore.Rows(ctx, r.ConnectionId, r.DatabaseName, rq)
	if err != nil {
		return nil, err
	}
	return gen.QueryRows200JSONResponse{Result: *page.Result, Total: page.Total}, nil
}

// --- Changes --------------------------------------------------------------

func (h *handlers) PlanChanges(ctx context.Context, r gen.PlanChangesRequestObject) (gen.PlanChangesResponseObject, error) {
	cs := *r.Body
	if cs.Database == "" {
		cs.Database = ident.Name(r.DatabaseName)
	}
	res, err := h.app.Changes.Plan(ctx, r.ConnectionId, r.DatabaseName, &cs)
	if err != nil {
		return nil, err
	}
	return gen.PlanChanges200JSONResponse(toPlanResult(res)), nil
}

func (h *handlers) ApplyChanges(ctx context.Context, r gen.ApplyChangesRequestObject) (gen.ApplyChangesResponseObject, error) {
	cs := r.Body.ChangeSet
	if cs.Database == "" {
		cs.Database = ident.Name(r.DatabaseName)
	}
	res, err := h.app.Changes.Apply(ctx, changes.ApplyInput{
		ConnectionID: r.ConnectionId,
		Database:     r.DatabaseName,
		ChangeSet:    &cs,
		Confirmation: deref(r.Body.Confirmation, ""),
		Transaction:  deref(r.Body.Transaction, true),
		Actor:        actor(ctx),
		Surface:      surface,
	})
	if err != nil {
		return nil, err
	}
	return gen.ApplyChanges200JSONResponse(toApplyResult(res)), nil
}

// --- Query ----------------------------------------------------------------

func (h *handlers) RunQuery(ctx context.Context, r gen.RunQueryRequestObject) (gen.RunQueryResponseObject, error) {
	res, err := h.app.Query.Run(ctx, query.RunInput{
		ConnectionID: r.ConnectionId,
		Database:     r.DatabaseName,
		SQL:          r.Body.Sql,
		MaxRows:      deref(r.Body.MaxRows, 500),
	})
	if err != nil {
		return nil, err
	}
	return gen.RunQuery200JSONResponse(*res), nil
}

func (h *handlers) ExplainQuery(ctx context.Context, r gen.ExplainQueryRequestObject) (gen.ExplainQueryResponseObject, error) {
	lines, err := h.app.Query.Explain(ctx, r.ConnectionId, r.DatabaseName, r.Body.Sql, deref(r.Body.Analyze, false))
	if err != nil {
		return nil, err
	}
	return gen.ExplainQuery200JSONResponse{Lines: lines}, nil
}

func (h *handlers) ListHistory(ctx context.Context, r gen.ListHistoryRequestObject) (gen.ListHistoryResponseObject, error) {
	list, err := h.app.Query.History(ctx, r.ConnectionId, deref(r.Params.Limit, 50))
	if err != nil {
		return nil, err
	}
	out := make(gen.ListHistory200JSONResponse, 0, len(list))
	for _, q := range list {
		out = append(out, toQueryRun(q))
	}
	return out, nil
}

func (h *handlers) ListSavedQueries(ctx context.Context, _ gen.ListSavedQueriesRequestObject) (gen.ListSavedQueriesResponseObject, error) {
	list, err := h.app.Query.Saved(ctx)
	if err != nil {
		return nil, err
	}
	out := make(gen.ListSavedQueries200JSONResponse, 0, len(list))
	for _, q := range list {
		out = append(out, toSavedQuery(q))
	}
	return out, nil
}

func (h *handlers) SaveQuery(ctx context.Context, r gen.SaveQueryRequestObject) (gen.SaveQueryResponseObject, error) {
	q := store.SavedQuery{Name: r.Body.Name, SQL: r.Body.Sql}
	if r.Body.Id != nil {
		q.ID = *r.Body.Id
	}
	if r.Body.ConnectionId != nil {
		id := *r.Body.ConnectionId
		q.ConnectionID = &id
	}
	saved, err := h.app.Query.Save(ctx, q)
	if err != nil {
		return nil, err
	}
	return gen.SaveQuery200JSONResponse(toSavedQuery(*saved)), nil
}

func (h *handlers) DeleteSavedQuery(ctx context.Context, r gen.DeleteSavedQueryRequestObject) (gen.DeleteSavedQueryResponseObject, error) {
	if err := h.app.Query.DeleteSaved(ctx, r.QueryId); err != nil {
		return nil, err
	}
	return gen.DeleteSavedQuery204Response{}, nil
}

// --- Monitor --------------------------------------------------------------

func (h *handlers) GetMonitorOverview(ctx context.Context, r gen.GetMonitorOverviewRequestObject) (gen.GetMonitorOverviewResponseObject, error) {
	ov, err := h.app.Monitor.Overview(ctx, r.ConnectionId, r.DatabaseName)
	if err != nil {
		return nil, err
	}
	return gen.GetMonitorOverview200JSONResponse(*ov), nil
}

func (h *handlers) GetMonitorSnapshot(ctx context.Context, r gen.GetMonitorSnapshotRequestObject) (gen.GetMonitorSnapshotResponseObject, error) {
	rng := ""
	if r.Params.Range != nil {
		rng = string(*r.Params.Range)
	}
	s, err := h.app.Monitor.Snapshot(ctx, r.ConnectionId, r.DatabaseName, rng)
	if err != nil {
		return nil, err
	}
	return gen.GetMonitorSnapshot200JSONResponse(toSnapshot(s)), nil
}

func (h *handlers) ListSessions(ctx context.Context, r gen.ListSessionsRequestObject) (gen.ListSessionsResponseObject, error) {
	list, err := h.app.Monitor.Sessions(ctx, r.ConnectionId, r.DatabaseName)
	if err != nil {
		return nil, err
	}
	return gen.ListSessions200JSONResponse(orEmpty(list)), nil
}

func (h *handlers) KillSession(ctx context.Context, r gen.KillSessionRequestObject) (gen.KillSessionResponseObject, error) {
	terminate := false
	if r.Body != nil {
		terminate = deref(r.Body.Terminate, false)
	}
	if err := h.app.Monitor.Kill(ctx, r.ConnectionId, r.DatabaseName, r.Pid, terminate); err != nil {
		return nil, err
	}
	return gen.KillSession204Response{}, nil
}

func (h *handlers) GetEnginePanel(ctx context.Context, r gen.GetEnginePanelRequestObject) (gen.GetEnginePanelResponseObject, error) {
	p, err := h.app.Monitor.Panel(ctx, r.ConnectionId, r.DatabaseName)
	if err != nil {
		return nil, err
	}
	return gen.GetEnginePanel200JSONResponse(*p), nil
}

// --- Roles ----------------------------------------------------------------

func (h *handlers) ListRoles(ctx context.Context, r gen.ListRolesRequestObject) (gen.ListRolesResponseObject, error) {
	list, err := h.app.Roles.List(ctx, r.ConnectionId)
	if err != nil {
		return nil, err
	}
	return gen.ListRoles200JSONResponse(orEmpty(list)), nil
}

func (h *handlers) CreateRole(ctx context.Context, r gen.CreateRoleRequestObject) (gen.CreateRoleResponseObject, error) {
	name, err := ident.Parse(r.Body.Name)
	if err != nil {
		return nil, err
	}
	in := roles.CreateInput{
		ConnectionID: r.ConnectionId,
		Role: engine.RoleInput{
			Name:       name,
			Password:   deref(r.Body.Password, ""),
			CanLogin:   deref(r.Body.CanLogin, true),
			CreateDB:   deref(r.Body.CreateDb, false),
			CreateRole: deref(r.Body.CreateRole, false),
			Superuser:  deref(r.Body.Superuser, false),
			ConnLimit:  deref(r.Body.ConnLimit, -1),
		},
		Confirmation: deref(r.Body.Confirmation, ""),
		DryRun:       deref(r.Body.DryRun, false),
		Actor:        actor(ctx),
		Surface:      surface,
	}
	if r.Body.MemberOf != nil {
		for _, m := range *r.Body.MemberOf {
			n, err := ident.Parse(m)
			if err != nil {
				return nil, err
			}
			in.Role.MemberOf = append(in.Role.MemberOf, n)
		}
	}
	res, err := h.app.Roles.Create(ctx, in)
	if err != nil {
		return nil, err
	}
	return gen.CreateRole200JSONResponse(toApplyResult(res)), nil
}

func (h *handlers) DropRole(ctx context.Context, r gen.DropRoleRequestObject) (gen.DropRoleResponseObject, error) {
	name, err := ident.Parse(r.Role)
	if err != nil {
		return nil, err
	}
	res, err := h.app.Roles.Drop(ctx, r.ConnectionId, name, deref(r.Params.Confirmation, ""), deref(r.Params.DryRun, false), actor(ctx), surface)
	if err != nil {
		return nil, err
	}
	return gen.DropRole200JSONResponse(toApplyResult(res)), nil
}

// --- Designer -------------------------------------------------------------

func (h *handlers) GetDesignerModel(ctx context.Context, r gen.GetDesignerModelRequestObject) (gen.GetDesignerModelResponseObject, error) {
	m, err := h.app.Designer.Model(ctx, r.ConnectionId, r.DatabaseName)
	if err != nil {
		return nil, err
	}
	return gen.GetDesignerModel200JSONResponse{Schema: *m.Schema.Fill(), Dbml: m.DBML, Layout: toLayout(m.Layout), Problems: toProblems(m.Problems)}, nil
}

func (h *handlers) DiffDesignerModel(ctx context.Context, r gen.DiffDesignerModelRequestObject) (gen.DiffDesignerModelResponseObject, error) {
	target := r.Body.Schema
	d, err := h.app.Designer.Diff(ctx, r.ConnectionId, r.DatabaseName, &target)
	if err != nil {
		return nil, err
	}
	return gen.DiffDesignerModel200JSONResponse(toDiff(d)), nil
}

func (h *handlers) SaveDesignerLayout(ctx context.Context, r gen.SaveDesignerLayoutRequestObject) (gen.SaveDesignerLayoutResponseObject, error) {
	if err := h.app.Designer.SaveLayout(ctx, r.ConnectionId, r.DatabaseName, fromLayout(*r.Body)); err != nil {
		return nil, err
	}
	return gen.SaveDesignerLayout204Response{}, nil
}

func (h *handlers) ParseDbml(_ context.Context, r gen.ParseDbmlRequestObject) (gen.ParseDbmlResponseObject, error) {
	db, problems := h.app.Designer.Parse(r.Body.Dbml)
	out := gen.ParseResult{Problems: toProblems(problems)}
	if db != nil {
		out.Schema = db.Fill()
	}
	return gen.ParseDbml200JSONResponse(out), nil
}

func (h *handlers) PrintDbml(_ context.Context, r gen.PrintDbmlRequestObject) (gen.PrintDbmlResponseObject, error) {
	db := r.Body.Schema
	return gen.PrintDbml200JSONResponse{Dbml: h.app.Designer.Print(&db)}, nil
}

// Silence unused-import checks for types referenced only in conversions.
var (
	_ = schema.Database{}
	_ = changeset.Plan{}
	_ = errs.Unknown
)

// orEmpty turns a nil list into an empty one so JSON carries [] not null.
func orEmpty[T any](list []T) []T {
	if list == nil {
		return []T{}
	}
	return list
}

func (h *handlers) PlanScript(ctx context.Context, r gen.PlanScriptRequestObject) (gen.PlanScriptResponseObject, error) {
	res, err := h.app.Changes.Apply(ctx, changes.ApplyInput{
		ConnectionID: r.ConnectionId, Database: r.DatabaseName,
		Plan:   changeset.FromScript(ident.Name(r.DatabaseName), r.Body.Sql),
		DryRun: true, Actor: actor(ctx), Surface: surface,
	})
	if err != nil {
		return nil, err
	}
	return gen.PlanScript200JSONResponse(gen.PlanResult{Plan: *res.Plan, Decision: res.Decision, UpSql: res.Plan.SQL(), DownSql: res.Plan.DownSQL()}), nil
}

func (h *handlers) ApplyScript(ctx context.Context, r gen.ApplyScriptRequestObject) (gen.ApplyScriptResponseObject, error) {
	res, err := h.app.Changes.Apply(ctx, changes.ApplyInput{
		ConnectionID: r.ConnectionId, Database: r.DatabaseName,
		Plan:         changeset.FromScript(ident.Name(r.DatabaseName), r.Body.Sql),
		Confirmation: deref(r.Body.Confirmation, ""),
		Transaction:  deref(r.Body.Transaction, true),
		Actor:        actor(ctx), Surface: surface,
	})
	if err != nil {
		return nil, err
	}
	return gen.ApplyScript200JSONResponse(toApplyResult(res)), nil
}
