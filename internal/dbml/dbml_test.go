package dbml

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/felipemalacarne/mesa/internal/core/errs"
	"github.com/felipemalacarne/mesa/internal/core/ident"
	"github.com/felipemalacarne/mesa/internal/core/schema"
)

func ptr(s string) *string { return &s }

func names(ns ...ident.Name) []ident.Name { return ns }

// everything is a model that uses every feature DBML can express, with
// names that are either conventional (left out by Print) or explicit.
func everything() *schema.Database {
	return &schema.Database{Name: "shop", Schemas: []schema.Schema{
		{Name: "billing",
			Enums: []schema.Enum{{Name: "currency", Values: []string{"USD", "EUR", "BRL "}}},
			Tables: []schema.Table{{
				Name:    "accounts",
				Comment: "Accounts of the billing schema",
				Columns: []schema.Column{
					{Name: "id", Type: "uuid", Default: ptr("gen_random_uuid()")},
					{Name: "owner id", Type: "int8"},
					{Name: "currency", Type: "billing.currency", Default: ptr("'USD'")},
					{Name: "balance", Type: "numeric(12,2)", Default: ptr("0")},
					{Name: "tags", Type: "text[]", Nullable: true, Default: ptr("'{}'::text[]")},
					{Name: "active", Type: "boolean", Nullable: true, Default: ptr("true")},
					{Name: "closed_at", Type: "timestamp with time zone", Nullable: true, Default: ptr("null")},
					{Name: "score", Type: "float8", Nullable: true, Default: ptr("-1.5")},
					{Name: "parent_id", Type: "uuid", Nullable: true},
				},
				PrimaryKey: &schema.Key{Name: "accounts_pkey", Columns: names("id")},
				ForeignKeys: []schema.ForeignKey{
					{Name: "accounts_owner id_fkey", Columns: names("owner id"), References: ident.Q("public", "users"), RefColumns: names("id")},
					{Name: "accounts_parent_id_fkey", Columns: names("parent_id"), References: ident.Q("billing", "accounts"), RefColumns: names("id")},
				},
			}},
		},
		{Name: "public",
			Enums: []schema.Enum{{Name: "order_status", Values: []string{"pending", "shipped", "on hold"}}},
			Tables: []schema.Table{
				{
					Name: "order_items",
					Columns: []schema.Column{
						{Name: "order_id", Type: "int8"},
						{Name: "line", Type: "int4"},
						{Name: "sku", Type: "text"},
						{Name: "qty", Type: "int4", Default: ptr("1")},
					},
					PrimaryKey: &schema.Key{Name: "order_items_pk", Columns: names("order_id", "line")},
					ForeignKeys: []schema.ForeignKey{
						{Name: "order_items_order_id_fkey", Columns: names("order_id"), References: ident.Q("public", "orders"), RefColumns: names("id"), OnDelete: "CASCADE", OnUpdate: "CASCADE"},
						{Name: "order_items_skus_fkey", Columns: names("order_id", "sku"), References: ident.Q("public", "skus"), RefColumns: names("order_id", "sku"), OnDelete: "SET NULL"},
					},
				},
				{
					Name: "orders",
					Columns: []schema.Column{
						{Name: "id", Type: "int8"},
						{Name: "user_id", Type: "int8"},
						{Name: "account_id", Type: "uuid"},
						{Name: "status", Type: "order_status", Default: ptr("'pending'")},
						{Name: "placed_at", Type: "timestamptz", Default: ptr("now()")},
					},
					PrimaryKey: &schema.Key{Name: "orders_pkey", Columns: names("id")},
					Uniques: []schema.Key{
						{Name: "orders_account_status_key", Columns: names("account_id", "status")},
						{Name: "orders_user_id_status_key", Columns: names("user_id", "status")},
					},
					Indexes: []schema.Index{
						{Name: "orders_status_placed_at_idx", Columns: names("status", "placed_at")},
					},
					ForeignKeys: []schema.ForeignKey{
						{Name: "billing_account", Columns: names("account_id"), References: ident.Q("billing", "accounts"), RefColumns: names("id"), OnDelete: "RESTRICT", OnUpdate: "CASCADE"},
						{Name: "orders_user_id_fkey", Columns: names("user_id"), References: ident.Q("public", "users"), RefColumns: names("id")},
					},
				},
				{
					Name: "skus",
					Columns: []schema.Column{
						{Name: "order_id", Type: "int8"},
						{Name: "sku", Type: "text"},
					},
					PrimaryKey: &schema.Key{Name: "skus_pkey", Columns: names("order_id", "sku")},
				},
				{
					Name: "users",
					Columns: []schema.Column{
						{Name: "id", Type: "int8", Identity: schema.IdentityByDefault, Comment: "Surrogate key"},
						{Name: "email", Type: "varchar(255)", Default: ptr("''")},
						{Name: "nick", Type: "text", Nullable: true},
						{Name: "motto", Type: "text", Nullable: true, Default: ptr("'it''s fine'")},
						{Name: "bio", Type: "text", Nullable: true, Comment: "Free text.\n  Indented line.\n\nLast line."},
						{Name: "note", Type: "text", Nullable: true, Comment: "a column named like a keyword"},
					},
					PrimaryKey: &schema.Key{Name: "users_pkey", Columns: names("id")},
					Uniques: []schema.Key{
						{Name: "users_email_key", Columns: names("email")},
						{Name: "users_nick_key", Columns: names("nick")},
					},
					Indexes: []schema.Index{
						{Name: "users_email_lower_idx", Expression: "lower(email)", Unique: true, Method: "btree"},
						{Name: "users_expr_idx", Expression: "now()"},
						{Name: "users_expr_idx1", Expression: "now()"},
						{Name: "users_id_expr_idx", Columns: names("id"), Expression: "lower(nick)"},
						{Name: "users_nick_idx", Columns: names("nick"), Method: "hash"},
					},
				},
			},
		},
	}}
}

func TestRoundTrip(t *testing.T) {
	want := everything()
	want.Normalize()
	text := Print(want)
	got, err := Parse(text)
	if err != nil {
		t.Fatalf("Parse(Print(db)): %v\n%s", err, text)
	}
	got.Normalize()
	if !reflect.DeepEqual(got, want) {
		gj, _ := json.MarshalIndent(got, "", "  ")
		wj, _ := json.MarshalIndent(want, "", "  ")
		t.Fatalf("Parse(Print(db)) differs from db\n--- dbml ---\n%s\n--- got ---\n%s\n--- want ---\n%s", text, gj, wj)
	}
	if again := Print(got); again != text {
		t.Errorf("Print is not idempotent:\n%s\n---\n%s", text, again)
	}
	if problems := Validate(got); len(problems) != 0 {
		t.Errorf("Validate(round-tripped model) = %v, want none", problems)
	}
}

func TestPrintDoesNotModifyInput(t *testing.T) {
	db := everything()
	before, _ := json.Marshal(db)
	Print(db)
	after, _ := json.Marshal(db)
	if string(before) != string(after) {
		t.Error("Print modified its argument")
	}
	if Print(nil) != "" {
		t.Error("Print(nil) should be empty")
	}
}

func TestParseDefaults(t *testing.T) {
	db, err := Parse(`
Table t {
  id int [pk]
  a text [not null]
  b text [null]
  c text
  d int [increment]
  e numeric(10, 2) [default: 1.5]
  f text [default: "dq"]
}`)
	if err != nil {
		t.Fatal(err)
	}
	tbl := db.Table(ident.Q("public", "t"))
	if tbl == nil {
		t.Fatal("table t not found in the public schema")
	}
	nullable := map[ident.Name]bool{}
	for _, c := range tbl.Columns {
		nullable[c.Name] = c.Nullable
	}
	for name, want := range map[ident.Name]bool{"id": false, "a": false, "b": true, "c": true, "d": true} {
		if nullable[name] != want {
			t.Errorf("column %s nullable = %v, want %v", name, nullable[name], want)
		}
	}
	if tbl.PrimaryKey == nil || tbl.PrimaryKey.Name != "t_pkey" || !sameNames(tbl.PrimaryKey.Columns, names("id")) {
		t.Errorf("primary key = %+v", tbl.PrimaryKey)
	}
	if c := tbl.Column("d"); c.Identity != schema.IdentityByDefault {
		t.Errorf("increment: identity = %q", c.Identity)
	}
	if c := tbl.Column("e"); c.Type != "numeric(10,2)" || *c.Default != "1.5" {
		t.Errorf("column e = %+v", c)
	}
	if c := tbl.Column("f"); *c.Default != "'dq'" {
		t.Errorf("column f default = %q", *c.Default)
	}
}

func TestParseRefs(t *testing.T) {
	db, err := Parse(`
Table a as A { id int [pk]  b_id int [ref: > B.id]  c_id int [ref: - c.id] }
Table b as B { id int [pk, ref: < a.id] }
Table c { id int [pk] }
Table d { a_id int  b_id int }
Ref d_pair: d.(a_id, b_id) > a.(id, b_id) [delete: set default]
Ref: d.a_id > a.id
Ref: d.a_id > a.id [update: restrict]
`)
	if err != nil {
		t.Fatal(err)
	}
	a := db.Table(ident.Q("public", "a"))
	fks := map[ident.Name]schema.ForeignKey{}
	for _, fk := range a.ForeignKeys {
		fks[fk.Name] = fk
	}
	if fk := fks["a_b_id_fkey"]; fk.References != ident.Q("public", "b") || !sameNames(fk.RefColumns, names("id")) {
		t.Errorf("alias ref: %+v", fk)
	}
	if fk := fks["a_c_id_fkey"]; fk.References != ident.Q("public", "c") {
		t.Errorf("one-to-one ref: %+v", fk)
	}
	if fk, ok := fks["a_id_fkey"]; !ok || fk.References != ident.Q("public", "b") || !sameNames(fk.Columns, names("id")) {
		t.Errorf("< ref should put the foreign key on the right table: %+v", a.ForeignKeys)
	}
	d := db.Table(ident.Q("public", "d"))
	if len(d.ForeignKeys) != 2 {
		t.Fatalf("d should have 2 foreign keys (the repeated ref merges), got %+v", d.ForeignKeys)
	}
	if fk := d.ForeignKeys[0]; fk.Name != "d_pair" || !sameNames(fk.Columns, names("a_id", "b_id")) || fk.OnDelete != "SET DEFAULT" {
		t.Errorf("composite ref: %+v", fk)
	}
	if fk := d.ForeignKeys[1]; fk.Name != "d_a_id_fkey" || fk.OnUpdate != "RESTRICT" || fk.OnDelete != "" {
		t.Errorf("merged ref: %+v", fk)
	}
}

func TestParseStrings(t *testing.T) {
	db, err := Parse("Table t {\n  a text [note: 'it\\'s \\\\ \\n done']\n  Note: '''\n    first\n      second\n\n    third\n  '''\n}")
	if err != nil {
		t.Fatal(err)
	}
	tbl := db.Schemas[0].Tables[0]
	if got, want := tbl.Column("a").Comment, "it's \\ \n done"; got != want {
		t.Errorf("escapes: got %q want %q", got, want)
	}
	if got, want := tbl.Comment, "first\n  second\n\nthird"; got != want {
		t.Errorf("multi-line: got %q want %q", got, want)
	}
}

func TestParseWindowsLineEndings(t *testing.T) {
	db, err := Parse("Table t {\r\n  id int [pk]\r\n  Note: '''\r\n    one\r\n    two\r\n  '''\r\n}\r\n")
	if err != nil {
		t.Fatal(err)
	}
	if got := db.Schemas[0].Tables[0].Comment; got != "one\ntwo" {
		t.Errorf("comment = %q", got)
	}
}

func TestParseErrors(t *testing.T) {
	cases := []struct {
		src       string
		line, col int
		want      string
	}{
		{"Table t {\n  id int\n", 1, 1, "expected '}'"},
		{"Table t {\n  id int [pk]\n  x\n}", 4, 1, "expected a type for column x"},
		{"Table t {\n  id int [pk, bogus]\n}", 2, 15, "unknown column setting 'bogus'"},
		{"Table t {\n  id int [default: foo]\n}", 2, 20, "unsupported default"},
		{"Table t { id int }\nTable t { id int }", 2, 7, "table public.t is defined twice"},
		{"Table t { id int [pk]\n  indexes { (id) [pk] } }", 2, 13, "more than one primary key"},
		{"Table t { id int [ref: <> u.id] }", 1, 19, "many-to-many"},
		{"Ref: t.id > u.id", 1, 6, "unknown table public.t"},
		{"Table t { id int }\nRef: t.(id, x) > u.id", 2, 6, "joins 2 columns to 1"},
		{"Table t { a text [note: 'open]\n}", 1, 25, "unterminated string"},
		{"Table t { a int [default: `now(]\n}", 1, 27, "unterminated expression"},
		{"Table t { a int }\n/* open", 2, 1, "unterminated block comment"},
		{"Table t { a int } $", 1, 19, "unexpected character '$'"},
		{"Widget w { }", 1, 1, "expected Table, Enum, Ref, Project or TableGroup"},
		{"Table t { \"\" int }", 1, 11, "identifier is empty"},
		{"Table t { a int }\nRef: t.a > u.b [delete: maybe]", 2, 17, "expected cascade, restrict"},
		{"Table t { a int\n indexes { a [name: 'x', bogus] } }", 2, 26, "unknown index setting 'bogus'"},
	}
	for _, c := range cases {
		_, err := Parse(c.src)
		if err == nil {
			t.Errorf("Parse(%q) succeeded, want error %q", c.src, c.want)
			continue
		}
		if !errs.Is(err, errs.Invalid) {
			t.Errorf("Parse(%q) error kind = %v, want Invalid", c.src, errs.KindOf(err))
		}
		msg := errs.Message(err)
		if !strings.Contains(msg, c.want) {
			t.Errorf("Parse(%q) = %q, want it to mention %q", c.src, msg, c.want)
		}
		if prefix := formatPosition(c.line, c.col); !strings.HasPrefix(msg, prefix) {
			t.Errorf("Parse(%q) = %q, want position %q", c.src, msg, prefix)
		}
	}
}

func formatPosition(line, col int) string {
	return (&syntaxError{line: line, col: col}).Error()
}

func TestPrintValues(t *testing.T) {
	cases := map[string]string{
		"0":              "0",
		"-12.5":          "-12.5",
		"TRUE":           "true",
		"NULL":           "null",
		"'free'":         "'free'",
		"'it''s'":        `'it\'s'`,
		"'free'::text":   "`'free'::text`",
		"now()":          "`now()`",
		"nextval('s')":   "`nextval('s')`",
		"'a' || 'b'":     "`'a' || 'b'`",
		"CURRENT_DATE":   "`CURRENT_DATE`",
		"'multi\nline'":  "'multi\\nline'",
		"'back\\slash'":  "'back\\\\slash'",
		"'quote'' ''x'":  `'quote\' \'x'`,
		"''":             "''",
		"'unterminated":  "`'unterminated`",
		"'a'::int + 1":   "`'a'::int + 1`",
		"'x'::text, 'y'": "`'x'::text, 'y'`",
	}
	for in, want := range cases {
		if got := defaultValue(in); got != want {
			t.Errorf("defaultValue(%q) = %s, want %s", in, got, want)
		}
	}
	types := map[string]string{
		"int8":                     "int8",
		"varchar(120)":             "varchar(120)",
		"numeric(10,2)":            "numeric(10,2)",
		"numeric(10, 2)":           `"numeric(10, 2)"`,
		"billing.currency":         "billing.currency",
		"text[]":                   "text[]",
		"int4[][]":                 "int4[][]",
		"timestamp with time zone": `"timestamp with time zone"`,
		"int[3]":                   `"int[3]"`,
		"":                         `""`,
	}
	for in, want := range types {
		if got := typeText(in); got != want {
			t.Errorf("typeText(%q) = %s, want %s", in, got, want)
		}
	}
	words := map[ident.Name]string{
		"customers":  "customers",
		"CustomerId": "CustomerId",
		"my col":     `"my col"`,
		"1st":        `"1st"`,
		"note":       `"note"`,
		"Table":      `"Table"`,
		`say "hi"`:   `"say \"hi\""`,
		"créditos":   `"créditos"`,
	}
	for in, want := range words {
		if got := word(in); got != want {
			t.Errorf("word(%q) = %s, want %s", in, got, want)
		}
	}
}

func TestPrintNonPublicSchemaAndBlocks(t *testing.T) {
	db := &schema.Database{Schemas: []schema.Schema{{Name: "crm", Tables: []schema.Table{{
		Name:    "leads",
		Columns: []schema.Column{{Name: "id", Type: "int8"}, {Name: "score", Type: "int4", Nullable: true}},
		// A non-conventional primary key name has to go through the
		// indexes block.
		PrimaryKey: &schema.Key{Name: "pk_leads", Columns: names("id")},
		Indexes:    []schema.Index{{Name: "leads_score_idx", Columns: names("score"), Unique: true}},
	}}}}}
	want := `Table crm.leads {
  id int8 [not null]
  score int4

  indexes {
    id [pk, name: 'pk_leads']
    score [unique, type: btree]
  }
}
`
	if got := Print(db); got != want {
		t.Errorf("Print =\n%s\nwant\n%s", got, want)
	}
	back, err := Parse(want)
	if err != nil {
		t.Fatal(err)
	}
	tbl := back.Table(ident.Q("crm", "leads"))
	if tbl == nil || tbl.PrimaryKey == nil || tbl.PrimaryKey.Name != "pk_leads" {
		t.Errorf("primary key name lost: %+v", tbl)
	}
	if len(tbl.Indexes) != 1 || !tbl.Indexes[0].Unique || tbl.Indexes[0].Method != "btree" || len(tbl.Uniques) != 0 {
		t.Errorf("unique index with a type should stay an index: %+v / %+v", tbl.Indexes, tbl.Uniques)
	}
}
