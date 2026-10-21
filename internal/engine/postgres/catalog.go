package postgres

import (
	"context"
	_ "embed"
	"strings"

	"github.com/jackc/pgx/v5"

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
	//go:embed sql/catalog_schemas.sql
	catalogSchemasSQL string
	//go:embed sql/catalog_tables.sql
	catalogTablesSQL string
	//go:embed sql/catalog_columns.sql
	catalogColumnsSQL string
	//go:embed sql/catalog_constraints.sql
	catalogConstraintsSQL string
	//go:embed sql/catalog_indexes.sql
	catalogIndexesSQL string
	//go:embed sql/catalog_views.sql
	catalogViewsSQL string
	//go:embed sql/catalog_enums.sql
	catalogEnumsSQL string
	//go:embed sql/catalog_functions.sql
	catalogFunctionsSQL string
	//go:embed sql/catalog_sequences.sql
	catalogSequencesSQL string
)

// Databases implements engine.Catalog.
func (s *Session) Databases(ctx context.Context) ([]engine.DatabaseInfo, error) {
	rows, err := s.pool.Query(ctx, databasesSQL)
	if err != nil {
		return nil, wrap(err, "cannot list databases")
	}
	out, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (engine.DatabaseInfo, error) {
		var d engine.DatabaseInfo
		var name string
		err := row.Scan(&name, &d.Owner, &d.Encoding, &d.SizeBytes)
		d.Name = ident.Name(name)
		return d, err
	})
	if err != nil {
		return nil, wrap(err, "cannot list databases")
	}
	return out, nil
}

// Database implements engine.Catalog.
func (s *Session) Database(ctx context.Context) (*schema.Database, error) {
	return s.load(ctx, nil, nil)
}

// Table implements engine.Catalog.
func (s *Session) Table(ctx context.Context, q ident.Qualified) (*schema.Table, error) {
	sch, name := string(q.Schema), string(q.Name)
	db, err := s.load(ctx, &sch, &name)
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
	db          *schema.Database
	schemas     map[string]*schema.Schema
	tables      map[[2]string]*schema.Table
	viewColumns map[[2]string][]schema.Column
}

func (c *catalog) schema(name string) *schema.Schema {
	if s, ok := c.schemas[name]; ok {
		return s
	}
	s := &schema.Schema{Name: ident.Name(name)}
	c.schemas[name] = s
	return s
}

// load reads the model, restricted to one schema and/or table when given.
// A table filter skips the objects a single table does not need.
func (s *Session) load(ctx context.Context, schemaFilter, tableFilter *string) (*schema.Database, error) {
	c := &catalog{
		db:          &schema.Database{},
		schemas:     map[string]*schema.Schema{},
		tables:      map[[2]string]*schema.Table{},
		viewColumns: map[[2]string][]schema.Column{},
	}
	steps := []struct {
		name string
		fn   func(context.Context, *catalog, *string, *string) error
	}{
		{"schemas", s.loadSchemas},
		{"tables", s.loadTables},
		{"columns", s.loadColumns},
		{"constraints", s.loadConstraints},
		{"indexes", s.loadIndexes},
	}
	if tableFilter == nil {
		steps = append(steps, []struct {
			name string
			fn   func(context.Context, *catalog, *string, *string) error
		}{
			{"views", s.loadViews},
			{"enums", s.loadEnums},
			{"functions", s.loadFunctions},
			{"sequences", s.loadSequences},
		}...)
	}
	for _, st := range steps {
		if err := st.fn(ctx, c, schemaFilter, tableFilter); err != nil {
			return nil, wrap(err, "cannot read "+st.name)
		}
	}
	for key, t := range c.tables {
		sc := c.schema(key[0])
		sc.Tables = append(sc.Tables, *t)
	}
	for _, sch := range c.schemas {
		c.db.Schemas = append(c.db.Schemas, *sch)
	}
	c.db.Normalize()
	return c.db, nil
}

func (s *Session) loadSchemas(ctx context.Context, c *catalog, schemaFilter, _ *string) error {
	rows, err := s.pool.Query(ctx, catalogSchemasSQL, schemaFilter)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var db, name string
		if err := rows.Scan(&db, &name); err != nil {
			return err
		}
		c.db.Name = ident.Name(db)
		c.schema(name)
	}
	return rows.Err()
}

func (s *Session) loadTables(ctx context.Context, c *catalog, schemaFilter, tableFilter *string) error {
	rows, err := s.pool.Query(ctx, catalogTablesSQL, schemaFilter, tableFilter)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var sch, name string
		t := &schema.Table{}
		if err := rows.Scan(&sch, &name, &t.RowEstimate, &t.SizeBytes, &t.Comment); err != nil {
			return err
		}
		if _, ok := c.schemas[sch]; !ok {
			continue
		}
		t.Name = ident.Name(name)
		c.tables[[2]string{sch, name}] = t
	}
	return rows.Err()
}

func (s *Session) loadColumns(ctx context.Context, c *catalog, schemaFilter, tableFilter *string) error {
	rows, err := s.pool.Query(ctx, catalogColumnsSQL, schemaFilter, tableFilter)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var sch, rel, identity string
		var col schema.Column
		var name string
		if err := rows.Scan(&sch, &rel, &name, &col.Type, &col.Nullable, &col.Default, &identity, &col.Comment); err != nil {
			return err
		}
		col.Name = ident.Name(name)
		col.Type = shortType(col.Type)
		switch identity {
		case "a":
			col.Identity = schema.IdentityAlways
		case "d":
			col.Identity = schema.IdentityByDefault
		}
		key := [2]string{sch, rel}
		if t, ok := c.tables[key]; ok {
			t.Columns = append(t.Columns, col)
		} else {
			// Views are loaded after columns; keep theirs until then.
			c.viewColumns[key] = append(c.viewColumns[key], col)
		}
	}
	return rows.Err()
}

func (s *Session) loadConstraints(ctx context.Context, c *catalog, schemaFilter, tableFilter *string) error {
	rows, err := s.pool.Query(ctx, catalogConstraintsSQL, schemaFilter, tableFilter)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var sch, rel, name, typ, refSchema, refTable, onDelete, onUpdate, def string
		var cols, refCols []string
		if err := rows.Scan(&sch, &rel, &name, &typ, &cols, &refSchema, &refTable, &refCols, &onDelete, &onUpdate, &def); err != nil {
			return err
		}
		t, ok := c.tables[[2]string{sch, rel}]
		if !ok {
			continue
		}
		switch typ {
		case "p":
			t.PrimaryKey = &schema.Key{Name: ident.Name(name), Columns: names(cols)}
		case "u":
			t.Uniques = append(t.Uniques, schema.Key{Name: ident.Name(name), Columns: names(cols)})
		case "c":
			t.Checks = append(t.Checks, schema.Check{Name: ident.Name(name), Expression: checkExpression(def)})
		case "f":
			t.ForeignKeys = append(t.ForeignKeys, schema.ForeignKey{
				Name:       ident.Name(name),
				Columns:    names(cols),
				References: ident.Q(ident.Name(refSchema), ident.Name(refTable)),
				RefColumns: names(refCols),
				OnDelete:   fkAction(onDelete),
				OnUpdate:   fkAction(onUpdate),
			})
		}
	}
	return rows.Err()
}

func (s *Session) loadIndexes(ctx context.Context, c *catalog, schemaFilter, tableFilter *string) error {
	rows, err := s.pool.Query(ctx, catalogIndexesSQL, schemaFilter, tableFilter)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var sch, rel, name string
		var cols, keys []string
		var idx schema.Index
		if err := rows.Scan(&sch, &rel, &name, &idx.Unique, &idx.Method, &idx.Predicate, &cols, &keys, &idx.SizeBytes, &idx.Scans); err != nil {
			return err
		}
		t, ok := c.tables[[2]string{sch, rel}]
		if !ok {
			continue
		}
		idx.Name = ident.Name(name)
		plain := len(cols) > 0
		for _, col := range cols {
			if col == "" {
				plain = false
			}
		}
		if plain {
			idx.Columns = names(cols)
		} else {
			idx.Expression = strings.Join(keys, ", ")
		}
		t.Indexes = append(t.Indexes, idx)
	}
	return rows.Err()
}

func (s *Session) loadViews(ctx context.Context, c *catalog, schemaFilter, _ *string) error {
	rows, err := s.pool.Query(ctx, catalogViewsSQL, schemaFilter)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var sch, name string
		var v schema.View
		if err := rows.Scan(&sch, &name, &v.Materialized, &v.Definition); err != nil {
			return err
		}
		sc, ok := c.schemas[sch]
		if !ok {
			continue
		}
		v.Name = ident.Name(name)
		v.Definition = strings.TrimSuffix(strings.TrimSpace(v.Definition), ";")
		v.Columns = c.viewColumns[[2]string{sch, name}]
		sc.Views = append(sc.Views, v)
	}
	return rows.Err()
}

func (s *Session) loadEnums(ctx context.Context, c *catalog, schemaFilter, _ *string) error {
	rows, err := s.pool.Query(ctx, catalogEnumsSQL, schemaFilter)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var sch, name string
		var labels []string
		if err := rows.Scan(&sch, &name, &labels); err != nil {
			return err
		}
		if sc, ok := c.schemas[sch]; ok {
			sc.Enums = append(sc.Enums, schema.Enum{Name: ident.Name(name), Values: labels})
		}
	}
	return rows.Err()
}

func (s *Session) loadFunctions(ctx context.Context, c *catalog, schemaFilter, _ *string) error {
	rows, err := s.pool.Query(ctx, catalogFunctionsSQL, schemaFilter)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var sch, name string
		var f schema.Function
		if err := rows.Scan(&sch, &name, &f.Signature, &f.Returns, &f.Language); err != nil {
			return err
		}
		if sc, ok := c.schemas[sch]; ok {
			f.Name = ident.Name(name)
			sc.Functions = append(sc.Functions, f)
		}
	}
	return rows.Err()
}

func (s *Session) loadSequences(ctx context.Context, c *catalog, schemaFilter, _ *string) error {
	rows, err := s.pool.Query(ctx, catalogSequencesSQL, schemaFilter)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var sch, name string
		if err := rows.Scan(&sch, &name); err != nil {
			return err
		}
		if sc, ok := c.schemas[sch]; ok {
			sc.Sequences = append(sc.Sequences, schema.Sequence{Name: ident.Name(name)})
		}
	}
	return rows.Err()
}

func names(ss []string) []ident.Name {
	out := make([]ident.Name, len(ss))
	for i, s := range ss {
		out[i] = ident.Name(s)
	}
	return out
}

// fkAction expands pg_constraint's action codes.
func fkAction(code string) string {
	switch code {
	case "r":
		return "RESTRICT"
	case "c":
		return "CASCADE"
	case "n":
		return "SET NULL"
	case "d":
		return "SET DEFAULT"
	default:
		return "NO ACTION"
	}
}

// checkExpression extracts the expression from pg_get_constraintdef's
// "CHECK (expr) [NOT VALID]" exactly as the server deparses it.
func checkExpression(def string) string {
	start := strings.IndexByte(def, '(')
	if start < 0 {
		return def
	}
	depth := 0
	for i := start; i < len(def); i++ {
		switch def[i] {
		case '(':
			depth++
		case ')':
			depth--
			if depth == 0 {
				return def[start+1 : i]
			}
		}
	}
	return def[start+1:]
}
