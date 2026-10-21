package designer

import (
	"testing"

	"github.com/felipemalacarne/mesa/internal/core/ident"
	"github.com/felipemalacarne/mesa/internal/core/schema"
)

func TestFoldMergesSchemasIntoTheDatabase(t *testing.T) {
	m := &schema.Database{Schemas: []schema.Schema{
		{Name: "public", Tables: []schema.Table{{Name: "coupons", ForeignKeys: []schema.ForeignKey{{Columns: []ident.Name{"customer_id"}, References: ident.Q("public", "customers"), RefColumns: []ident.Name{"id"}}}}}},
		{Name: "shop", Tables: []schema.Table{{Name: "customers"}}},
	}}
	Fold(m, "shop")
	if len(m.Schemas) != 1 || m.Schemas[0].Name != "shop" || len(m.Schemas[0].Tables) != 2 {
		t.Fatalf("folded = %+v", m.Schemas)
	}
	if ref := m.Table(ident.Q("shop", "coupons")).ForeignKeys[0].References; ref != ident.Q("shop", "customers") {
		t.Errorf("reference = %v, want shop.customers", ref)
	}
}
