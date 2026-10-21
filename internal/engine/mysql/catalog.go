package mysql

import (
	"context"
	"database/sql"
	_ "embed"
	"regexp"
	"strings"

	"github.com/felipemalacarne/mesa/internal/core/errs"
	"github.com/felipemalacarne/mesa/internal/core/ident"
	"github.com/felipemalacarne/mesa/internal/core/schema"
	"github.com/felipemalacarne/mesa/internal/engine"
)

var (
	//go:embed sql/column_names.sql
	columnNamesSQL string
	//go:embed sql/databases.sql
	databasesSQL string
	//go:embed sql/catalog_tables.sql
	catalogTablesSQL string
	//go:embed sql/catalog_columns.sql
	catalogColumnsSQL string
	//go:embed sql/catalog_keys.sql
	catalogKeysSQL string
	//go:embed sql/catalog_checks.sql
	catalogChecksSQL string
	//go:embed sql/catalog_indexes.sql
	catalogIndexesSQL string
	//go:embed sql/catalog_views.sql
	catalogViewsSQL string
	//go:embed sql/catalog_routines.sql
	catalogRoutinesSQL string
)

// Databases implements engine.Catalog.
func (s *Session) Databases(ctx context.Context) ([]engine.DatabaseInfo, error) {
	rows, err := s.db.QueryContext(ctx, databasesSQL)
	if err != nil {
		return nil, wrap(err, "cannot list databases")
	}
	defer func() { _ = rows.Close() }()
	var out []engine.DatabaseInfo
	for rows.Next() {
		var d engine.DatabaseInfo
		var name string
		if err := rows.Scan(&name, &d.Encoding, &d.SizeBytes); err != nil {
			return nil, wrap(err, "cannot list databases")
		}
		d.Name = ident.Name(name)
		out = append(out, d)
	}
	return out, wrap(rows.Err(), "cannot list databases")
}

// Database implements engine.Catalog. The database has one schema, named
// after it, even when it is empty.
func (s *Session) Database(ctx context.Context) (*schema.Database, error) {
	return s.load(ctx, s.database, nil)
}

// Table implements engine.Catalog.
func (s *Session) Table(ctx context.Context, q ident.Qualified) (*schema.Table, error) {
	q = s.qualify(q)
	name := string(q.Name)
	db, err := s.load(ctx, string(q.Schema), &name)
	if err != nil {
		return nil, err
	}
	t := db.Table(q)
	if t == nil {
		return nil, errs.Newf(errs.NotFound, "table %s not found", q)
	}
	return t, nil
}

// catalog accumulates query results before they are flattened into a model.
type catalog struct {
	schema      string
	sc          *schema.Schema
	tables      map[string]*schema.Table
	order       []string
	views       map[string]bool
	viewColumns map[string][]schema.Column
}

// load reads one database ("schema" in MySQL's catalog), or one table of it.
func (s *Session) load(ctx context.Context, database string, tableFilter *string) (*schema.Database, error) {
	c := &catalog{
		schema:      database,
		sc:          &schema.Schema{Name: ident.Name(database)},
		tables:      map[string]*schema.Table{},
		views:       map[string]bool{},
		viewColumns: map[string][]schema.Column{},
	}
	steps := []struct {
		name string
		fn   func(context.Context, *catalog, *string) error
	}{
		{"tables", s.loadTables},
		{"columns", s.loadColumns},
		{"keys", s.loadKeys},
		{"checks", s.loadChecks},
		{"indexes", s.loadIndexes},
	}
	if tableFilter == nil {
		steps = append(steps, []struct {
			name string
			fn   func(context.Context, *catalog, *string) error
		}{
			{"views", s.loadViews},
			{"routines", s.loadRoutines},
		}...)
	}
	for _, st := range steps {
		if err := st.fn(ctx, c, tableFilter); err != nil {
			return nil, wrap(err, "cannot read "+st.name)
		}
	}
	for _, name := range c.order {
		c.sc.Tables = append(c.sc.Tables, *c.tables[name])
	}
	db := &schema.Database{Name: ident.Name(database), Schemas: []schema.Schema{*c.sc}}
	db.Normalize()
	return db, nil
}

// filterArgs binds the schema and the optional table filter.
func filterArgs(c *catalog, tableFilter *string) []any {
	var t any
	if tableFilter != nil {
		t = *tableFilter
	}
	return []any{c.schema, t, t}
}

func (s *Session) loadTables(ctx context.Context, c *catalog, tableFilter *string) error {
	rows, err := s.db.QueryContext(ctx, catalogTablesSQL, filterArgs(c, tableFilter)...)
	if err != nil {
		return err
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var name, typ string
		t := &schema.Table{}
		if err := rows.Scan(&name, &typ, &t.RowEstimate, &t.SizeBytes, &t.Comment); err != nil {
			return err
		}
		if typ != "BASE TABLE" {
			c.views[name] = true
			continue
		}
		t.Name = ident.Name(name)
		c.tables[name] = t
		c.order = append(c.order, name)
	}
	return rows.Err()
}

func (s *Session) loadColumns(ctx context.Context, c *catalog, tableFilter *string) error {
	rows, err := s.db.QueryContext(ctx, catalogColumnsSQL, filterArgs(c, tableFilter)...)
	if err != nil {
		return err
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var rel, name, colType, dataType, extra string
		var def sql.NullString
		var col schema.Column
		if err := rows.Scan(&rel, &name, &colType, &dataType, &col.Nullable, &def, &extra, &col.Comment); err != nil {
			return err
		}
		col.Name = ident.Name(name)
		col.Type = colType
		extra = strings.ToLower(extra)
		if strings.Contains(extra, "auto_increment") {
			col.Identity = schema.IdentityByDefault
		}
		if def.Valid {
			d := defaultSQL(def.String, dataType, extra)
			col.Default = &d
		}
		if t, ok := c.tables[rel]; ok {
			t.Columns = append(t.Columns, col)
		} else {
			c.viewColumns[rel] = append(c.viewColumns[rel], col)
		}
	}
	return rows.Err()
}

// defaultSQL turns information_schema's column_default into an expression:
// literals come back unquoted, expressions are marked DEFAULT_GENERATED.
func defaultSQL(def, dataType, extra string) string {
	upper := strings.ToUpper(def)
	switch {
	case strings.HasPrefix(upper, "CURRENT_TIMESTAMP"), upper == "NULL":
		return def
	case strings.Contains(extra, "default_generated"):
		return "(" + def + ")"
	}
	switch strings.ToLower(dataType) {
	case "tinyint", "smallint", "mediumint", "int", "integer", "bigint", "decimal", "numeric", "float", "double", "real", "bit", "year":
		return def
	}
	return quoteLiteral(def)
}

func (s *Session) loadKeys(ctx context.Context, c *catalog, tableFilter *string) error {
	rows, err := s.db.QueryContext(ctx, catalogKeysSQL, filterArgs(c, tableFilter)...)
	if err != nil {
		return err
	}
	defer func() { _ = rows.Close() }()
	type keyID struct{ table, name string }
	fks := map[keyID]*schema.ForeignKey{}
	uniques := map[keyID]*schema.Key{}
	var fkOrder, uqOrder []keyID
	for rows.Next() {
		var rel, name, typ, col, refSchema, refTable, refCol, onDelete, onUpdate string
		if err := rows.Scan(&rel, &name, &typ, &col, &refSchema, &refTable, &refCol, &onDelete, &onUpdate); err != nil {
			return err
		}
		t, ok := c.tables[rel]
		if !ok {
			continue
		}
		id := keyID{rel, name}
		switch typ {
		case "PRIMARY KEY":
			if t.PrimaryKey == nil {
				t.PrimaryKey = &schema.Key{Name: ident.Name(name)}
			}
			t.PrimaryKey.Columns = append(t.PrimaryKey.Columns, ident.Name(col))
		case "UNIQUE":
			k, ok := uniques[id]
			if !ok {
				k = &schema.Key{Name: ident.Name(name)}
				uniques[id] = k
				uqOrder = append(uqOrder, id)
			}
			k.Columns = append(k.Columns, ident.Name(col))
		case "FOREIGN KEY":
			fk, ok := fks[id]
			if !ok {
				fk = &schema.ForeignKey{
					Name:       ident.Name(name),
					References: ident.Q(ident.Name(refSchema), ident.Name(refTable)),
					OnDelete:   onDelete,
					OnUpdate:   onUpdate,
				}
				fks[id] = fk
				fkOrder = append(fkOrder, id)
			}
			fk.Columns = append(fk.Columns, ident.Name(col))
			fk.RefColumns = append(fk.RefColumns, ident.Name(refCol))
		}
	}
	for _, id := range uqOrder {
		t := c.tables[id.table]
		t.Uniques = append(t.Uniques, *uniques[id])
	}
	for _, id := range fkOrder {
		t := c.tables[id.table]
		t.ForeignKeys = append(t.ForeignKeys, *fks[id])
	}
	return rows.Err()
}

func (s *Session) loadChecks(ctx context.Context, c *catalog, tableFilter *string) error {
	rows, err := s.db.QueryContext(ctx, catalogChecksSQL, filterArgs(c, tableFilter)...)
	if err != nil {
		if errs.KindOf(wrap(err, "")) == errs.NotFound {
			return nil // before 8.0.16 there are no check constraints
		}
		return err
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var rel, name, clause string
		if err := rows.Scan(&rel, &name, &clause); err != nil {
			return err
		}
		if t, ok := c.tables[rel]; ok {
			t.Checks = append(t.Checks, schema.Check{Name: ident.Name(name), Expression: checkExpression(clause)})
		}
	}
	return rows.Err()
}

// charsetIntroducer matches the _utf8mb4'…' prefixes MySQL adds to string
// literals in stored expressions.
var charsetIntroducer = regexp.MustCompile(`_[a-z0-9]+'`)

// checkExpression turns a stored check clause back into the expression a
// person writes: no outer parentheses, no backticks, no charset introducers.
func checkExpression(clause string) string {
	e := strings.TrimSpace(clause)
	for strings.HasPrefix(e, "(") && strings.HasSuffix(e, ")") && balanced(e[1:len(e)-1]) {
		e = strings.TrimSpace(e[1 : len(e)-1])
	}
	e = charsetIntroducer.ReplaceAllString(e, "'")
	return strings.ReplaceAll(e, "`", "")
}

func balanced(s string) bool {
	depth := 0
	for _, r := range s {
		switch r {
		case '(':
			depth++
		case ')':
			depth--
			if depth < 0 {
				return false
			}
		}
	}
	return depth == 0
}

func (s *Session) loadIndexes(ctx context.Context, c *catalog, tableFilter *string) error {
	rows, err := s.db.QueryContext(ctx, catalogIndexesSQL, filterArgs(c, tableFilter)...)
	if err != nil {
		return err
	}
	defer func() { _ = rows.Close() }()
	type idxID struct{ table, name string }
	idx := map[idxID]*schema.Index{}
	var order []idxID
	for rows.Next() {
		var rel, name, method, col, expr string
		var unique bool
		var subPart sql.NullInt64
		if err := rows.Scan(&rel, &name, &unique, &method, &col, &expr, &subPart); err != nil {
			return err
		}
		t, ok := c.tables[rel]
		if !ok || name == "PRIMARY" || isConstraint(t, ident.Name(name)) {
			continue
		}
		id := idxID{rel, name}
		ix, ok := idx[id]
		if !ok {
			ix = &schema.Index{Name: ident.Name(name), Unique: unique}
			if m := strings.ToLower(method); m != "btree" {
				ix.Method = m
			}
			idx[id] = ix
			order = append(order, id)
		}
		switch {
		case expr != "":
			ix.Expression = joinExpr(ix.Expression, "("+checkExpression(expr)+")")
		case subPart.Valid:
			ix.Expression = joinExpr(ix.Expression, quote(ident.Name(col))+"("+itoa(subPart.Int64)+")")
		default:
			ix.Columns = append(ix.Columns, ident.Name(col))
		}
	}
	for _, id := range order {
		ix := idx[id]
		if ix.Expression != "" && len(ix.Columns) > 0 {
			// Mixed column and expression keys: keep them all as an expression.
			parts := make([]string, 0, len(ix.Columns)+1)
			for _, c := range ix.Columns {
				parts = append(parts, quote(c))
			}
			ix.Expression = strings.Join(append(parts, ix.Expression), ", ")
			ix.Columns = nil
		}
		t := c.tables[id.table]
		t.Indexes = append(t.Indexes, *ix)
	}
	return rows.Err()
}

func joinExpr(a, b string) string {
	if a == "" {
		return b
	}
	return a + ", " + b
}

// isConstraint reports whether an index backs a unique constraint or is the
// one MySQL created for a foreign key.
func isConstraint(t *schema.Table, name ident.Name) bool {
	for _, u := range t.Uniques {
		if u.Name == name {
			return true
		}
	}
	for _, fk := range t.ForeignKeys {
		if fk.Name == name {
			return true
		}
	}
	return false
}

func (s *Session) loadViews(ctx context.Context, c *catalog, _ *string) error {
	rows, err := s.db.QueryContext(ctx, catalogViewsSQL, c.schema)
	if err != nil {
		return err
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var name string
		var v schema.View
		if err := rows.Scan(&name, &v.Definition); err != nil {
			return err
		}
		v.Name = ident.Name(name)
		v.Columns = c.viewColumns[name]
		c.sc.Views = append(c.sc.Views, v)
	}
	return rows.Err()
}

func (s *Session) loadRoutines(ctx context.Context, c *catalog, _ *string) error {
	rows, err := s.db.QueryContext(ctx, catalogRoutinesSQL, c.schema)
	if err != nil {
		return err
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var name, kind, returns, params string
		if err := rows.Scan(&name, &kind, &returns, &params); err != nil {
			return err
		}
		f := schema.Function{Name: ident.Name(name), Signature: "(" + params + ")", Returns: returns, Language: "sql"}
		if kind == "PROCEDURE" {
			f.Language = "procedure"
		}
		c.sc.Functions = append(c.sc.Functions, f)
	}
	return rows.Err()
}
