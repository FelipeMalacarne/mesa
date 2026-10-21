package dbml

import (
	"cmp"
	"fmt"
	"slices"
	"strings"

	"github.com/felipemalacarne/mesa/internal/core/ident"
	"github.com/felipemalacarne/mesa/internal/core/schema"
)

// Print renders db as a DBML document: the project, then enums, then tables,
// then the refs that do not fit inline, everything sorted as schema.Normalize
// sorts it so the output is stable. Names that follow the PostgreSQL
// convention are left out; Parse puts them back, so Parse(Print(db)) is db.
// The argument is not modified.
func Print(db *schema.Database) string {
	if db == nil {
		return ""
	}
	p := &printer{}
	if db.Name != "" {
		p.block()
		p.printf("Project %s {\n}\n", word(db.Name))
	}
	schemas := sortedByName(db.Schemas, func(s schema.Schema) ident.Name { return s.Name })
	for _, s := range schemas {
		for _, e := range sortedByName(s.Enums, func(e schema.Enum) ident.Name { return e.Name }) {
			p.enum(s.Name, &e)
		}
	}
	var refs []string
	for _, s := range schemas {
		for _, t := range sortedByName(s.Tables, func(t schema.Table) ident.Name { return t.Name }) {
			refs = append(refs, p.table(s.Name, &t)...)
		}
	}
	if len(refs) > 0 {
		p.block()
		for _, r := range refs {
			p.printf("%s\n", r)
		}
	}
	return p.b.String()
}

type printer struct {
	b      strings.Builder
	blocks int
}

func (p *printer) printf(format string, args ...any) {
	fmt.Fprintf(&p.b, format, args...)
}

// block separates top-level elements with a blank line.
func (p *printer) block() {
	if p.blocks > 0 {
		p.b.WriteByte('\n')
	}
	p.blocks++
}

func (p *printer) enum(schemaName ident.Name, e *schema.Enum) {
	p.block()
	p.printf("Enum %s {\n", qualified(ident.Q(schemaName, e.Name)))
	for _, v := range e.Values {
		p.printf("  %s\n", word(ident.Name(v)))
	}
	p.printf("}\n")
}

// table prints one table and returns the refs to print at the end of the
// document: composite foreign keys, foreign keys with actions and foreign
// keys that could not go inline.
func (p *printer) table(schemaName ident.Name, t *schema.Table) []string {
	q := ident.Q(schemaName, t.Name)
	p.block()
	p.printf("Table %s {\n", qualified(q))

	inlinePK := t.PrimaryKey != nil && len(t.PrimaryKey.Columns) == 1 && conventional(t.PrimaryKey.Name, pkName(t.Name))

	uniques := sortedByName(t.Uniques, func(k schema.Key) ident.Name { return k.Name })
	inlineUnique := map[ident.Name]bool{}
	var blockUniques []schema.Key
	for _, u := range uniques {
		if len(u.Columns) == 1 && conventional(u.Name, keyName(t.Name, u.Columns)) && !inlineUnique[u.Columns[0]] {
			inlineUnique[u.Columns[0]] = true
			continue
		}
		blockUniques = append(blockUniques, u)
	}

	fks := sortedByName(t.ForeignKeys, func(fk schema.ForeignKey) ident.Name { return fk.Name })
	inlineFK := map[ident.Name]*schema.ForeignKey{}
	var refs []string
	for i := range fks {
		fk := &fks[i]
		if len(fk.Columns) == 1 && noAction(fk.OnDelete) && noAction(fk.OnUpdate) &&
			conventional(fk.Name, fkName(t.Name, fk.Columns)) && inlineFK[fk.Columns[0]] == nil {
			inlineFK[fk.Columns[0]] = fk
			continue
		}
		refs = append(refs, ref(q, t, fk))
	}

	for i := range t.Columns {
		c := &t.Columns[i]
		// The pk setting implies not null; a primary key declared in the
		// indexes block does not.
		pk := inlinePK && t.IsPrimaryKey(c.Name)
		var settings []string
		if pk {
			settings = append(settings, "pk")
		}
		if c.Identity != schema.IdentityNone {
			settings = append(settings, "increment")
		}
		if inlineUnique[c.Name] {
			settings = append(settings, "unique")
		}
		switch {
		case !c.Nullable && !pk:
			settings = append(settings, "not null")
		case c.Nullable && pk:
			settings = append(settings, "null")
		}
		if c.Default != nil {
			settings = append(settings, "default: "+defaultValue(*c.Default))
		}
		if c.Comment != "" {
			settings = append(settings, "note: "+str(c.Comment, "  "))
		}
		if fk := inlineFK[c.Name]; fk != nil {
			settings = append(settings, "ref: > "+endpointText(fk.References, fk.RefColumns))
		}
		p.printf("  %s %s", word(c.Name), typeText(c.Type))
		if len(settings) > 0 {
			p.printf(" [%s]", strings.Join(settings, ", "))
		}
		p.b.WriteByte('\n')
	}

	var entries []string
	if t.PrimaryKey != nil && !inlinePK {
		entries = append(entries, indexEntry(t.PrimaryKey.Columns, "", "pk", nameSetting(t.PrimaryKey.Name, pkName(t.Name))))
	}
	for _, u := range blockUniques {
		entries = append(entries, indexEntry(u.Columns, "", "unique", nameSetting(u.Name, keyName(t.Name, u.Columns))))
	}
	for _, ix := range sortedByName(t.Indexes, func(ix schema.Index) ident.Name { return ix.Name }) {
		var settings []string
		if ix.Unique {
			settings = append(settings, "unique")
		}
		expressions := 0
		if ix.Expression != "" {
			expressions = 1
		}
		settings = append(settings, nameSetting(ix.Name, indexName(t.Name, ix.Columns, expressions)))
		method := ix.Method
		if ix.Unique && method == "" {
			// Without a method the entry would read as a unique constraint.
			method = "btree"
		}
		if method != "" {
			settings = append(settings, "type: "+word(ident.Name(method)))
		}
		entries = append(entries, indexEntry(ix.Columns, ix.Expression, settings...))
	}
	if len(entries) > 0 {
		p.printf("\n  indexes {\n")
		for _, e := range entries {
			p.printf("    %s\n", e)
		}
		p.printf("  }\n")
	}

	if t.Comment != "" {
		p.printf("\n  Note: %s\n", str(t.Comment, "  "))
	}
	p.printf("}\n")
	return refs
}

// conventional reports whether a name can be left out: it is empty or it is
// the one Parse would generate.
func conventional(name, generated ident.Name) bool {
	return name == "" || name == generated
}

// nameSetting is "name: '…'" when the name must be written, else "".
func nameSetting(name, generated ident.Name) string {
	if conventional(name, generated) {
		return ""
	}
	return "name: " + singleLine(name.String())
}

func indexEntry(cols []ident.Name, expression string, settings ...string) string {
	var parts []string
	for _, c := range cols {
		parts = append(parts, word(c))
	}
	if expression != "" {
		parts = append(parts, "`"+expression+"`")
	}
	entry := strings.Join(parts, ", ")
	if len(parts) != 1 {
		entry = "(" + entry + ")"
	}
	var kept []string
	for _, s := range settings {
		if s != "" {
			kept = append(kept, s)
		}
	}
	if len(kept) > 0 {
		entry += " [" + strings.Join(kept, ", ") + "]"
	}
	return entry
}

func ref(q ident.Qualified, t *schema.Table, fk *schema.ForeignKey) string {
	var b strings.Builder
	b.WriteString("Ref")
	if !conventional(fk.Name, fkName(t.Name, fk.Columns)) {
		b.WriteByte(' ')
		b.WriteString(word(fk.Name))
	}
	b.WriteString(": ")
	b.WriteString(endpointText(q, fk.Columns))
	b.WriteString(" > ")
	b.WriteString(endpointText(fk.References, fk.RefColumns))
	var settings []string
	if !noAction(fk.OnDelete) {
		settings = append(settings, "delete: "+strings.ToLower(fk.OnDelete))
	}
	if !noAction(fk.OnUpdate) {
		settings = append(settings, "update: "+strings.ToLower(fk.OnUpdate))
	}
	if len(settings) > 0 {
		b.WriteString(" [" + strings.Join(settings, ", ") + "]")
	}
	return b.String()
}

func endpointText(q ident.Qualified, cols []ident.Name) string {
	if len(cols) == 1 {
		return qualified(q) + "." + word(cols[0])
	}
	parts := make([]string, len(cols))
	for i, c := range cols {
		parts[i] = word(c)
	}
	return qualified(q) + ".(" + strings.Join(parts, ", ") + ")"
}

// qualified renders a table or enum name, with its schema unless it is the
// default one.
func qualified(q ident.Qualified) string {
	if q.Schema == defaultSchema || q.Schema == "" {
		return word(q.Name)
	}
	return word(q.Schema) + "." + word(q.Name)
}

// reserved are the words that cannot stand bare where a name is expected.
var reserved = map[string]bool{
	"table": true, "ref": true, "enum": true, "project": true, "tablegroup": true,
	"indexes": true, "note": true, "as": true,
}

// word renders a name bare when DBML allows it and double-quoted otherwise.
func word(n ident.Name) string {
	s := n.String()
	if isBareWord(s) && !reserved[strings.ToLower(s)] {
		return s
	}
	return quoted(s)
}

func isBareWord(s string) bool {
	if s == "" {
		return false
	}
	for i, r := range s {
		switch {
		case r == '_', r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z':
		case r >= '0' && r <= '9':
			if i == 0 {
				return false
			}
		default:
			return false
		}
	}
	return true
}

// typeText renders a column type bare when the parser would read it back
// unchanged (int8, varchar(120), numeric(10,2), billing.currency, text[])
// and double-quoted otherwise ("timestamp with time zone").
func typeText(t string) string {
	base := t
	for strings.HasSuffix(base, "[]") {
		base = strings.TrimSuffix(base, "[]")
	}
	if base == "" || !isBareWord(strings.Map(func(r rune) rune {
		if r == '.' || r == '(' || r == ')' || r == ',' {
			return -1
		}
		return r
	}, base)) || strings.Count(base, "(") != strings.Count(base, ")") || strings.Contains(base, "[") {
		return quoted(t)
	}
	return t
}

// defaultValue renders a SQL default expression as a DBML default: numbers,
// booleans and null bare, plain string literals quoted, anything else as a
// backtick expression.
func defaultValue(sql string) string {
	if isNumber(sql) {
		return sql
	}
	switch l := strings.ToLower(sql); l {
	case "true", "false", "null":
		return l
	}
	if s, ok := sqlStringLiteral(sql); ok {
		return singleLine(s)
	}
	return "`" + sql + "`"
}

func isNumber(s string) bool {
	s = strings.TrimPrefix(s, "-")
	if s == "" {
		return false
	}
	dot := false
	for i, r := range s {
		switch {
		case r >= '0' && r <= '9':
		case r == '.' && !dot && i > 0 && i < len(s)-1:
			dot = true
		default:
			return false
		}
	}
	return true
}

// sqlStringLiteral decodes '…' with doubled quotes inside; anything else
// (a cast, a function) is not a plain literal.
func sqlStringLiteral(sql string) (string, bool) {
	if len(sql) < 2 || sql[0] != '\'' || sql[len(sql)-1] != '\'' {
		return "", false
	}
	inner := sql[1 : len(sql)-1]
	var b strings.Builder
	for i := 0; i < len(inner); i++ {
		if inner[i] == '\'' {
			if i+1 >= len(inner) || inner[i+1] != '\'' {
				return "", false
			}
			i++
		}
		b.WriteByte(inner[i])
	}
	return b.String(), true
}

// str renders a string value: single-quoted on one line, or as a triple-quoted
// block, indented past indent, when it spans lines and the block form reads
// back exactly.
func str(s, indent string) string {
	if strings.Contains(s, "\n") && blockSafe(s) {
		var b strings.Builder
		b.WriteString("'''\n")
		for _, line := range strings.Split(s, "\n") {
			if line != "" {
				b.WriteString(indent + "  " + line)
			}
			b.WriteByte('\n')
		}
		b.WriteString(indent + "'''")
		return b.String()
	}
	return singleLine(s)
}

// singleLine renders a string value on one line, escaping newlines.
func singleLine(s string) string { return "'" + escape(s, '\'') + "'" }

// blockSafe reports whether the triple-quoted form preserves s: no triple quote inside, no
// carriage returns, and a line that starts at column zero so the shared
// indentation the parser strips is exactly the one Print adds.
func blockSafe(s string) bool {
	if strings.Contains(s, "'''") || strings.Contains(s, "\r") {
		return false
	}
	for _, line := range strings.Split(s, "\n") {
		if strings.TrimSpace(line) != "" && line[0] != ' ' && line[0] != '\t' {
			return true
		}
	}
	return false
}

func quoted(s string) string { return `"` + escape(s, '"') + `"` }

func escape(s string, quote byte) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		switch c := s[i]; c {
		case '\\':
			b.WriteString(`\\`)
		case quote:
			b.WriteByte('\\')
			b.WriteByte(c)
		case '\n':
			b.WriteString(`\n`)
		case '\r':
			b.WriteString(`\r`)
		default:
			b.WriteByte(c)
		}
	}
	return b.String()
}

// sortedByName returns a copy of items sorted by name, keeping the order of
// equal names.
func sortedByName[T any](items []T, name func(T) ident.Name) []T {
	out := slices.Clone(items)
	slices.SortStableFunc(out, func(a, b T) int { return cmp.Compare(name(a), name(b)) })
	return out
}
