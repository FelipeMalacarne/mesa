package dbml

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/felipemalacarne/mesa/internal/core/errs"
	"github.com/felipemalacarne/mesa/internal/core/ident"
	"github.com/felipemalacarne/mesa/internal/core/schema"
)

// Parse reads a DBML document into a database. Tables and enums without a
// schema go to "public". Objects stay in document order; call Normalize or
// Print to sort them. Errors are errs.Invalid and name the line and column.
func Parse(src string) (*schema.Database, error) {
	db, _, serr := parse(src)
	if serr != nil {
		return nil, errs.New(errs.Invalid, serr.Error())
	}
	return db, nil
}

// positions remembers the line where each object was declared, keyed by the
// strings posTable, posColumn and friends build, so Check can report lines.
type positions map[string]int

func posTable(q ident.Qualified) string { return "table " + q.String() }

func posColumn(q ident.Qualified, col ident.Name) string {
	return "column " + q.String() + "." + col.String()
}

func posPrimaryKey(q ident.Qualified) string { return "pk " + q.String() }

func posUnique(q ident.Qualified, cols []ident.Name) string {
	return "unique " + q.String() + " " + joinNames(cols)
}

func posIndex(q ident.Qualified, name ident.Name) string {
	return "index " + q.String() + " " + name.String()
}

func posForeignKey(q ident.Qualified, fk *schema.ForeignKey) string {
	return "fk " + q.String() + " " + joinNames(fk.Columns) + " > " + fk.References.String() + " " + joinNames(fk.RefColumns)
}

func joinNames(names []ident.Name) string {
	parts := make([]string, len(names))
	for i, n := range names {
		parts[i] = n.String()
	}
	return strings.Join(parts, ",")
}

// endpoint is one side of a ref as written: a table path (alias, table or
// schema.table) and its columns. Inline refs pre-resolve their own side.
type endpoint struct {
	tok      token
	path     []ident.Name
	table    ident.Qualified
	resolved bool
	cols     []ident.Name
}

// pendingRef is a ref waiting for every table to be known before it becomes
// a foreign key.
type pendingRef struct {
	tok                token
	name               ident.Name
	left, right        endpoint
	rel                string // ">", "<", "-" or "<>"
	onDelete, onUpdate string
}

type parser struct {
	src     string
	toks    []token
	pos     int
	db      *schema.Database
	aliases map[ident.Name]ident.Qualified
	refs    []pendingRef
	lines   positions
}

func parse(src string) (db *schema.Database, lines positions, serr *syntaxError) {
	toks, lerr := lex(src)
	if lerr != nil {
		return nil, nil, lerr
	}
	p := &parser{
		src:     src,
		toks:    toks,
		db:      &schema.Database{},
		aliases: map[ident.Name]ident.Qualified{},
		lines:   positions{},
	}
	defer func() {
		if r := recover(); r != nil {
			e, ok := r.(*syntaxError)
			if !ok {
				panic(r)
			}
			db, lines, serr = nil, nil, e
		}
	}()
	p.document()
	p.resolveRefs()
	return p.db, p.lines, nil
}

// Token access.

func (p *parser) peek() token { return p.toks[p.pos] }

func (p *parser) peekN(n int) token {
	if p.pos+n < len(p.toks) {
		return p.toks[p.pos+n]
	}
	return p.toks[len(p.toks)-1]
}

func (p *parser) next() token {
	t := p.toks[p.pos]
	if t.kind != tokEOF {
		p.pos++
	}
	return t
}

func (p *parser) fail(t token, format string, args ...any) {
	panic(&syntaxError{line: t.line, col: t.col, msg: fmt.Sprintf(format, args...)})
}

func (p *parser) expectSymbol(s string) token {
	t := p.next()
	if !t.isSymbol(s) {
		p.fail(t, "expected '%s', found %s", s, t.describe())
	}
	return t
}

func (p *parser) expectWord(w string) token {
	t := p.next()
	if !t.isWord(w) {
		p.fail(t, "expected '%s', found %s", w, t.describe())
	}
	return t
}

// name reads an identifier: a bare word or a quoted string.
func (p *parser) name(what string) (ident.Name, token) {
	t := p.next()
	if t.kind != tokWord && t.kind != tokString {
		p.fail(t, "expected %s, found %s", what, t.describe())
	}
	n, err := ident.Parse(t.text)
	if err != nil {
		p.fail(t, "%s: %s", what, errs.Message(err))
	}
	return n, t
}

// qualifiedName reads name or schema.name.
func (p *parser) qualifiedName(what string) (ident.Qualified, token) {
	first, tok := p.name(what)
	if p.peek().isSymbol(".") {
		p.next()
		second, _ := p.name(what)
		return ident.Q(first, second), tok
	}
	return ident.Q(defaultSchema, first), tok
}

func (p *parser) stringValue(what string) string {
	t := p.next()
	if t.kind != tokString {
		p.fail(t, "expected %s, found %s", what, t.describe())
	}
	return t.text
}

// skipBalanced consumes a block from its opening symbol to the matching
// closing one.
func (p *parser) skipBalanced(open, closing string) {
	start := p.expectSymbol(open)
	depth := 1
	for depth > 0 {
		t := p.next()
		switch {
		case t.kind == tokEOF:
			p.fail(start, "expected '%s' to close '%s'", closing, open)
		case t.isSymbol(open):
			depth++
		case t.isSymbol(closing):
			depth--
		}
	}
}

// skipBlock skips an element Mesa does not model: its header up to the
// opening brace and the block itself.
func (p *parser) skipBlock() {
	kw := p.next()
	for !p.peek().isSymbol("{") {
		if p.peek().kind == tokEOF {
			p.fail(kw, "expected '{' after %s", kw.describe())
		}
		p.next()
	}
	p.skipBalanced("{", "}")
}

// Document structure.

func (p *parser) document() {
	for {
		t := p.peek()
		if t.kind == tokEOF {
			return
		}
		if t.kind != tokWord {
			p.fail(t, "expected Table, Enum, Ref, Project or TableGroup, found %s", t.describe())
		}
		switch strings.ToLower(t.text) {
		case "project":
			p.project()
		case "table":
			p.table()
		case "enum":
			p.enum()
		case "ref":
			p.ref()
		case "tablegroup", "note":
			p.skipBlock()
		default:
			p.fail(t, "expected Table, Enum, Ref, Project or TableGroup, found %s", t.describe())
		}
	}
}

func (p *parser) project() {
	p.next()
	if t := p.peek(); t.kind == tokWord || t.kind == tokString {
		name, _ := p.name("project name")
		p.db.Name = name
	}
	p.skipBalanced("{", "}")
}

func (p *parser) schema(name ident.Name) *schema.Schema {
	if s := p.db.Schema(name); s != nil {
		return s
	}
	p.db.Schemas = append(p.db.Schemas, schema.Schema{Name: name})
	return &p.db.Schemas[len(p.db.Schemas)-1]
}

func (p *parser) enum() {
	p.next()
	q, tok := p.qualifiedName("enum name")
	p.expectSymbol("{")
	e := schema.Enum{Name: q.Name}
	for !p.peek().isSymbol("}") {
		t := p.next()
		switch t.kind {
		case tokWord, tokString, tokNumber:
			e.Values = append(e.Values, t.text)
		default:
			p.fail(t, "expected an enum value, found %s", t.describe())
		}
		if p.peek().isSymbol("[") {
			p.skipBalanced("[", "]") // notes on values are not modelled
		}
	}
	p.expectSymbol("}")
	s := p.schema(q.Schema)
	if s.Enum(q.Name) != nil {
		p.fail(tok, "enum %s is defined twice", q)
	}
	s.Enums = append(s.Enums, e)
}

// tableScope is what the column and index parsers need to know about the
// table they are filling.
type tableScope struct {
	q      ident.Qualified
	t      *schema.Table
	pkCols []ident.Name // columns marked pk
}

func (p *parser) table() {
	kw := p.next()
	q, tok := p.qualifiedName("table name")
	if p.db.Table(q) != nil {
		p.fail(tok, "table %s is defined twice", q)
	}
	if p.peek().isWord("as") {
		p.next()
		alias, atok := p.name("table alias")
		if _, dup := p.aliases[alias]; dup {
			p.fail(atok, "alias %s is used twice", alias)
		}
		p.aliases[alias] = q
	}
	ts := &tableScope{q: q, t: &schema.Table{Name: q.Name}}
	if p.peek().isSymbol("[") {
		p.tableSettings(ts)
	}
	p.expectSymbol("{")
	for !p.peek().isSymbol("}") {
		t := p.peek()
		switch {
		case t.kind == tokEOF:
			p.fail(kw, "expected '}' to close table %s", q)
		case t.isWord("indexes") && p.peekN(1).isSymbol("{"):
			p.indexes(ts)
		case t.isWord("note") && (p.peekN(1).isSymbol(":") || p.peekN(1).isSymbol("{")):
			ts.t.Comment = p.note()
		default:
			p.column(ts)
		}
	}
	p.expectSymbol("}")
	if len(ts.pkCols) > 0 {
		if ts.t.PrimaryKey != nil {
			p.fail(tok, "table %s has more than one primary key", q)
		}
		ts.t.PrimaryKey = &schema.Key{Name: pkName(q.Name), Columns: ts.pkCols}
	}
	if ts.t.PrimaryKey != nil {
		p.lines[posPrimaryKey(q)] = tok.line
	}
	p.lines[posTable(q)] = tok.line
	s := p.schema(q.Schema)
	s.Tables = append(s.Tables, *ts.t)
}

func (p *parser) tableSettings(ts *tableScope) {
	p.expectSymbol("[")
	for {
		key := p.next()
		if key.kind != tokWord {
			p.fail(key, "expected a table setting, found %s", key.describe())
		}
		switch strings.ToLower(key.text) {
		case "note":
			p.expectSymbol(":")
			ts.t.Comment = p.stringValue("a note")
		default:
			// headercolor and other diagram-only settings.
			p.expectSymbol(":")
			for !p.peek().isSymbol(",") && !p.peek().isSymbol("]") {
				if p.peek().kind == tokEOF {
					p.fail(key, "expected ']'")
				}
				p.next()
			}
		}
		if p.peek().isSymbol(",") {
			p.next()
			continue
		}
		p.expectSymbol("]")
		return
	}
}

// note reads "Note: '…'" or "Note { '…' }".
func (p *parser) note() string {
	p.next()
	if p.peek().isSymbol("{") {
		p.next()
		s := p.stringValue("a note")
		p.expectSymbol("}")
		return s
	}
	p.expectSymbol(":")
	return p.stringValue("a note")
}

func (p *parser) column(ts *tableScope) {
	name, tok := p.name("a column name")
	col := schema.Column{Name: name, Type: p.columnType(name), Nullable: true}
	if p.peek().isSymbol("[") {
		p.columnSettings(ts, &col)
	}
	p.lines[posColumn(ts.q, name)] = tok.line
	ts.t.Columns = append(ts.t.Columns, col)
}

// columnType reads a type such as int8, varchar(120), numeric(10, 2),
// billing.currency or text[]. A quoted type ("timestamp with time zone") is
// taken verbatim; a bare one loses the whitespace inside its parentheses.
func (p *parser) columnType(col ident.Name) string {
	t := p.next()
	switch t.kind {
	case tokString:
		return t.text
	case tokWord:
	default:
		p.fail(t, "expected a type for column %s, found %s", col, t.describe())
	}
	end := t.end
	for p.peek().isSymbol(".") && p.peek().adjacent && p.peekN(1).kind == tokWord && p.peekN(1).adjacent {
		p.next()
		end = p.next().end
	}
	if p.peek().isSymbol("(") && p.peek().adjacent {
		open := p.next()
		depth := 1
		for depth > 0 {
			c := p.next()
			switch {
			case c.kind == tokEOF:
				p.fail(open, "expected ')' in the type of column %s", col)
			case c.isSymbol("("):
				depth++
			case c.isSymbol(")"):
				depth--
			}
			end = c.end
		}
	}
	for p.peek().isSymbol("[") && p.peek().adjacent && p.peekN(1).isSymbol("]") && p.peekN(1).adjacent {
		p.next()
		end = p.next().end
	}
	return compactType(p.src[t.start:end])
}

// compactType removes whitespace inside parentheses, so numeric(10, 2) is
// spelled as the catalog spells it: numeric(10,2).
func compactType(s string) string {
	var b strings.Builder
	depth := 0
	for _, r := range s {
		switch r {
		case '(':
			depth++
		case ')':
			depth--
		case ' ', '\t', '\n', '\r':
			if depth > 0 {
				continue
			}
		}
		b.WriteRune(r)
	}
	return b.String()
}

func (p *parser) columnSettings(ts *tableScope, col *schema.Column) {
	p.expectSymbol("[")
	var pk, nullable, nullSet bool
	for {
		key := p.next()
		if key.kind != tokWord {
			p.fail(key, "expected a column setting, found %s", key.describe())
		}
		switch strings.ToLower(key.text) {
		case "pk":
			pk = true
		case "primary":
			p.expectWord("key")
			pk = true
		case "unique":
			cols := []ident.Name{col.Name}
			ts.t.Uniques = append(ts.t.Uniques, schema.Key{Name: keyName(ts.q.Name, cols), Columns: cols})
			p.lines[posUnique(ts.q, cols)] = key.line
		case "not":
			p.expectWord("null")
			nullable, nullSet = false, true
		case "null":
			nullable, nullSet = true, true
		case "increment":
			col.Identity = schema.IdentityByDefault
		case "default":
			p.expectSymbol(":")
			v := p.defaultValue(col.Name)
			col.Default = &v
		case "note":
			p.expectSymbol(":")
			col.Comment = p.stringValue("a note")
		case "ref":
			p.expectSymbol(":")
			rel := p.relation()
			right := p.endpoint()
			p.refs = append(p.refs, pendingRef{
				tok:   key,
				left:  endpoint{tok: key, table: ts.q, resolved: true, cols: []ident.Name{col.Name}},
				right: right,
				rel:   rel,
			})
		default:
			p.fail(key, "unknown column setting '%s'", key.text)
		}
		if p.peek().isSymbol(",") {
			p.next()
			continue
		}
		p.expectSymbol("]")
		break
	}
	switch {
	case nullSet:
		col.Nullable = nullable
	case pk:
		col.Nullable = false
	}
	if pk {
		ts.pkCols = append(ts.pkCols, col.Name)
	}
}

// defaultValue reads a default and returns it as a SQL expression: strings
// become quoted literals, numbers, booleans and null stay bare, and backtick
// expressions are taken as written.
func (p *parser) defaultValue(col ident.Name) string {
	t := p.next()
	switch t.kind {
	case tokString:
		return "'" + strings.ReplaceAll(t.text, "'", "''") + "'"
	case tokNumber:
		return t.text
	case tokExpr:
		return t.text
	case tokSymbol:
		if t.text == "-" && p.peek().kind == tokNumber {
			return "-" + p.next().text
		}
	case tokWord:
		switch l := strings.ToLower(t.text); l {
		case "true", "false", "null":
			return l
		}
	}
	p.fail(t, "unsupported default for column %s: %s (quote strings, put expressions in backticks)", col, t.describe())
	return ""
}

func (p *parser) relation() string {
	t := p.next()
	if t.kind == tokSymbol {
		switch t.text {
		case ">", "<", "-", "<>":
			return t.text
		}
	}
	p.fail(t, "expected '>', '<' or '-', found %s", t.describe())
	return ""
}

// endpoint reads table.column, schema.table.column, alias.column or
// table.(col1, col2).
func (p *parser) endpoint() endpoint {
	ep := endpoint{tok: p.peek()}
	for {
		if p.peek().isSymbol("(") {
			ep.cols = p.columnList()
			break
		}
		part, _ := p.name("a table or column name")
		ep.path = append(ep.path, part)
		if !p.peek().isSymbol(".") {
			break
		}
		p.next()
	}
	if ep.cols == nil {
		if len(ep.path) < 2 {
			p.fail(ep.tok, "expected table.column")
		}
		ep.cols = []ident.Name{ep.path[len(ep.path)-1]}
		ep.path = ep.path[:len(ep.path)-1]
	}
	if len(ep.path) == 0 || len(ep.path) > 2 {
		p.fail(ep.tok, "expected table.column or schema.table.column")
	}
	return ep
}

func (p *parser) columnList() []ident.Name {
	p.expectSymbol("(")
	var cols []ident.Name
	for {
		c, _ := p.name("a column name")
		cols = append(cols, c)
		if p.peek().isSymbol(",") {
			p.next()
			continue
		}
		p.expectSymbol(")")
		return cols
	}
}

func (p *parser) indexes(ts *tableScope) {
	kw := p.next()
	p.expectSymbol("{")
	for !p.peek().isSymbol("}") {
		start := p.peek()
		var cols []ident.Name
		var exprs []string
		switch {
		case start.kind == tokEOF:
			p.fail(kw, "expected '}' to close indexes")
		case start.isSymbol("("):
			p.next()
			for {
				t := p.peek()
				switch t.kind {
				case tokExpr:
					p.next()
					exprs = append(exprs, t.text)
				case tokWord, tokString:
					c, _ := p.name("a column name")
					cols = append(cols, c)
				default:
					p.fail(t, "expected a column or an expression, found %s", t.describe())
				}
				if p.peek().isSymbol(",") {
					p.next()
					continue
				}
				p.expectSymbol(")")
				break
			}
		case start.kind == tokExpr:
			p.next()
			exprs = append(exprs, start.text)
		case start.kind == tokWord || start.kind == tokString:
			c, _ := p.name("a column name")
			cols = append(cols, c)
		default:
			p.fail(start, "expected an index definition, found %s", start.describe())
		}
		var unique, pk bool
		var name ident.Name
		var method string
		if p.peek().isSymbol("[") {
			p.next()
			for {
				key := p.next()
				if key.kind != tokWord {
					p.fail(key, "expected an index setting, found %s", key.describe())
				}
				switch strings.ToLower(key.text) {
				case "unique":
					unique = true
				case "pk":
					pk = true
				case "primary":
					p.expectWord("key")
					pk = true
				case "name":
					p.expectSymbol(":")
					n, err := ident.Parse(p.stringValue("an index name"))
					if err != nil {
						p.fail(key, "index name: %s", errs.Message(err))
					}
					name = n
				case "type":
					p.expectSymbol(":")
					t := p.next()
					if t.kind != tokWord && t.kind != tokString {
						p.fail(t, "expected an index type, found %s", t.describe())
					}
					method = strings.ToLower(t.text)
				case "note":
					p.expectSymbol(":")
					p.stringValue("a note")
				default:
					p.fail(key, "unknown index setting '%s'", key.text)
				}
				if p.peek().isSymbol(",") {
					p.next()
					continue
				}
				p.expectSymbol("]")
				break
			}
		}
		switch {
		case pk:
			if ts.t.PrimaryKey != nil || len(ts.pkCols) > 0 {
				p.fail(start, "table %s has more than one primary key", ts.q)
			}
			if len(exprs) > 0 {
				p.fail(start, "a primary key cannot contain expressions")
			}
			if name == "" {
				name = pkName(ts.q.Name)
			}
			ts.t.PrimaryKey = &schema.Key{Name: name, Columns: cols}
		case unique && method == "" && len(exprs) == 0:
			// A unique index on plain columns with no access method is a
			// unique constraint, like [unique] on a column.
			if name == "" {
				name = keyName(ts.q.Name, cols)
			}
			ts.t.Uniques = append(ts.t.Uniques, schema.Key{Name: name, Columns: cols})
			p.lines[posUnique(ts.q, cols)] = start.line
		default:
			if name == "" {
				name = freeIndexName(ts.t, cols, len(exprs))
			}
			ts.t.Indexes = append(ts.t.Indexes, schema.Index{
				Name:       name,
				Columns:    cols,
				Expression: strings.Join(exprs, ", "),
				Unique:     unique,
				Method:     method,
			})
			p.lines[posIndex(ts.q, name)] = start.line
		}
	}
	p.expectSymbol("}")
}

// freeIndexName is the conventional index name, with a counter appended when
// the table already uses it (two expression indexes, for instance).
func freeIndexName(t *schema.Table, cols []ident.Name, expressions int) ident.Name {
	base := indexName(t.Name, cols, expressions)
	name := base
	for i := 1; indexNameTaken(t, name); i++ {
		suffix := strconv.Itoa(i)
		name = ident.Name(truncateName(string(base), ident.MaxLen-len(suffix)) + suffix)
	}
	return name
}

func indexNameTaken(t *schema.Table, name ident.Name) bool {
	if t.PrimaryKey != nil && t.PrimaryKey.Name == name {
		return true
	}
	for _, u := range t.Uniques {
		if u.Name == name {
			return true
		}
	}
	for _, ix := range t.Indexes {
		if ix.Name == name {
			return true
		}
	}
	return false
}

// ref reads a standalone ref: "Ref name: a.b > c.d [delete: cascade]" or the
// block form "Ref name { a.b > c.d }".
func (p *parser) ref() {
	kw := p.next()
	var name ident.Name
	if t := p.peek(); t.kind == tokWord || t.kind == tokString {
		name, _ = p.name("a ref name")
	}
	switch {
	case p.peek().isSymbol(":"):
		p.next()
		p.refLine(name)
	case p.peek().isSymbol("{"):
		p.next()
		for !p.peek().isSymbol("}") {
			if p.peek().kind == tokEOF {
				p.fail(kw, "expected '}' to close Ref")
			}
			p.refLine(name)
		}
		p.next()
	default:
		p.fail(p.peek(), "expected ':' or '{' after Ref, found %s", p.peek().describe())
	}
}

func (p *parser) refLine(name ident.Name) {
	r := pendingRef{tok: p.peek(), name: name}
	r.left = p.endpoint()
	r.rel = p.relation()
	r.right = p.endpoint()
	if p.peek().isSymbol("[") {
		p.next()
		for {
			key := p.next()
			if key.kind != tokWord {
				p.fail(key, "expected a ref setting, found %s", key.describe())
			}
			switch strings.ToLower(key.text) {
			case "delete":
				p.expectSymbol(":")
				r.onDelete = p.action(key)
			case "update":
				p.expectSymbol(":")
				r.onUpdate = p.action(key)
			case "color":
				p.expectSymbol(":")
				p.next()
			default:
				p.fail(key, "unknown ref setting '%s'", key.text)
			}
			if p.peek().isSymbol(",") {
				p.next()
				continue
			}
			p.expectSymbol("]")
			break
		}
	}
	p.refs = append(p.refs, r)
}

// action reads a referential action: cascade, restrict, set null, set
// default or no action.
func (p *parser) action(key token) string {
	var words []string
	for p.peek().kind == tokWord {
		words = append(words, strings.ToUpper(p.next().text))
	}
	a := strings.Join(words, " ")
	switch a {
	case "CASCADE", "RESTRICT", "SET NULL", "SET DEFAULT", "NO ACTION":
		return a
	}
	p.fail(key, "%s: expected cascade, restrict, set null, set default or no action", key.text)
	return ""
}

// resolveRefs turns the refs into foreign keys now that aliases and tables
// are all known. The foreign key sits on the left table for > and -, and on
// the right table for <. A ref that repeats an existing foreign key (an
// inline ref plus a standalone one adding actions) updates it.
func (p *parser) resolveRefs() {
	for _, r := range p.refs {
		var fkSide, refSide endpoint
		switch r.rel {
		case ">", "-":
			fkSide, refSide = r.left, r.right
		case "<":
			fkSide, refSide = r.right, r.left
		default:
			p.fail(r.tok, "many-to-many refs (<>) are not supported; use a join table")
		}
		fkTable := p.resolveTable(fkSide)
		t := p.db.Table(fkTable)
		if t == nil {
			p.fail(fkSide.tok, "unknown table %s", fkTable)
		}
		if len(fkSide.cols) != len(refSide.cols) {
			p.fail(r.tok, "ref joins %d columns to %d", len(fkSide.cols), len(refSide.cols))
		}
		fk := schema.ForeignKey{
			Name:       r.name,
			Columns:    fkSide.cols,
			References: p.resolveTable(refSide),
			RefColumns: refSide.cols,
			OnDelete:   r.onDelete,
			OnUpdate:   r.onUpdate,
		}
		if existing := findForeignKey(t, fk); existing != nil {
			if r.name != "" {
				existing.Name = r.name
			}
			if fk.OnDelete != "" {
				existing.OnDelete = fk.OnDelete
			}
			if fk.OnUpdate != "" {
				existing.OnUpdate = fk.OnUpdate
			}
			continue
		}
		if fk.Name == "" {
			fk.Name = fkName(t.Name, fk.Columns)
		}
		t.ForeignKeys = append(t.ForeignKeys, fk)
		p.lines[posForeignKey(fkTable, &fk)] = r.tok.line
	}
}

func (p *parser) resolveTable(ep endpoint) ident.Qualified {
	if ep.resolved {
		return ep.table
	}
	if len(ep.path) == 2 {
		return ident.Q(ep.path[0], ep.path[1])
	}
	if q, ok := p.aliases[ep.path[0]]; ok {
		return q
	}
	return ident.Q(defaultSchema, ep.path[0])
}

func findForeignKey(t *schema.Table, fk schema.ForeignKey) *schema.ForeignKey {
	for i := range t.ForeignKeys {
		c := &t.ForeignKeys[i]
		if sameNames(c.Columns, fk.Columns) && c.References == fk.References && sameNames(c.RefColumns, fk.RefColumns) {
			return c
		}
	}
	return nil
}
