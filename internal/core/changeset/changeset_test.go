package changeset

import (
	"testing"

	"github.com/felipemalacarne/mesa/internal/core/ident"
	"github.com/felipemalacarne/mesa/internal/core/value"
)

func TestDisplayInlinesArgs(t *testing.T) {
	s := Statement{SQL: `UPDATE "public"."customers" SET "plan" = $1, "mrr_cents" = $2 WHERE "id" = $3`, Args: []value.Value{value.Text("te'am"), value.Int(19900), value.Int(1042)}}
	got := s.Display()
	want := `UPDATE "public"."customers" SET "plan" = 'te''am', "mrr_cents" = 19900 WHERE "id" = 1042`
	if got != want {
		t.Errorf("got %s\nwant %s", got, want)
	}
	if Literal(value.Null()) != "NULL" || Literal(value.Bool(true)) != "true" {
		t.Error("literals")
	}
}

func TestValidate(t *testing.T) {
	cs := ChangeSet{Rows: []RowChange{{Kind: Update, Table: ident.Q("public", "t"), Values: map[ident.Name]value.Value{"a": value.Int(1)}}}}
	if err := cs.Validate(); err == nil {
		t.Error("update without key accepted")
	}
	cs.Rows[0].Key = map[ident.Name]value.Value{"id": value.Int(1)}
	if err := cs.Validate(); err != nil {
		t.Error(err)
	}
	if s := cs.Summary(); s.Updated != 1 {
		t.Errorf("summary %+v", s)
	}
}

func TestPlanWorst(t *testing.T) {
	p := Plan{Risks: []Risk{{Level: Warning}, {Level: Danger}}}
	if p.Worst() != Danger {
		t.Error("worst")
	}
	if (&Plan{}).Worst() != "" {
		t.Error("no risks")
	}
	p = Plan{Statements: []Statement{{SQL: "CREATE TABLE x (id int)"}, {SQL: "DROP TABLE y;"}}}
	if p.SQL() != "CREATE TABLE x (id int);\nDROP TABLE y;\n" {
		t.Errorf("script: %q", p.SQL())
	}
}
