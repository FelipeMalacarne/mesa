// Package storetest is the conformance suite every store implementation
// passes. Backends call Run from their own tests with an opener that returns
// a fresh, empty store.
package storetest

import (
	"bytes"
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/felipemalacarne/mesa/internal/core/account"
	"github.com/felipemalacarne/mesa/internal/core/connection"
	"github.com/felipemalacarne/mesa/internal/core/errs"
	"github.com/felipemalacarne/mesa/internal/store"
)

// Run exercises every port against stores from open. open is called once
// per subtest and must return an empty store, registering its own cleanup.
func Run(t *testing.T, open func(t *testing.T) store.Store) {
	t.Helper()
	t.Run("connections", func(t *testing.T) { testConnections(t, open(t)) })
	t.Run("secrets", func(t *testing.T) { testSecrets(t, open(t)) })
	t.Run("audit", func(t *testing.T) { testAudit(t, open(t)) })
	t.Run("layouts", func(t *testing.T) { testLayouts(t, open(t)) })
	t.Run("history", func(t *testing.T) { testHistory(t, open(t)) })
	t.Run("saved queries", func(t *testing.T) { testSavedQueries(t, open(t)) })
	t.Run("metrics", func(t *testing.T) { testMetrics(t, open(t)) })
	t.Run("users", func(t *testing.T) { testUsers(t, open(t)) })
	t.Run("tokens", func(t *testing.T) { testTokens(t, open(t)) })
}

func testMetrics(t *testing.T, s store.Store) {
	ctx := context.Background()
	id := uuid.New()
	m := s.Metrics()
	// Ten samples a minute apart; two buckets of five.
	for i := 0; i < 10; i++ {
		p := store.MetricPoint{At: base.Add(time.Duration(i) * time.Minute), TPS: float64(i), Active: i, LatencyMs: 1, SizeBytes: int64(1000 + i)}
		if err := m.Add(ctx, id, "shop", p); err != nil {
			t.Fatalf("Add: %v", err)
		}
	}
	if err := m.Add(ctx, id, "other", store.MetricPoint{At: base, TPS: 99}); err != nil {
		t.Fatal(err)
	}
	got, err := m.Range(ctx, id, "shop", base, base.Add(10*time.Minute), 2)
	if err != nil {
		t.Fatalf("Range: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("Range buckets = %d, want 2: %+v", len(got), got)
	}
	if got[0].TPS != 2 || got[1].TPS != 7 || got[1].SizeBytes != 1009 || got[0].Active != 2 {
		t.Errorf("Range averages = %+v", got)
	}
	if !got[0].At.Before(got[1].At) {
		t.Error("Range must be oldest first")
	}
	if err := m.Prune(ctx, base.Add(5*time.Minute)); err != nil {
		t.Fatalf("Prune: %v", err)
	}
	got, err = m.Range(ctx, id, "shop", base, base.Add(10*time.Minute), 10)
	if err != nil || len(got) != 5 {
		t.Errorf("after Prune: %d points, %v", len(got), err)
	}
}

// base is a fixed instant with microsecond precision, the finest both
// backends keep.
var base = time.Date(2026, time.October, 21, 12, 0, 0, 123456000, time.UTC)

func at(offset time.Duration) time.Time { return base.Add(offset) }

func testConnections(t *testing.T, st store.Store) {
	ctx := context.Background()
	conns := st.Connections()

	if got, err := conns.List(ctx); err != nil || len(got) != 0 {
		t.Fatalf("List on an empty store = %v, %v; want none", got, err)
	}
	if _, err := conns.Get(ctx, uuid.New()); !errs.Is(err, errs.NotFound) {
		t.Fatalf("Get of a missing connection = %v, want NotFound", err)
	}

	alpha := newConnection(t, "alpha", connection.Production)
	beta := newConnection(t, "beta", connection.Local)
	for _, c := range []connection.Connection{beta, alpha} {
		if err := conns.Put(ctx, c); err != nil {
			t.Fatalf("Put %s: %v", c.Name, err)
		}
	}

	got, err := conns.List(ctx)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(got) != 2 || got[0].Name != "alpha" || got[1].Name != "beta" {
		t.Fatalf("List = %v, want alpha then beta", names(got))
	}
	assertConnection(t, alpha, got[0])

	one, err := conns.Get(ctx, beta.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	assertConnection(t, beta, *one)

	// Put with an existing ID updates in place.
	readOnly := true
	err = alpha.Update(connection.Input{
		Name: "alpha-2", Engine: "mysql", Environment: "staging", Host: "db2.internal",
		Port: 3307, Database: "shop", User: "reporter", TLS: "require", ReadOnly: &readOnly,
	}, at(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if err := conns.Put(ctx, alpha); err != nil {
		t.Fatalf("Put update: %v", err)
	}
	updated, err := conns.Get(ctx, alpha.ID)
	if err != nil {
		t.Fatalf("Get after update: %v", err)
	}
	assertConnection(t, alpha, *updated)
	if got, _ := conns.List(ctx); len(got) != 2 {
		t.Fatalf("List after update has %d connections, want 2", len(got))
	}

	// TouchOpened and LastOpened.
	if err := conns.TouchOpened(ctx, uuid.New(), at(0)); !errs.Is(err, errs.NotFound) {
		t.Fatalf("TouchOpened of a missing connection = %v, want NotFound", err)
	}
	opened, err := conns.LastOpened(ctx)
	if err != nil || len(opened) != 0 {
		t.Fatalf("LastOpened before any open = %v, %v; want none", opened, err)
	}
	if err := conns.TouchOpened(ctx, alpha.ID, at(time.Minute)); err != nil {
		t.Fatalf("TouchOpened: %v", err)
	}
	if err := conns.TouchOpened(ctx, alpha.ID, at(2*time.Minute)); err != nil {
		t.Fatalf("TouchOpened again: %v", err)
	}
	opened, err = conns.LastOpened(ctx)
	if err != nil {
		t.Fatalf("LastOpened: %v", err)
	}
	if len(opened) != 1 || !opened[alpha.ID].Equal(at(2*time.Minute)) {
		t.Fatalf("LastOpened = %v, want only alpha at %v", opened, at(2*time.Minute))
	}

	// Delete is final and idempotent.
	if err := conns.Delete(ctx, alpha.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := conns.Get(ctx, alpha.ID); !errs.Is(err, errs.NotFound) {
		t.Fatalf("Get after Delete = %v, want NotFound", err)
	}
	if err := conns.Delete(ctx, alpha.ID); err != nil {
		t.Fatalf("Delete again: %v", err)
	}
	if got, _ := conns.List(ctx); len(got) != 1 || got[0].ID != beta.ID {
		t.Fatalf("List after Delete = %v, want only beta", names(got))
	}
	if opened, _ := conns.LastOpened(ctx); len(opened) != 0 {
		t.Fatalf("LastOpened after Delete = %v, want none", opened)
	}
}

func testSecrets(t *testing.T, st store.Store) {
	ctx := context.Background()
	secrets := st.Secrets()
	id := uuid.New()

	if _, err := secrets.Get(ctx, id); !errs.Is(err, errs.NotFound) {
		t.Fatalf("Get of a missing secret = %v, want NotFound", err)
	}
	blob := []byte{0x00, 0xff, 0x10, 'a', 'b'}
	if err := secrets.Set(ctx, id, blob); err != nil {
		t.Fatalf("Set: %v", err)
	}
	got, err := secrets.Get(ctx, id)
	if err != nil || !bytes.Equal(got, blob) {
		t.Fatalf("Get = %x, %v; want %x", got, err, blob)
	}

	blob = []byte("rotated")
	if err := secrets.Set(ctx, id, blob); err != nil {
		t.Fatalf("Set again: %v", err)
	}
	if got, err := secrets.Get(ctx, id); err != nil || !bytes.Equal(got, blob) {
		t.Fatalf("Get after rotation = %q, %v; want %q", got, err, blob)
	}

	other := uuid.New()
	if err := secrets.Set(ctx, other, []byte("other")); err != nil {
		t.Fatalf("Set other: %v", err)
	}
	if err := secrets.Delete(ctx, id); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := secrets.Get(ctx, id); !errs.Is(err, errs.NotFound) {
		t.Fatalf("Get after Delete = %v, want NotFound", err)
	}
	if err := secrets.Delete(ctx, id); err != nil {
		t.Fatalf("Delete again: %v", err)
	}
	if got, err := secrets.Get(ctx, other); err != nil || string(got) != "other" {
		t.Fatalf("other secret after Delete = %q, %v; want it untouched", got, err)
	}
}

func testAudit(t *testing.T, st store.Store) {
	ctx := context.Background()
	audit := st.Audit()
	connA, connB := uuid.New(), uuid.New()

	if got, err := audit.List(ctx, nil, 10); err != nil || len(got) != 0 {
		t.Fatalf("List on an empty store = %v, %v; want none", got, err)
	}

	first := store.AuditEntry{
		ID: uuid.New(), ConnectionID: connA, Database: "app", Environment: connection.Staging,
		Actor: "ana", Surface: "http", Summary: "add column users.nick",
		SQL: "ALTER TABLE users ADD COLUMN nick text", DownSQL: "ALTER TABLE users DROP COLUMN nick",
		StartedAt: at(0),
	}
	second := store.AuditEntry{
		ID: uuid.New(), ConnectionID: connB, Database: "shop", Environment: connection.Local,
		Actor: "cli", Surface: "cli", Summary: "drop table junk", SQL: "DROP TABLE junk",
		Status: store.AuditStarted, StartedAt: at(time.Minute),
	}
	third := store.AuditEntry{
		ID: uuid.New(), ConnectionID: connA, Database: "app", Environment: connection.Staging,
		Actor: "ana", Surface: "mcp", Summary: "create index", SQL: "CREATE INDEX i ON t (c)",
		Status: store.AuditStarted, StartedAt: at(2 * time.Minute),
	}
	for _, e := range []store.AuditEntry{first, second, third} {
		if err := audit.Begin(ctx, e); err != nil {
			t.Fatalf("Begin %s: %v", e.Summary, err)
		}
	}

	all, err := audit.List(ctx, nil, 10)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	assertIDs(t, "List all", auditIDs(all), third.ID, second.ID, first.ID)
	assertAuditEntry(t, first, all[2])
	if all[2].Status != store.AuditStarted {
		t.Fatalf("Begin without a status stored %q, want %q", all[2].Status, store.AuditStarted)
	}

	forA, err := audit.List(ctx, &connA, 10)
	if err != nil {
		t.Fatalf("List for a connection: %v", err)
	}
	assertIDs(t, "List for connection A", auditIDs(forA), third.ID, first.ID)

	limited, err := audit.List(ctx, nil, 2)
	if err != nil {
		t.Fatalf("List with limit: %v", err)
	}
	assertIDs(t, "List limit 2", auditIDs(limited), third.ID, second.ID)

	// Finish records the outcome.
	if err := audit.Finish(ctx, first.ID, store.AuditApplied, "", at(5*time.Second)); err != nil {
		t.Fatalf("Finish applied: %v", err)
	}
	if err := audit.Finish(ctx, second.ID, store.AuditFailed, "permission denied", at(time.Minute+time.Second)); err != nil {
		t.Fatalf("Finish failed: %v", err)
	}
	if err := audit.Finish(ctx, uuid.New(), store.AuditApplied, "", at(0)); !errs.Is(err, errs.NotFound) {
		t.Fatalf("Finish of a missing entry = %v, want NotFound", err)
	}
	all, err = audit.List(ctx, nil, 10)
	if err != nil {
		t.Fatalf("List after Finish: %v", err)
	}
	byID := map[uuid.UUID]store.AuditEntry{}
	for _, e := range all {
		byID[e.ID] = e
	}
	if e := byID[first.ID]; e.Status != store.AuditApplied || e.Error != "" || e.FinishedAt == nil || !e.FinishedAt.Equal(at(5*time.Second)) {
		t.Fatalf("applied entry = %+v, want applied at %v with no error", e, at(5*time.Second))
	}
	if e := byID[second.ID]; e.Status != store.AuditFailed || e.Error != "permission denied" || e.FinishedAt == nil || !e.FinishedAt.Equal(at(time.Minute+time.Second)) {
		t.Fatalf("failed entry = %+v, want failed with its error and finish time", e)
	}
	if e := byID[third.ID]; e.Status != store.AuditStarted || e.FinishedAt != nil {
		t.Fatalf("unfinished entry = %+v, want it still started", e)
	}
}

func testLayouts(t *testing.T, st store.Store) {
	ctx := context.Background()
	layouts := st.Layouts()
	conn := uuid.New()

	if _, err := layouts.Get(ctx, conn, "app"); !errs.Is(err, errs.NotFound) {
		t.Fatalf("Get of a missing layout = %v, want NotFound", err)
	}

	first := map[string]store.Position{
		"public.users": {X: 10, Y: 20.5},
		"public.posts": {X: -3, Y: 0},
	}
	if err := layouts.Put(ctx, conn, "app", first); err != nil {
		t.Fatalf("Put: %v", err)
	}
	got, err := layouts.Get(ctx, conn, "app")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	assertPositions(t, first, got)

	// Put replaces the whole layout rather than merging.
	second := map[string]store.Position{"public.users": {X: 1, Y: 2}}
	if err := layouts.Put(ctx, conn, "app", second); err != nil {
		t.Fatalf("Put replace: %v", err)
	}
	got, err = layouts.Get(ctx, conn, "app")
	if err != nil {
		t.Fatalf("Get after replace: %v", err)
	}
	assertPositions(t, second, got)

	// Layouts are per database, and an empty layout is still a layout.
	if _, err := layouts.Get(ctx, conn, "shop"); !errs.Is(err, errs.NotFound) {
		t.Fatalf("Get of another database = %v, want NotFound", err)
	}
	if err := layouts.Put(ctx, conn, "shop", nil); err != nil {
		t.Fatalf("Put empty: %v", err)
	}
	got, err = layouts.Get(ctx, conn, "shop")
	if err != nil || got == nil || len(got) != 0 {
		t.Fatalf("Get of an empty layout = %v, %v; want an empty map", got, err)
	}
}

func testHistory(t *testing.T, st store.Store) {
	ctx := context.Background()
	history := st.History()
	connA, connB := uuid.New(), uuid.New()

	if got, err := history.List(ctx, connA, 10); err != nil || len(got) != 0 {
		t.Fatalf("List on an empty store = %v, %v; want none", got, err)
	}

	runs := []store.QueryRun{
		{ID: uuid.New(), ConnectionID: connA, Database: "app", SQL: "select 1", RowCount: 1, Duration: 15 * time.Millisecond, RanAt: at(0)},
		{ID: uuid.New(), ConnectionID: connA, Database: "app", SQL: "select * from nope", Error: `relation "nope" does not exist`, Duration: 2 * time.Millisecond, RanAt: at(time.Second)},
		{ID: uuid.New(), ConnectionID: connB, Database: "shop", SQL: "select 2", RowCount: 1, Duration: time.Millisecond, RanAt: at(time.Second)},
		{ID: uuid.New(), ConnectionID: connA, Database: "analytics", SQL: "select count(*) from events", RowCount: 1, Duration: 1200 * time.Millisecond, RanAt: at(2 * time.Second)},
	}
	for _, r := range runs {
		if err := history.Add(ctx, r); err != nil {
			t.Fatalf("Add %q: %v", r.SQL, err)
		}
	}

	got, err := history.List(ctx, connA, 10)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	assertIDs(t, "List for connection A", runIDs(got), runs[3].ID, runs[1].ID, runs[0].ID)
	assertQueryRun(t, runs[1], got[1])
	assertQueryRun(t, runs[3], got[0])

	limited, err := history.List(ctx, connA, 2)
	if err != nil {
		t.Fatalf("List with limit: %v", err)
	}
	assertIDs(t, "List limit 2", runIDs(limited), runs[3].ID, runs[1].ID)

	forB, err := history.List(ctx, connB, 10)
	if err != nil {
		t.Fatalf("List for connection B: %v", err)
	}
	assertIDs(t, "List for connection B", runIDs(forB), runs[2].ID)
}

func testSavedQueries(t *testing.T, st store.Store) {
	ctx := context.Background()
	queries := st.Queries()
	conn := uuid.New()

	if got, err := queries.List(ctx); err != nil || len(got) != 0 {
		t.Fatalf("List on an empty store = %v, %v; want none", got, err)
	}

	anywhere := store.SavedQuery{ID: uuid.New(), Name: "slow queries", SQL: "select * from pg_stat_statements", UpdatedAt: at(0)}
	scoped := store.SavedQuery{ID: uuid.New(), ConnectionID: &conn, Name: "active users", SQL: "select count(*) from users where active", UpdatedAt: at(time.Minute)}
	for _, q := range []store.SavedQuery{anywhere, scoped} {
		if err := queries.Put(ctx, q); err != nil {
			t.Fatalf("Put %q: %v", q.Name, err)
		}
	}

	got, err := queries.List(ctx)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	assertIDs(t, "List", queryIDs(got), scoped.ID, anywhere.ID) // by name
	assertSavedQuery(t, scoped, got[0])
	assertSavedQuery(t, anywhere, got[1])

	// Put with an existing ID updates in place, including the scope.
	anywhere.Name, anywhere.SQL, anywhere.ConnectionID, anywhere.UpdatedAt = "slowest", "select 1", &conn, at(time.Hour)
	if err := queries.Put(ctx, anywhere); err != nil {
		t.Fatalf("Put update: %v", err)
	}
	got, err = queries.List(ctx)
	if err != nil {
		t.Fatalf("List after update: %v", err)
	}
	assertIDs(t, "List after update", queryIDs(got), scoped.ID, anywhere.ID)
	assertSavedQuery(t, anywhere, got[1])

	if err := queries.Delete(ctx, scoped.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if err := queries.Delete(ctx, scoped.ID); err != nil {
		t.Fatalf("Delete again: %v", err)
	}
	got, err = queries.List(ctx)
	if err != nil {
		t.Fatalf("List after Delete: %v", err)
	}
	assertIDs(t, "List after Delete", queryIDs(got), anywhere.ID)
}

// --- helpers ---------------------------------------------------------------

func newConnection(t *testing.T, name string, env connection.Environment) connection.Connection {
	t.Helper()
	c, err := connection.New(connection.Input{
		Name: name, Engine: "postgres", Environment: string(env), Host: name + ".db.internal",
		Port: 5433, Database: "app", User: "mesa", TLS: "verify-full",
	}, at(0))
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func names(conns []connection.Connection) []string {
	out := make([]string, len(conns))
	for i, c := range conns {
		out[i] = c.Name
	}
	return out
}

func assertConnection(t *testing.T, want, got connection.Connection) {
	t.Helper()
	same := got.ID == want.ID && got.Name == want.Name && got.Engine == want.Engine &&
		got.Environment == want.Environment && got.Host == want.Host && got.Port == want.Port &&
		got.Database == want.Database && got.User == want.User && got.TLS == want.TLS &&
		got.ReadOnly == want.ReadOnly && got.CreatedAt.Equal(want.CreatedAt) && got.UpdatedAt.Equal(want.UpdatedAt)
	if !same {
		t.Fatalf("connection round trip:\n got %+v\nwant %+v", got, want)
	}
}

func assertAuditEntry(t *testing.T, want, got store.AuditEntry) {
	t.Helper()
	same := got.ID == want.ID && got.ConnectionID == want.ConnectionID && got.Database == want.Database &&
		got.Environment == want.Environment && got.Actor == want.Actor && got.Surface == want.Surface &&
		got.Summary == want.Summary && got.SQL == want.SQL && got.DownSQL == want.DownSQL &&
		got.Error == want.Error && got.StartedAt.Equal(want.StartedAt) && got.FinishedAt == nil
	if !same {
		t.Fatalf("audit entry round trip:\n got %+v\nwant %+v", got, want)
	}
}

func assertQueryRun(t *testing.T, want, got store.QueryRun) {
	t.Helper()
	same := got.ID == want.ID && got.ConnectionID == want.ConnectionID && got.Database == want.Database &&
		got.SQL == want.SQL && got.RowCount == want.RowCount && got.Duration == want.Duration &&
		got.Error == want.Error && got.RanAt.Equal(want.RanAt)
	if !same {
		t.Fatalf("query run round trip:\n got %+v\nwant %+v", got, want)
	}
}

func assertSavedQuery(t *testing.T, want, got store.SavedQuery) {
	t.Helper()
	sameScope := (want.ConnectionID == nil && got.ConnectionID == nil) ||
		(want.ConnectionID != nil && got.ConnectionID != nil && *want.ConnectionID == *got.ConnectionID)
	same := got.ID == want.ID && sameScope && got.Name == want.Name && got.SQL == want.SQL &&
		got.UpdatedAt.Equal(want.UpdatedAt)
	if !same {
		t.Fatalf("saved query round trip:\n got %+v\nwant %+v", got, want)
	}
}

func assertPositions(t *testing.T, want, got map[string]store.Position) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("positions = %v, want %v", got, want)
	}
	for name, p := range want {
		if got[name] != p {
			t.Fatalf("positions = %v, want %v", got, want)
		}
	}
}

func assertIDs(t *testing.T, what string, got []uuid.UUID, want ...uuid.UUID) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("%s returned %d rows, want %d", what, len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("%s order: row %d is %s, want %s", what, i, got[i], want[i])
		}
	}
}

func auditIDs(entries []store.AuditEntry) []uuid.UUID {
	out := make([]uuid.UUID, len(entries))
	for i, e := range entries {
		out[i] = e.ID
	}
	return out
}

func runIDs(runs []store.QueryRun) []uuid.UUID {
	out := make([]uuid.UUID, len(runs))
	for i, r := range runs {
		out[i] = r.ID
	}
	return out
}

func queryIDs(queries []store.SavedQuery) []uuid.UUID {
	out := make([]uuid.UUID, len(queries))
	for i, q := range queries {
		out[i] = q.ID
	}
	return out
}

func newUser(t *testing.T, subject, email string, role account.Role) account.User {
	t.Helper()
	u, err := account.New(account.Input{Subject: subject, Email: email, Role: role}, base)
	if err != nil {
		t.Fatal(err)
	}
	return u
}

func testUsers(t *testing.T, st store.Store) {
	ctx := context.Background()
	users := st.Users()

	if list, err := users.List(ctx); err != nil || len(list) != 0 {
		t.Fatalf("List on an empty store = %v, %v", list, err)
	}
	ana := newUser(t, "https://auth.example.com 1", "ana@example.com", account.Admin)
	bo := newUser(t, "https://auth.example.com 2", "bo@example.com", account.Viewer)
	for _, u := range []account.User{bo, ana} {
		if err := users.Put(ctx, u); err != nil {
			t.Fatalf("Put %s: %v", u.Email, err)
		}
	}
	list, err := users.List(ctx)
	if err != nil || len(list) != 2 || list[0].Name != "ana" || list[1].Name != "bo" {
		t.Fatalf("List = %+v, %v", list, err)
	}
	got, err := users.GetBySubject(ctx, bo.Subject)
	if err != nil || got.ID != bo.ID || got.Role != account.Viewer || got.Email != "bo@example.com" || !got.CreatedAt.Equal(base) {
		t.Fatalf("GetBySubject = %+v, %v", got, err)
	}
	if _, err := users.GetBySubject(ctx, "nobody"); !errs.Is(err, errs.NotFound) {
		t.Fatalf("GetBySubject of a stranger = %v, want NotFound", err)
	}
	if _, err := users.Get(ctx, uuid.New()); !errs.Is(err, errs.NotFound) {
		t.Fatalf("Get of a missing user = %v, want NotFound", err)
	}

	// The provider changed their role and name: Put updates in place.
	if err := ana.Update(account.Input{Subject: ana.Subject, Email: ana.Email, Name: "Ana Lima", Role: account.Editor}, at(time.Hour)); err != nil {
		t.Fatal(err)
	}
	if err := users.Put(ctx, ana); err != nil {
		t.Fatalf("Put update: %v", err)
	}
	got, err = users.Get(ctx, ana.ID)
	if err != nil || got.Name != "Ana Lima" || got.Role != account.Editor || !got.UpdatedAt.Equal(at(time.Hour)) {
		t.Fatalf("Get after update = %+v, %v", got, err)
	}
	if err := users.TouchSeen(ctx, ana.ID, at(2*time.Hour)); err != nil {
		t.Fatalf("TouchSeen: %v", err)
	}
	if got, _ := users.Get(ctx, ana.ID); got.LastSeenAt == nil || !got.LastSeenAt.Equal(at(2*time.Hour)) {
		t.Fatalf("LastSeenAt = %v", got.LastSeenAt)
	}
}

func testTokens(t *testing.T, st store.Store) {
	ctx := context.Background()
	ana := newUser(t, "https://auth.example.com 1", "ana@example.com", account.Editor)
	if err := st.Users().Put(ctx, ana); err != nil {
		t.Fatal(err)
	}
	tokens := st.Tokens()
	expires := at(time.Hour)
	session := store.Token{ID: uuid.New(), UserID: ana.ID, Kind: store.SessionToken, Hash: []byte{1, 2, 3}, CreatedAt: at(0), ExpiresAt: &expires}
	agent := store.Token{ID: uuid.New(), UserID: ana.ID, Kind: store.APIToken, Name: "claude", Hash: []byte{4, 5, 6}, CreatedAt: at(time.Minute)}
	older := store.Token{ID: uuid.New(), UserID: ana.ID, Kind: store.APIToken, Name: "ci", Hash: []byte{7}, CreatedAt: at(0)}
	for _, tok := range []store.Token{session, agent, older} {
		if err := tokens.Create(ctx, tok); err != nil {
			t.Fatalf("Create %s: %v", tok.Name, err)
		}
	}
	got, err := tokens.ByHash(ctx, []byte{1, 2, 3})
	if err != nil || got.ID != session.ID || got.Kind != store.SessionToken || got.ExpiresAt == nil || !got.ExpiresAt.Equal(expires) {
		t.Fatalf("ByHash = %+v, %v", got, err)
	}
	if _, err := tokens.ByHash(ctx, []byte{9}); !errs.Is(err, errs.NotFound) {
		t.Fatalf("ByHash of an unknown hash = %v, want NotFound", err)
	}
	list, err := tokens.List(ctx, ana.ID, store.APIToken)
	if err != nil || len(list) != 2 || list[0].Name != "claude" || list[1].Name != "ci" {
		t.Fatalf("List = %+v, %v", list, err)
	}
	if err := tokens.Touch(ctx, agent.ID, at(2*time.Minute)); err != nil {
		t.Fatal(err)
	}
	if got, _ := tokens.ByHash(ctx, agent.Hash); got.LastUsedAt == nil || !got.LastUsedAt.Equal(at(2*time.Minute)) {
		t.Fatalf("LastUsedAt = %v", got.LastUsedAt)
	}
	if err := tokens.Prune(ctx, at(2*time.Hour)); err != nil {
		t.Fatalf("Prune: %v", err)
	}
	if _, err := tokens.ByHash(ctx, session.Hash); !errs.Is(err, errs.NotFound) {
		t.Fatalf("expired session after Prune = %v, want NotFound", err)
	}
	if err := tokens.Delete(ctx, older.ID); err != nil {
		t.Fatal(err)
	}
	if list, _ := tokens.List(ctx, ana.ID, store.APIToken); len(list) != 1 {
		t.Fatalf("List after Delete = %d tokens", len(list))
	}
	if err := tokens.DeleteForUser(ctx, ana.ID, ""); err != nil {
		t.Fatal(err)
	}
	if list, _ := tokens.List(ctx, ana.ID, store.APIToken); len(list) != 0 {
		t.Fatalf("List after DeleteForUser = %d tokens", len(list))
	}
}
