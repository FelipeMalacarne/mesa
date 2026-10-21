package postgres

import (
	"strings"
	"testing"

	"github.com/felipemalacarne/mesa/internal/core/changeset"
	"github.com/felipemalacarne/mesa/internal/core/ident"
	"github.com/felipemalacarne/mesa/internal/core/schema"
	"github.com/felipemalacarne/mesa/internal/core/value"
)

func strp(s string) *string { return &s }

func current() *schema.Database {
	return &schema.Database{Name: "shop", Schemas: []schema.Schema{{
		Name: "public",
		Tables: []schema.Table{{
			Name: "orders",
			Columns: []schema.Column{
				{Name: "id", Type: "bigint", Identity: schema.IdentityAlways},
				{Name: "status", Type: "text", Default: strp("'new'::text")},
				{Name: "total", Type: "numeric(10,2)"},
				{Name: "note", Type: "varchar(200)", Nullable: true},
			},
			PrimaryKey:  &schema.Key{Name: "orders_pkey", Columns: []ident.Name{"id"}},
			Indexes:     []schema.Index{{Name: "orders_open_idx", Columns: []ident.Name{"status"}, Predicate: "status <> 'done'"}},
			Checks:      []schema.Check{{Name: "orders_total_check", Expression: "total >= 0"}},
			RowEstimate: 2_000_000,
		}},
	}}}
}

func compile(t *testing.T, cs *changeset.ChangeSet) *changeset.Plan {
	t.Helper()
	p, err := dialect{version: 16, current: current()}.compile(cs)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	return p
}

var orders = ident.Q("public", "orders")

func TestCompileRowChanges(t *testing.T) {
	p := compile(t, &changeset.ChangeSet{Rows: []changeset.RowChange{
		{Kind: changeset.Insert, Table: orders, Values: map[ident.Name]value.Value{"id": value.Int(7), "total": value.Decimal("9.90")}},
		{Kind: changeset.Update, Table: orders, Key: map[ident.Name]value.Value{"id": value.Int(7), "note": value.Null()}, Values: map[ident.Name]value.Value{"note": value.Text("gift")}},
		{Kind: changeset.Delete, Table: orders, Key: map[ident.Name]value.Value{"id": value.Int(3)}},
	}})
	want := []string{
		`INSERT INTO "public"."orders" ("id", "total") VALUES ($1, $2)`,
		`UPDATE "public"."orders" SET "note" = $1 WHERE "id" = $2 AND "note" IS NULL`,
		`DELETE FROM "public"."orders" WHERE "id" = $1`,
	}
	for i, w := range want {
		if p.Statements[i].SQL != w {
			t.Errorf("statement %d:\n got %s\nwant %s", i, p.Statements[i].SQL, w)
		}
		if p.Statements[i].ExpectRows != 1 {
			t.Errorf("statement %d must expect one row", i)
		}
	}
	if got := p.Statements[1].Display(); !strings.Contains(got, `"note" = 'gift'`) {
		t.Errorf("display does not inline arguments: %s", got)
	}
	// Down runs in reverse: the update is reversed (old value known), the
	// insert removed by key, and the partial delete is irreversible.
	if len(p.Down) != 2 || !strings.HasPrefix(p.Down[0].SQL, "UPDATE") || !strings.HasPrefix(p.Down[1].SQL, "DELETE") {
		t.Errorf("down = %+v", p.Down)
	}
	if !p.Has(changeset.RiskIrreversible) {
		t.Error("deleting with only the key must be flagged irreversible")
	}
}

func TestCompileSchemaChanges(t *testing.T) {
	cur := current()
	tbl := cur.Table(orders)
	newCol := schema.Column{Name: "total", Type: "numeric(8,2)"}
	p := compile(t, &changeset.ChangeSet{Schema: []schema.Change{
		{Kind: schema.AlterColumnType, Table: orders, Column: &newCol, OldColumn: tbl.Column("total")},
		{Kind: schema.AddColumn, Table: orders, Column: &schema.Column{Name: "paid", Type: "boolean"}},
		{Kind: schema.DropIndex, Table: orders, Name: "orders_open_idx"},
		{Kind: schema.DropTable, Table: orders},
	}})
	sql := p.SQL()
	for _, want := range []string{
		`ALTER TABLE "public"."orders" ALTER COLUMN "total" TYPE numeric(8,2) USING "total"::numeric(8,2)`,
		`ALTER TABLE "public"."orders" ADD COLUMN "paid" boolean NOT NULL`,
		`DROP INDEX "public"."orders_open_idx"`,
		`DROP TABLE "public"."orders"`,
	} {
		if !strings.Contains(sql, want) {
			t.Errorf("plan is missing %q:\n%s", want, sql)
		}
	}
	down := p.DownSQL()
	for _, want := range []string{
		`CREATE INDEX "orders_open_idx" ON "public"."orders" ("status") WHERE status <> 'done'`,
		`CONSTRAINT "orders_total_check" CHECK (total >= 0)`,
		`"id" bigint NOT NULL GENERATED ALWAYS AS IDENTITY`,
		`TYPE numeric(10,2)`,
	} {
		if !strings.Contains(down, want) {
			t.Errorf("down is missing %q:\n%s", want, down)
		}
	}
	if !p.Has(changeset.RiskDataLoss) || p.Worst() != changeset.Danger {
		t.Errorf("narrowing and dropping must be danger data loss: %+v", p.Risks)
	}
	if !p.Has(changeset.RiskTableRewrite) {
		t.Errorf("type change on a big table must warn about a rewrite: %+v", p.Risks)
	}
	if !p.Has(changeset.RiskFullScan) {
		t.Errorf("NOT NULL column without default must be flagged: %+v", p.Risks)
	}
}

func TestCompileCreateTable(t *testing.T) {
	tbl := &schema.Table{
		Name: "shipments",
		Columns: []schema.Column{
			{Name: "id", Type: "bigint", Identity: schema.IdentityByDefault},
			{Name: "order_id", Type: "bigint"},
			{Name: "carrier", Type: "text", Comment: "who ships it"},
		},
		PrimaryKey: &schema.Key{Columns: []ident.Name{"id"}},
		Uniques:    []schema.Key{{Columns: []ident.Name{"order_id", "carrier"}}},
		Indexes:    []schema.Index{{Name: "shipments_carrier_idx", Columns: []ident.Name{"carrier"}, Method: "hash"}},
	}
	q := ident.Q("public", "shipments")
	p := compile(t, &changeset.ChangeSet{Schema: []schema.Change{
		{Kind: schema.CreateTable, Table: q, NewTable: tbl},
		{Kind: schema.AddForeignKey, Table: q, ForeignKey: &schema.ForeignKey{Columns: []ident.Name{"order_id"}, References: orders, RefColumns: []ident.Name{"id"}, OnDelete: "SET NULL"}},
	}})
	sql := p.SQL()
	for _, want := range []string{
		`"id" bigint NOT NULL GENERATED BY DEFAULT AS IDENTITY`,
		`CONSTRAINT "shipments_pkey" PRIMARY KEY ("id")`,
		`CONSTRAINT "shipments_order_id_carrier_key" UNIQUE ("order_id", "carrier")`,
		`COMMENT ON COLUMN "public"."shipments"."carrier" IS 'who ships it'`,
		`CREATE INDEX "shipments_carrier_idx" ON "public"."shipments" USING hash ("carrier")`,
		`ADD CONSTRAINT "shipments_order_id_fkey" FOREIGN KEY ("order_id") REFERENCES "public"."orders" ("id") ON DELETE SET NULL`,
	} {
		if !strings.Contains(sql, want) {
			t.Errorf("plan is missing %q:\n%s", want, sql)
		}
	}
	if p.Has(changeset.RiskFullScan) {
		t.Errorf("a foreign key on a new table checks nothing: %+v", p.Risks)
	}
	if p.Down[0].SQL != `ALTER TABLE "public"."shipments" DROP CONSTRAINT "shipments_order_id_fkey"` || p.Down[1].SQL != `DROP TABLE "public"."shipments"` {
		t.Errorf("down = %s", p.DownSQL())
	}
}

func TestShortType(t *testing.T) {
	for in, want := range map[string]string{
		"character varying(120)":      "varchar(120)",
		"character varying":           "varchar",
		"character varying(20)[]":     "varchar(20)[]",
		"timestamp with time zone":    "timestamptz",
		"timestamp(3) with time zone": "timestamptz(3)",
		"timestamp without time zone": "timestamp",
		"time without time zone[]":    "time[]",
		"double precision":            "float8",
		"character(2)":                "char(2)",
		"bigint":                      "bigint",
		"numeric(10,2)":               "numeric(10,2)",
		"public.order_status":         "public.order_status",
	} {
		if got := shortType(in); got != want {
			t.Errorf("shortType(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestSameTypeAndDefault(t *testing.T) {
	same := [][2]string{
		{"varchar(20)", "character varying(20)"},
		{"int", "integer"},
		{"int8", "bigint"},
		{"timestamptz", "timestamp with time zone"},
		{"numeric(10, 2)", "numeric(10,2)"},
		{"order_status", "public.order_status"},
		{"text[]", "text[]"},
	}
	for _, p := range same {
		if !sameType(p[0], p[1]) {
			t.Errorf("sameType(%q, %q) = false", p[0], p[1])
		}
	}
	different := [][2]string{{"varchar(20)", "varchar(30)"}, {"int", "bigint"}, {"text", "text[]"}, {"timestamp", "timestamptz"}}
	for _, p := range different {
		if sameType(p[0], p[1]) {
			t.Errorf("sameType(%q, %q) = true", p[0], p[1])
		}
	}
	for _, p := range [][2]string{
		{"'new'", "'new'::order_status"},
		{"now()", "CURRENT_TIMESTAMP"},
		{"(0)", "0"},
		{"'a'::character varying", "'a'"},
		{"('{}'::jsonb)", "'{}'"},
	} {
		if !sameDefault(p[0], p[1]) {
			t.Errorf("sameDefault(%q, %q) = false", p[0], p[1])
		}
	}
	if sameDefault("'A'", "'a'") {
		t.Error("string literals are case-sensitive")
	}
}
