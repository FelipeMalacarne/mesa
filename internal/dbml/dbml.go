// Package dbml reads and writes DBML (https://dbml.dbdiagram.io), the text
// form of the schema model. Parse turns a document into a schema.Database and
// Print renders a database as a normalized document; the two round-trip, so
// the designer can edit either the diagram or the text.
//
// The engine's type names are kept verbatim, as the model does. Constraints
// the document does not name get the names PostgreSQL would choose
// (customers_pkey, customers_email_key, orders_customer_id_fkey,
// orders_customer_id_idx) and Print omits names that follow that convention,
// so a hand-written model matches what a live catalog reports.
//
// DBML has no words for check constraints, views, functions, sequences,
// partial-index predicates, identity ALWAYS or notes on enum values; Print
// drops them (identity ALWAYS prints as increment).
package dbml

import (
	"strings"
	"unicode/utf8"

	"github.com/felipemalacarne/mesa/internal/core/ident"
)

// defaultSchema is where unqualified tables and enums live.
const defaultSchema ident.Name = "public"

// Conventional object names, as PostgreSQL generates them.

func pkName(table ident.Name) ident.Name { return objectName(table, nil, "pkey") }

func keyName(table ident.Name, cols []ident.Name) ident.Name {
	return objectName(table, cols, "key")
}

func fkName(table ident.Name, cols []ident.Name) ident.Name {
	return objectName(table, cols, "fkey")
}

// indexName is the conventional name of an index; each expression counts as
// "expr", as in PostgreSQL.
func indexName(table ident.Name, cols []ident.Name, expressions int) ident.Name {
	parts := make([]ident.Name, 0, len(cols)+expressions)
	parts = append(parts, cols...)
	for range expressions {
		parts = append(parts, "expr")
	}
	return objectName(table, parts, "idx")
}

func objectName(table ident.Name, parts []ident.Name, suffix string) ident.Name {
	var b strings.Builder
	b.WriteString(string(table))
	for _, p := range parts {
		b.WriteByte('_')
		b.WriteString(string(p))
	}
	b.WriteByte('_')
	b.WriteString(suffix)
	return ident.Name(truncateName(b.String(), ident.MaxLen))
}

// truncateName cuts s to at most n bytes on a rune boundary.
func truncateName(s string, n int) string {
	if len(s) <= n {
		return s
	}
	for n > 0 && !utf8.RuneStart(s[n]) {
		n--
	}
	return s[:n]
}

func sameNames(a, b []ident.Name) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// noAction reports whether a referential action means NO ACTION, the default.
func noAction(a string) bool {
	return a == "" || strings.EqualFold(a, "NO ACTION")
}
