// Package enginetest is the conformance suite every engine must pass. It
// builds its fixture through the engine's own Dialect, so the suite is
// engine-neutral: the engine only supplies a session on an empty database
// and the spellings of a few types.
package enginetest

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/felipemalacarne/mesa/internal/core/changeset"
	"github.com/felipemalacarne/mesa/internal/core/errs"
	"github.com/felipemalacarne/mesa/internal/core/ident"
	"github.com/felipemalacarne/mesa/internal/core/schema"
	"github.com/felipemalacarne/mesa/internal/core/value"
	"github.com/felipemalacarne/mesa/internal/engine"
)

// Types are the engine's spellings for the fixture's columns.
type Types struct {
	Int, Text, Bool, Decimal, WideDecimal string
}

// Fixture describes the engine to the suite.
type Fixture struct {
	Engine engine.Engine
	// Open returns a session on a fresh, empty database. It registers its
	// own cleanup.
	Open func(t *testing.T) engine.Session
	// Schema is the default schema new tables land in ("public").
	Schema ident.Name
	Types  Types
	// SelectOne is a query returning a single row with one column "one" = 1.
	SelectOne string
	// QuoteTable renders a table name in hand-written SQL; nil uses the
	// standard double quotes.
	QuoteTable func(ident.Qualified) string
}

func (f Fixture) quote(q ident.Qualified) string {
	if f.QuoteTable != nil {
		return f.QuoteTable(q)
	}
	return q.Quote()
}

// Run runs every conformance test.
func Run(t *testing.T, f Fixture) {
	t.Run("ping and overview", func(t *testing.T) { testOverview(t, f) })
	t.Run("schema round trip", func(t *testing.T) { testSchemaRoundTrip(t, f) })
	t.Run("rows", func(t *testing.T) { testRows(t, f) })
	t.Run("row edits and their reversal", func(t *testing.T) { testRowEdits(t, f) })
	t.Run("schema change and its reversal", func(t *testing.T) { testSchemaChangeReversal(t, f) })
	t.Run("read-only query", func(t *testing.T) { testReadOnly(t, f) })
	t.Run("monitor", func(t *testing.T) { testMonitor(t, f) })
}

func ctx(t *testing.T) context.Context {
	c, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	t.Cleanup(cancel)
	return c
}

// model is the fixture: authors 1—n books.
func model(f Fixture) *schema.Database {
	def := "true"
	return &schema.Database{Schemas: []schema.Schema{{
		Name: f.Schema,
		Tables: []schema.Table{
			{
				Name: "authors",
				Columns: []schema.Column{
					{Name: "id", Type: f.Types.Int},
					{Name: "name", Type: f.Types.Text},
					{Name: "active", Type: f.Types.Bool, Default: &def},
				},
				PrimaryKey: &schema.Key{Name: "authors_pkey", Columns: []ident.Name{"id"}},
			},
			{
				Name: "books",
				Columns: []schema.Column{
					{Name: "id", Type: f.Types.Int},
					{Name: "author_id", Type: f.Types.Int},
					{Name: "title", Type: f.Types.Text},
					{Name: "price", Type: f.Types.Decimal, Nullable: true},
				},
				PrimaryKey:  &schema.Key{Name: "books_pkey", Columns: []ident.Name{"id"}},
				Uniques:     []schema.Key{{Name: "books_title_key", Columns: []ident.Name{"title"}}},
				Indexes:     []schema.Index{{Name: "books_author_idx", Columns: []ident.Name{"author_id"}}},
				ForeignKeys: []schema.ForeignKey{{Name: "books_author_id_fkey", Columns: []ident.Name{"author_id"}, References: ident.Q(f.Schema, "authors"), RefColumns: []ident.Name{"id"}, OnDelete: "CASCADE"}},
			},
		},
	}}}
}

// create builds the fixture through the engine's dialect.
func create(t *testing.T, f Fixture, s engine.Session) *schema.Database {
	t.Helper()
	m := model(f)
	empty, err := s.Database(ctx(t))
	if err != nil {
		t.Fatalf("Database on an empty database: %v", err)
	}
	changes := schema.DiffWith(empty, m, opts(s))
	apply(t, s, empty, &changeset.ChangeSet{Schema: changes})
	return m
}

func opts(s engine.Session) schema.DiffOptions {
	return schema.DiffOptions{SameType: s.SameType, SameDefault: s.SameDefault}
}

// apply compiles a change set and runs its statements in one transaction.
func apply(t *testing.T, s engine.Session, current *schema.Database, cs *changeset.ChangeSet) *changeset.Plan {
	t.Helper()
	p, err := s.Compile(ctx(t), current, cs)
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}
	run(t, s, p.Statements)
	return p
}

func run(t *testing.T, s engine.Session, stmts []changeset.Statement) {
	t.Helper()
	err := s.Tx(ctx(t), func(tx engine.Tx) error {
		for _, st := range stmts {
			n, err := tx.Exec(ctx(t), st.SQL, st.Args)
			if err != nil {
				return errors.New(st.Display() + ": " + err.Error())
			}
			if st.ExpectRows > 0 && n != st.ExpectRows {
				return errors.New(st.Display() + ": wrong row count")
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
}

func live(t *testing.T, s engine.Session) *schema.Database {
	t.Helper()
	db, err := s.Database(ctx(t))
	if err != nil {
		t.Fatalf("Database: %v", err)
	}
	return db
}

func assertSame(t *testing.T, s engine.Session, got, want *schema.Database) {
	t.Helper()
	if d := schema.DiffWith(got, want, opts(s)); len(d) > 0 {
		for _, c := range d {
			t.Errorf("unexpected difference: %s", c.Describe())
		}
	}
}

func testOverview(t *testing.T, f Fixture) {
	s := f.Open(t)
	if err := s.Ping(ctx(t)); err != nil {
		t.Fatalf("Ping: %v", err)
	}
	ov, err := s.Overview(ctx(t))
	if err != nil {
		t.Fatalf("Overview: %v", err)
	}
	if ov.Version == "" || ov.Database == "" || ov.Role == "" {
		t.Errorf("Overview is missing fields: %+v", ov)
	}
	dbs, err := s.Databases(ctx(t))
	if err != nil {
		t.Fatalf("Databases: %v", err)
	}
	found := false
	for _, d := range dbs {
		found = found || d.Name.String() == ov.Database
	}
	if !found {
		t.Errorf("Databases does not list the session's database %q: %+v", ov.Database, dbs)
	}
}

func testSchemaRoundTrip(t *testing.T, f Fixture) {
	s := f.Open(t)
	m := create(t, f, s)
	got := live(t, s)
	assertSame(t, s, got, m)
	books := got.Table(ident.Q(f.Schema, "books"))
	if books == nil {
		t.Fatal("books is missing from the catalog")
	}
	if len(books.ForeignKeys) != 1 || books.ForeignKeys[0].OnDelete != "CASCADE" {
		t.Errorf("books foreign key: %+v", books.ForeignKeys)
	}
	tbl, err := s.Table(ctx(t), ident.Q(f.Schema, "books"))
	if err != nil {
		t.Fatalf("Table: %v", err)
	}
	if len(tbl.Columns) != 4 || !tbl.IsPrimaryKey("id") {
		t.Errorf("Table(books) = %+v", tbl)
	}
	if _, err := s.Table(ctx(t), ident.Q(f.Schema, "nope")); !errs.Is(err, errs.NotFound) {
		t.Errorf("Table(nope) error = %v, want NotFound", err)
	}
}

func seed(t *testing.T, f Fixture, s engine.Session, current *schema.Database) {
	t.Helper()
	authors := ident.Q(f.Schema, "authors")
	books := ident.Q(f.Schema, "books")
	cs := &changeset.ChangeSet{}
	for i, name := range []string{"Ursula", "Octavia", "Ted"} {
		cs.Rows = append(cs.Rows, changeset.RowChange{Kind: changeset.Insert, Table: authors, Values: map[ident.Name]value.Value{
			"id": value.Int(int64(i + 1)), "name": value.Text(name), "active": value.Bool(name != "Ted"),
		}})
	}
	for i, title := range []string{"The Dispossessed", "Kindred", "Parable of the Sower", "Stories of Your Life"} {
		author := []int64{1, 2, 2, 3}[i]
		cs.Rows = append(cs.Rows, changeset.RowChange{Kind: changeset.Insert, Table: books, Values: map[ident.Name]value.Value{
			"id": value.Int(int64(i + 1)), "author_id": value.Int(author), "title": value.Text(title), "price": value.Decimal("12.50"),
		}})
	}
	apply(t, s, current, cs)
}

func testRows(t *testing.T, f Fixture) {
	s := f.Open(t)
	m := create(t, f, s)
	seed(t, f, s, m)

	page, err := s.TableRows(ctx(t), engine.RowsQuery{
		Table:   ident.Q(f.Schema, "authors"),
		Filters: []engine.Filter{{Column: "active", Op: engine.OpEq, Value: value.Bool(true)}},
		Sort:    []engine.Sort{{Column: "name", Desc: true}},
		Limit:   1,
	})
	if err != nil {
		t.Fatalf("TableRows: %v", err)
	}
	if page.Total != 2 {
		t.Errorf("Total = %d, want 2", page.Total)
	}
	if len(page.Result.Rows) != 1 {
		t.Fatalf("rows = %d, want 1", len(page.Result.Rows))
	}
	name := column(t, page.Result, "name")
	if got := page.Result.Rows[0][name].Text(); got != "Ursula" {
		t.Errorf("first row name = %q, want Ursula (sorted desc)", got)
	}

	page, err = s.TableRows(ctx(t), engine.RowsQuery{
		Table:   ident.Q(f.Schema, "books"),
		Filters: []engine.Filter{{Column: "title", Op: engine.OpILike, Value: value.Text("%parable%")}},
	})
	if err != nil {
		t.Fatalf("TableRows ilike: %v", err)
	}
	if page.Total != 1 {
		t.Errorf("ilike Total = %d, want 1", page.Total)
	}
	price := column(t, page.Result, "price")
	if v := page.Result.Rows[0][price]; v.Kind != value.KindDecimal || v.Text() != "12.50" {
		t.Errorf("price = %+v (%s), want decimal 12.50", v, v.Text())
	}

	res, err := s.Query(ctx(t), f.SelectOne, nil, engine.QueryOptions{})
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if len(res.Rows) != 1 || res.Rows[0][0].Text() != "1" {
		t.Errorf("SelectOne = %+v", res.Rows)
	}
	lines, err := s.Explain(ctx(t), f.SelectOne, nil, false)
	if err != nil || len(lines) == 0 {
		t.Errorf("Explain = %v, %v", lines, err)
	}
	if _, err := s.Query(ctx(t), "this is not sql", nil, engine.QueryOptions{}); !errs.Is(err, errs.Invalid) {
		t.Errorf("bad SQL error = %v (kind %v), want Invalid", err, errs.KindOf(err))
	}
}

func column(t *testing.T, r *engine.Result, name string) int {
	t.Helper()
	for i, c := range r.Columns {
		if c.Name == name {
			return i
		}
	}
	t.Fatalf("no column %q in %+v", name, r.Columns)
	return -1
}

// countBooks counts the fixture's books matching the filters.
func countBooks(t *testing.T, s engine.Session, f Fixture, filters ...engine.Filter) int64 {
	t.Helper()
	page, err := s.TableRows(ctx(t), engine.RowsQuery{Table: ident.Q(f.Schema, "books"), Filters: filters, Limit: 1})
	if err != nil {
		t.Fatalf("TableRows(books): %v", err)
	}
	return page.Total
}

func testRowEdits(t *testing.T, f Fixture) {
	s := f.Open(t)
	m := create(t, f, s)
	seed(t, f, s, m)
	books := ident.Q(f.Schema, "books")
	current := live(t, s)

	// Update with the old value in the key: reversible.
	upd := apply(t, s, current, &changeset.ChangeSet{Rows: []changeset.RowChange{{
		Kind: changeset.Update, Table: books,
		Key:    map[ident.Name]value.Value{"id": value.Int(2), "title": value.Text("Kindred")},
		Values: map[ident.Name]value.Value{"title": value.Text("Kindred (1979)")},
	}}})
	if countBooks(t, s, f, engine.Filter{Column: "title", Op: engine.OpEq, Value: value.Text("Kindred (1979)")}) != 1 {
		t.Fatal("update did not apply")
	}
	if len(upd.Down) != 1 {
		t.Fatalf("update has %d down statements, want 1", len(upd.Down))
	}
	run(t, s, upd.Down)
	if countBooks(t, s, f, engine.Filter{Column: "title", Op: engine.OpEq, Value: value.Text("Kindred")}) != 1 {
		t.Fatal("update reversal did not restore the title")
	}

	// Delete with every column in the key: reversible.
	page, err := s.TableRows(ctx(t), engine.RowsQuery{Table: books, Filters: []engine.Filter{{Column: "id", Op: engine.OpEq, Value: value.Int(4)}}})
	if err != nil || len(page.Result.Rows) != 1 {
		t.Fatalf("read book 4: %v", err)
	}
	key := map[ident.Name]value.Value{}
	for i, c := range page.Result.Columns {
		key[ident.Name(c.Name)] = page.Result.Rows[0][i]
	}
	del := apply(t, s, current, &changeset.ChangeSet{Rows: []changeset.RowChange{{Kind: changeset.Delete, Table: books, Key: key}}})
	if countBooks(t, s, f) != 3 {
		t.Fatal("delete did not apply")
	}
	run(t, s, del.Down)
	if countBooks(t, s, f) != 4 {
		t.Fatal("delete reversal did not re-insert the row")
	}

	// A stale key matches nothing: the statement must report zero rows.
	stale, err := s.Compile(ctx(t), current, &changeset.ChangeSet{Rows: []changeset.RowChange{{
		Kind: changeset.Update, Table: books,
		Key:    map[ident.Name]value.Value{"id": value.Int(2), "title": value.Text("not the title")},
		Values: map[ident.Name]value.Value{"price": value.Decimal("1.00")},
	}}})
	if err != nil {
		t.Fatalf("Compile stale: %v", err)
	}
	err = s.Tx(ctx(t), func(tx engine.Tx) error {
		n, err := tx.Exec(ctx(t), stale.Statements[0].SQL, stale.Statements[0].Args)
		if err == nil && n != 0 {
			t.Errorf("stale update touched %d rows, want 0", n)
		}
		if stale.Statements[0].ExpectRows != 1 {
			t.Errorf("row statements must expect exactly one row")
		}
		return err
	})
	if err != nil {
		t.Fatalf("stale update: %v", err)
	}

	// Cascade: deleting an author removes their books.
	apply(t, s, current, &changeset.ChangeSet{Rows: []changeset.RowChange{{
		Kind: changeset.Delete, Table: ident.Q(f.Schema, "authors"), Key: map[ident.Name]value.Value{"id": value.Int(2)},
	}}})
	if n := countBooks(t, s, f); n != 2 {
		t.Errorf("after cascading delete books = %d, want 2", n)
	}
}

func testSchemaChangeReversal(t *testing.T, f Fixture) {
	s := f.Open(t)
	m := create(t, f, s)
	seed(t, f, s, m)
	before := live(t, s)

	after := live(t, s)
	books := after.Table(ident.Q(f.Schema, "books"))
	books.Columns = append(books.Columns, schema.Column{Name: "subtitle", Type: f.Types.Text, Nullable: true})
	for i := range books.Columns {
		if books.Columns[i].Name == "price" {
			books.Columns[i].Type = f.Types.WideDecimal
		}
	}
	books.Indexes = append(books.Indexes, schema.Index{Name: "books_price_idx", Columns: []ident.Name{"price"}})
	books.Comment = "Books we sell"

	changes := schema.DiffWith(before, after, opts(s))
	if len(changes) != 4 {
		for _, c := range changes {
			t.Logf("change: %s", c.Describe())
		}
		t.Fatalf("diff has %d changes, want 4 (add column, widen type, index, comment)", len(changes))
	}
	p := apply(t, s, before, &changeset.ChangeSet{Schema: changes})
	if p.Has(changeset.RiskDataLoss) {
		t.Errorf("widening a type is not data loss: %+v", p.Risks)
	}
	assertSame(t, s, live(t, s), after)

	run(t, s, p.Down)
	assertSame(t, s, live(t, s), before)

	// Dropping a column is data loss, and its reversal re-adds the column.
	drop := apply(t, s, before, &changeset.ChangeSet{Schema: []schema.Change{{Kind: schema.DropColumn, Table: ident.Q(f.Schema, "books"), Name: "price"}}})
	if !drop.Has(changeset.RiskDataLoss) || drop.Worst() != changeset.Danger {
		t.Errorf("drop column risks = %+v, want danger data_loss", drop.Risks)
	}
	run(t, s, drop.Down)
	if live(t, s).Table(ident.Q(f.Schema, "books")).Column("price") == nil {
		t.Error("drop column reversal did not re-add the column")
	}
}

func testReadOnly(t *testing.T, f Fixture) {
	s := f.Open(t)
	m := create(t, f, s)
	seed(t, f, s, m)
	_, err := s.Query(ctx(t), "DELETE FROM "+f.quote(ident.Q(f.Schema, "books")), nil, engine.QueryOptions{ReadOnly: true})
	if !errs.Is(err, errs.Forbidden) {
		t.Errorf("write in a read-only query: error = %v (kind %v), want Forbidden", err, errs.KindOf(err))
	}
	if n := countBooks(t, s, f); n != 4 {
		t.Errorf("read-only query changed data: books = %d", n)
	}
	res, err := s.Query(ctx(t), "SELECT * FROM "+f.quote(ident.Q(f.Schema, "books")), nil, engine.QueryOptions{MaxRows: 2})
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if len(res.Rows) != 2 || !res.Truncated {
		t.Errorf("MaxRows: rows = %d truncated = %v, want 2 and true", len(res.Rows), res.Truncated)
	}
}

func testMonitor(t *testing.T, f Fixture) {
	s := f.Open(t)
	mt, err := s.Metrics(ctx(t))
	if err != nil {
		t.Fatalf("Metrics: %v", err)
	}
	if mt.ConnectionsMax <= 0 || mt.SampledAt.IsZero() {
		t.Errorf("Metrics = %+v", mt)
	}
	sessions, err := s.Sessions(ctx(t))
	if err != nil {
		t.Fatalf("Sessions: %v", err)
	}
	_ = sessions // the session's own backend may be filtered out; listing must not fail
	if f.Engine.Capabilities().EnginePanel {
		p, err := s.EnginePanel(ctx(t))
		if err != nil {
			t.Fatalf("EnginePanel: %v", err)
		}
		if p.Title == "" || len(p.Tiles)+len(p.Meters)+len(p.Tables) == 0 {
			t.Errorf("EnginePanel is empty: %+v", p)
		}
	}
	if _, err := s.Roles(ctx(t)); err != nil && f.Engine.Capabilities().Roles {
		t.Errorf("Roles: %v", err)
	}
}
