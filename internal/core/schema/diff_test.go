package schema

import (
	"testing"

	"github.com/felipemalacarne/mesa/internal/core/ident"
)

func str(s string) *string { return &s }

func billing() *Database {
	return &Database{Name: "billing", Schemas: []Schema{{Name: "public", Tables: []Table{
		{Name: "customers", Columns: []Column{
			{Name: "id", Type: "int8", Identity: IdentityByDefault},
			{Name: "email", Type: "text"},
			{Name: "plan", Type: "text", Nullable: true, Default: str("'free'")},
		}, PrimaryKey: &Key{Name: "customers_pkey", Columns: []ident.Name{"id"}},
			Uniques: []Key{{Name: "customers_email_key", Columns: []ident.Name{"email"}}}},
		{Name: "subscriptions", Columns: []Column{
			{Name: "id", Type: "int8"}, {Name: "customer_id", Type: "int8"},
		}, PrimaryKey: &Key{Columns: []ident.Name{"id"}},
			ForeignKeys: []ForeignKey{{Name: "subscriptions_customer_id_fkey", Columns: []ident.Name{"customer_id"}, References: ident.Q("public", "customers"), RefColumns: []ident.Name{"id"}}}},
	}}}}
}

func kinds(cs []Change) []ChangeKind {
	out := make([]ChangeKind, len(cs))
	for i, c := range cs {
		out[i] = c.Kind
	}
	return out
}

func TestDiffNoChange(t *testing.T) {
	if cs := Diff(billing(), billing()); len(cs) != 0 {
		t.Fatalf("expected no changes, got %v", kinds(cs))
	}
}

func TestDiffAddTableAndColumn(t *testing.T) {
	from, to := billing(), billing()
	pub := to.Schema("public")
	pub.Tables = append(pub.Tables, Table{Name: "coupons", Columns: []Column{{Name: "id", Type: "int8"}, {Name: "code", Type: "text"}}, PrimaryKey: &Key{Columns: []ident.Name{"id"}}})
	sub := pub.Table("subscriptions")
	sub.Columns = append(sub.Columns, Column{Name: "coupon_id", Type: "int8", Nullable: true})
	sub.ForeignKeys = append(sub.ForeignKeys, ForeignKey{Columns: []ident.Name{"coupon_id"}, References: ident.Q("public", "coupons"), RefColumns: []ident.Name{"id"}})
	cs := Diff(from, to)
	want := []ChangeKind{AddColumn, CreateTable, AddForeignKey}
	got := kinds(cs)
	if len(got) != len(want) {
		t.Fatalf("got %v want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %v want %v", got, want)
		}
	}
	if cs[2].ForeignKey.References.Name != "coupons" {
		t.Errorf("fk target: %+v", cs[2].ForeignKey)
	}
}

func TestDiffAlterColumn(t *testing.T) {
	from, to := billing(), billing()
	plan := to.Schema("public").Table("customers").Column("plan")
	plan.Nullable = false
	plan.Type = "varchar(16)"
	plan.Default = nil
	plan.Comment = "free · pro · team"
	got := kinds(Diff(from, to))
	want := []ChangeKind{AlterColumnType, SetNotNull, DropDefault, SetColumnComment}
	if len(got) != len(want) {
		t.Fatalf("got %v want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %v want %v", got, want)
		}
	}
}

func TestDiffDropTableDropsForeignKeysFirst(t *testing.T) {
	from, to := billing(), billing()
	pub := to.Schema("public")
	pub.Tables = pub.Tables[:1] // drop subscriptions
	got := kinds(Diff(from, to))
	if len(got) != 2 || got[0] != DropForeignKey || got[1] != DropTable {
		t.Fatalf("got %v", got)
	}
}

func TestDiffEnums(t *testing.T) {
	from, to := billing(), billing()
	from.Schema("public").Enums = []Enum{{Name: "status", Values: []string{"a"}}}
	to.Schema("public").Enums = []Enum{{Name: "status", Values: []string{"a", "b"}}, {Name: "kind", Values: []string{"x"}}}
	got := kinds(Diff(from, to))
	if len(got) != 2 || got[0] != AddEnumValue || got[1] != CreateEnum {
		t.Fatalf("got %v", got)
	}
}
