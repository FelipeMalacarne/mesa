package dbml

import (
	"strings"
	"testing"

	"github.com/felipemalacarne/mesa/internal/core/ident"
	"github.com/felipemalacarne/mesa/internal/core/schema"
)

func TestValidate(t *testing.T) {
	db := &schema.Database{Schemas: []schema.Schema{{Name: "public", Tables: []schema.Table{
		{
			Name:       "orders",
			Columns:    []schema.Column{{Name: "id", Type: "int8"}, {Name: "id", Type: "int8"}, {Name: "untyped"}},
			PrimaryKey: &schema.Key{Columns: names("id")},
			Uniques:    []schema.Key{{Columns: names("ghost")}},
			Indexes:    []schema.Index{{Name: "", Columns: names("nope")}},
			ForeignKeys: []schema.ForeignKey{
				{Name: "fk_customer", Columns: names("customer_id"), References: ident.Q("public", "customers"), RefColumns: names("id")},
				{Columns: names("id"), References: ident.Q("public", "plans"), RefColumns: names("id", "code")},
				{Columns: names("id"), References: ident.Q("public", "plans"), RefColumns: names("missing")},
			},
		},
		{Name: "plans", Columns: []schema.Column{{Name: "id", Type: "text"}}},
		{Name: "plans", Columns: []schema.Column{{Name: "id", Type: "text"}}},
		{Name: "empty"},
	}}}}
	got := Validate(db)
	want := []string{
		"error: column id of table public.orders is defined twice",
		"error: column untyped of table public.orders has no type",
		"error: unique constraint (ghost) of table public.orders uses unknown column ghost",
		"error: an index of table public.orders has no name",
		"error: index  of table public.orders uses unknown column nope",
		"error: foreign key fk_customer of table public.orders uses unknown column customer_id",
		"error: foreign key fk_customer of table public.orders references unknown table public.customers",
		"error: foreign key (id) of table public.orders joins 1 columns to 2",
		"error: foreign key (id) of table public.orders references unknown column code of table public.plans",
		"error: foreign key (id) of table public.orders references unknown column missing of table public.plans",
		"warning: table public.plans has no primary key",
		"error: table public.plans is defined twice",
		"warning: table public.empty has no columns",
		"warning: table public.empty has no primary key",
	}
	var lines []string
	for _, p := range got {
		if p.Line != 0 {
			t.Errorf("Validate on a hand-built model should not know lines: %+v", p)
		}
		lines = append(lines, p.Level+": "+p.Message)
	}
	if strings.Join(lines, "\n") != strings.Join(want, "\n") {
		t.Errorf("Validate =\n%s\nwant\n%s", strings.Join(lines, "\n"), strings.Join(want, "\n"))
	}
}

func TestCheckReportsLines(t *testing.T) {
	src := `Table customers {
  id int8 [pk]
}

Table orders {
  id int8 [pk]
  customer_id int8 [ref: > customers.customer_id]
  customer_id int8
}

Table notes {
  body text
}

Ref: orders.id > invoices.id
`
	got := Check(src)
	type want struct {
		line  int
		level string
		msg   string
	}
	wants := []want{
		{8, LevelError, "column customer_id of table public.orders is defined twice"},
		{7, LevelError, "foreign key orders_customer_id_fkey of table public.orders references unknown column customer_id of table public.customers"},
		{15, LevelError, "foreign key orders_id_fkey of table public.orders references unknown table public.invoices"},
		{11, LevelWarning, "table public.notes has no primary key"},
	}
	if len(got) != len(wants) {
		t.Fatalf("Check = %+v, want %d problems", got, len(wants))
	}
	for i, w := range wants {
		p := got[i]
		if p.Line != w.line || p.Level != w.level || p.Message != w.msg {
			t.Errorf("problem %d = %+v, want %+v", i, p, w)
		}
	}

	syntax := Check("Table t {\n  id int [pk\n}")
	if len(syntax) != 1 || syntax[0].Line != 3 || syntax[0].Level != LevelError || !strings.Contains(syntax[0].Message, "expected") {
		t.Errorf("Check on a syntax error = %+v", syntax)
	}
	if problems := Check(""); len(problems) != 0 {
		t.Errorf("Check(\"\") = %+v, want none", problems)
	}
}
