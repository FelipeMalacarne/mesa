package ident

import "testing"

func TestParse(t *testing.T) {
	for _, ok := range []string{"customers", "Customers", "créditos", "a b", "x"} {
		if _, err := Parse(ok); err != nil {
			t.Errorf("Parse(%q) = %v, want ok", ok, err)
		}
	}
	for _, bad := range []string{"", "a\x00b", "tab\tname", string(make([]byte, 64))} {
		if _, err := Parse(bad); err == nil {
			t.Errorf("Parse(%q) = ok, want error", bad)
		}
	}
}

func TestQuote(t *testing.T) {
	cases := map[Name]string{
		"customers": `"customers"`,
		`say "hi"`:  `"say ""hi"""`,
		"Mixed":     `"Mixed"`,
	}
	for in, want := range cases {
		if got := in.Quote(); got != want {
			t.Errorf("%q.Quote() = %s, want %s", in, got, want)
		}
	}
	if !Name("created_at").IsSimple() || Name("1abc").IsSimple() || Name("Mixed").IsSimple() {
		t.Error("IsSimple rules broken")
	}
}

func TestQualified(t *testing.T) {
	q, err := ParseQualified("public.customers", "public")
	if err != nil || q.String() != "public.customers" || q.Quote() != `"public"."customers"` {
		t.Fatalf("ParseQualified: %v %+v", err, q)
	}
	q, err = ParseQualified("invoices", "billing")
	if err != nil || q.Schema != "billing" {
		t.Fatalf("default schema not applied: %v %+v", err, q)
	}
}
