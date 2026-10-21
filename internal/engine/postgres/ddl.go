package postgres

import (
	"fmt"
	"strings"

	"github.com/felipemalacarne/mesa/internal/core/changeset"
	"github.com/felipemalacarne/mesa/internal/core/errs"
	"github.com/felipemalacarne/mesa/internal/core/ident"
	"github.com/felipemalacarne/mesa/internal/core/schema"
)

// schemaChange renders one structural change as DDL, its reverse, and the
// risks a reviewer should know about. The reverse is built from the current
// model when a change destroys something; when the model is not available
// the change is marked irreversible instead.
func (d dialect) schemaChange(c schema.Change) (up, down []changeset.Statement, risks []changeset.Risk, err error) {
	t := c.Table.Quote()
	cur := d.table(c.Table)
	add := func(s changeset.Statement) { up = append(up, s) }
	rev := func(s changeset.Statement) { down = append(down, s) }
	warn := func(level changeset.RiskLevel, code changeset.RiskCode, msg, object string) {
		risks = append(risks, risk(level, code, msg, object))
	}
	irreversible := func(msg, object string) { warn(changeset.Info, changeset.RiskIrreversible, msg, object) }

	switch c.Kind {
	case schema.CreateSchema:
		add(ddl("CREATE SCHEMA "+c.Schema.Quote(), c.Describe()))
		rev(ddl("DROP SCHEMA "+c.Schema.Quote(), "Drop schema "+c.Schema.String()))

	case schema.DropSchema:
		add(ddl("DROP SCHEMA "+c.Schema.Quote(), c.Describe()))
		rev(ddl("CREATE SCHEMA "+c.Schema.Quote(), "Recreate schema "+c.Schema.String()))
		warn(changeset.Warning, changeset.RiskDataLoss, "schema "+c.Schema.String()+" is dropped; anything still inside it makes the statement fail", c.Schema.String())

	case schema.CreateEnum:
		if c.Enum == nil {
			return nil, nil, nil, errs.New(errs.Invalid, "create_enum needs the enum")
		}
		name := ident.Q(c.Schema, c.Enum.Name)
		add(ddl("CREATE TYPE "+name.Quote()+" AS ENUM ("+literalList(c.Enum.Values)+")", c.Describe()))
		rev(ddl("DROP TYPE "+name.Quote(), "Drop type "+name.String()))

	case schema.DropEnum:
		name := ident.Q(c.Schema, c.Name)
		add(ddl("DROP TYPE "+name.Quote(), c.Describe()))
		if e := d.enum(c.Schema, c.Name); e != nil {
			rev(ddl("CREATE TYPE "+name.Quote()+" AS ENUM ("+literalList(e.Values)+")", "Recreate type "+name.String()))
		} else {
			irreversible("the values of type "+name.String()+" are not known, so it cannot be recreated", name.String())
		}

	case schema.AddEnumValue:
		name := ident.Q(c.Schema, c.Name)
		for _, v := range c.Values {
			add(ddl("ALTER TYPE "+name.Quote()+" ADD VALUE IF NOT EXISTS "+quoteLiteral(v), "Add value '"+v+"' to type "+name.String()))
		}
		irreversible("PostgreSQL cannot remove a value from an enum type", name.String())
		if d.version > 0 && d.version < 12 {
			warn(changeset.Warning, changeset.RiskIrreversible, "before PostgreSQL 12, ALTER TYPE … ADD VALUE cannot run inside a transaction", name.String())
		}

	case schema.CreateTable:
		if c.NewTable == nil {
			return nil, nil, nil, errs.New(errs.Invalid, "create_table needs the table")
		}
		for _, s := range createTable(c.Table, c.NewTable) {
			add(s)
		}
		rev(ddl("DROP TABLE "+t, "Drop table "+c.Table.String()))

	case schema.DropTable:
		add(ddl("DROP TABLE "+t, c.Describe()))
		if cur != nil {
			for _, s := range createTable(c.Table, cur) {
				rev(s)
			}
			risks = append(risks, dropRisk(c.Table, cur))
		} else {
			warn(changeset.Danger, changeset.RiskDataLoss, "every row of "+c.Table.String()+" is lost", c.Table.String())
			irreversible("the definition of "+c.Table.String()+" is not known, so it cannot be recreated", c.Table.String())
		}

	case schema.AddColumn:
		if c.Column == nil {
			return nil, nil, nil, errs.New(errs.Invalid, "add_column needs the column")
		}
		col := c.Column
		add(ddl("ALTER TABLE "+t+" ADD COLUMN "+columnDef(col), c.Describe()))
		rev(ddl("ALTER TABLE "+t+" DROP COLUMN "+col.Name.Quote(), "Drop column "+c.Table.String()+"."+col.Name.String()))
		obj := c.Table.String() + "." + col.Name.String()
		switch {
		case !col.Nullable && col.Default == nil && col.Identity == schema.IdentityNone:
			warn(changeset.Danger, changeset.RiskFullScan, "a NOT NULL column without a default fails when "+c.Table.String()+" already has rows", obj)
		case col.Default != nil && volatileDefault(*col.Default):
			warn(d.sized(cur, changeset.Warning), changeset.RiskTableRewrite, "a volatile default rewrites every row of "+c.Table.String()+" under an exclusive lock", obj)
		}

	case schema.DropColumn:
		add(ddl("ALTER TABLE "+t+" DROP COLUMN "+c.Name.Quote(), c.Describe()))
		obj := c.Table.String() + "." + c.Name.String()
		old := c.OldColumn
		if old == nil && cur != nil {
			old = cur.Column(c.Name)
		}
		if old != nil {
			rev(ddl("ALTER TABLE "+t+" ADD COLUMN "+columnDef(old), "Re-add column "+obj+" (its values are gone)"))
		}
		warn(changeset.Danger, changeset.RiskDataLoss, "the values of "+obj+" are lost", obj)

	case schema.AlterColumnType:
		if c.Column == nil {
			return nil, nil, nil, errs.New(errs.Invalid, "alter_column_type needs the column")
		}
		col := c.Column
		obj := c.Table.String() + "." + col.Name.String()
		add(ddl("ALTER TABLE "+t+" ALTER COLUMN "+col.Name.Quote()+" TYPE "+col.Type+" USING "+col.Name.Quote()+"::"+col.Type, c.Describe()))
		if c.OldColumn != nil {
			old := c.OldColumn
			rev(ddl("ALTER TABLE "+t+" ALTER COLUMN "+col.Name.Quote()+" TYPE "+old.Type+" USING "+col.Name.Quote()+"::"+old.Type, "Change "+obj+" back to "+old.Type))
			if narrowing(old.Type, col.Type) {
				warn(changeset.Danger, changeset.RiskDataLoss, "changing "+obj+" from "+old.Type+" to "+col.Type+" can truncate or reject existing values", obj)
			}
			if !binaryCoercible(old.Type, col.Type) {
				warn(d.sized(cur, changeset.Warning), changeset.RiskTableRewrite, "changing the type of "+obj+" rewrites "+c.Table.String()+" under an exclusive lock", obj)
			}
		} else {
			irreversible("the previous type of "+obj+" is not known", obj)
			warn(d.sized(cur, changeset.Warning), changeset.RiskTableRewrite, "changing the type of "+obj+" may rewrite "+c.Table.String()+" under an exclusive lock", obj)
		}

	case schema.SetNotNull:
		obj := c.Table.String() + "." + c.Name.String()
		add(ddl("ALTER TABLE "+t+" ALTER COLUMN "+c.Name.Quote()+" SET NOT NULL", c.Describe()))
		rev(ddl("ALTER TABLE "+t+" ALTER COLUMN "+c.Name.Quote()+" DROP NOT NULL", "Allow nulls in "+obj))
		warn(d.sized(cur, changeset.Info), changeset.RiskFullScan, "every row of "+c.Table.String()+" is checked; the change fails if "+obj+" holds a null", obj)

	case schema.DropNotNull:
		obj := c.Table.String() + "." + c.Name.String()
		add(ddl("ALTER TABLE "+t+" ALTER COLUMN "+c.Name.Quote()+" DROP NOT NULL", c.Describe()))
		rev(ddl("ALTER TABLE "+t+" ALTER COLUMN "+c.Name.Quote()+" SET NOT NULL", "Require a value in "+obj))

	case schema.SetDefault:
		if c.Column == nil || c.Column.Default == nil {
			return nil, nil, nil, errs.New(errs.Invalid, "set_default needs the column and its default")
		}
		col := c.Column
		obj := c.Table.String() + "." + col.Name.String()
		add(ddl("ALTER TABLE "+t+" ALTER COLUMN "+col.Name.Quote()+" SET DEFAULT "+*col.Default, c.Describe()))
		rev(d.restoreDefault(t, cur, col.Name, obj))

	case schema.DropDefault:
		obj := c.Table.String() + "." + c.Name.String()
		add(ddl("ALTER TABLE "+t+" ALTER COLUMN "+c.Name.Quote()+" DROP DEFAULT", c.Describe()))
		if prev := d.previousDefault(cur, c.Name); prev != nil {
			rev(ddl("ALTER TABLE "+t+" ALTER COLUMN "+c.Name.Quote()+" SET DEFAULT "+*prev, "Restore the default of "+obj))
		} else {
			irreversible("the previous default of "+obj+" is not known", obj)
		}

	case schema.AddPrimaryKey:
		if c.Key == nil {
			return nil, nil, nil, errs.New(errs.Invalid, "add_primary_key needs the key")
		}
		name := keyName(c.Key.Name, c.Table.Name, nil, "pkey")
		add(ddl("ALTER TABLE "+t+" ADD CONSTRAINT "+name.Quote()+" PRIMARY KEY ("+quoteList(c.Key.Columns)+")", c.Describe()))
		rev(ddl("ALTER TABLE "+t+" DROP CONSTRAINT "+name.Quote(), "Drop the primary key of "+c.Table.String()))
		warn(d.sized(cur, changeset.Info), changeset.RiskExclusiveLock, "building the primary key index locks "+c.Table.String()+" against writes", c.Table.String())

	case schema.DropPrimaryKey:
		name := c.Name
		if name == "" && cur != nil && cur.PrimaryKey != nil {
			name = cur.PrimaryKey.Name
		}
		if name == "" {
			name = keyName("", c.Table.Name, nil, "pkey")
		}
		add(ddl("ALTER TABLE "+t+" DROP CONSTRAINT "+name.Quote(), c.Describe()))
		if cur != nil && cur.PrimaryKey != nil {
			rev(ddl("ALTER TABLE "+t+" ADD CONSTRAINT "+name.Quote()+" PRIMARY KEY ("+quoteList(cur.PrimaryKey.Columns)+")", "Restore the primary key of "+c.Table.String()))
		} else {
			irreversible("the columns of the primary key of "+c.Table.String()+" are not known", c.Table.String())
		}

	case schema.AddUnique:
		if c.Key == nil {
			return nil, nil, nil, errs.New(errs.Invalid, "add_unique needs the key")
		}
		name := keyName(c.Key.Name, c.Table.Name, c.Key.Columns, "key")
		add(ddl("ALTER TABLE "+t+" ADD CONSTRAINT "+name.Quote()+" UNIQUE ("+quoteList(c.Key.Columns)+")", c.Describe()))
		rev(ddl("ALTER TABLE "+t+" DROP CONSTRAINT "+name.Quote(), "Drop unique "+name.String()))
		warn(d.sized(cur, changeset.Info), changeset.RiskExclusiveLock, "building the unique index locks "+c.Table.String()+" against writes; duplicates make it fail", c.Table.String())

	case schema.DropUnique:
		add(ddl("ALTER TABLE "+t+" DROP CONSTRAINT "+c.Name.Quote(), c.Describe()))
		if k := findKeyByName(curUniques(cur), c.Name); k != nil {
			rev(ddl("ALTER TABLE "+t+" ADD CONSTRAINT "+c.Name.Quote()+" UNIQUE ("+quoteList(k.Columns)+")", "Restore unique "+c.Name.String()))
		} else {
			irreversible("the columns of unique constraint "+c.Name.String()+" are not known", c.Name.String())
		}

	case schema.AddCheck:
		if c.Check == nil {
			return nil, nil, nil, errs.New(errs.Invalid, "add_check needs the check")
		}
		name := c.Check.Name
		if name == "" {
			name = keyName("", c.Table.Name, nil, "check")
		}
		add(ddl("ALTER TABLE "+t+" ADD CONSTRAINT "+name.Quote()+" CHECK ("+c.Check.Expression+")", c.Describe()))
		rev(ddl("ALTER TABLE "+t+" DROP CONSTRAINT "+name.Quote(), "Drop check "+name.String()))
		warn(d.sized(cur, changeset.Info), changeset.RiskFullScan, "every row of "+c.Table.String()+" is checked against the constraint", c.Table.String())

	case schema.DropCheck:
		add(ddl("ALTER TABLE "+t+" DROP CONSTRAINT "+c.Name.Quote(), c.Describe()))
		if cur != nil {
			for _, ch := range cur.Checks {
				if ch.Name == c.Name {
					rev(ddl("ALTER TABLE "+t+" ADD CONSTRAINT "+c.Name.Quote()+" CHECK ("+ch.Expression+")", "Restore check "+c.Name.String()))
				}
			}
		}
		if len(down) == 0 {
			irreversible("the expression of check "+c.Name.String()+" is not known", c.Name.String())
		}

	case schema.CreateIndex:
		if c.Index == nil {
			return nil, nil, nil, errs.New(errs.Invalid, "create_index needs the index")
		}
		add(createIndex(c.Table, c.Index))
		rev(ddl("DROP INDEX "+ident.Q(c.Table.Schema, c.Index.Name).Quote(), "Drop index "+c.Index.Name.String()))
		warn(d.sized(cur, changeset.Info), changeset.RiskExclusiveLock, "building the index blocks writes to "+c.Table.String()+" until it is done", c.Index.Name.String())

	case schema.DropIndex:
		add(ddl("DROP INDEX "+ident.Q(c.Table.Schema, c.Name).Quote(), c.Describe()))
		if ix := findIndexByName(cur, c.Name); ix != nil {
			rev(createIndex(c.Table, ix))
		} else {
			irreversible("the definition of index "+c.Name.String()+" is not known", c.Name.String())
		}

	case schema.AddForeignKey:
		if c.ForeignKey == nil {
			return nil, nil, nil, errs.New(errs.Invalid, "add_foreign_key needs the foreign key")
		}
		fk := c.ForeignKey
		name := keyName(fk.Name, c.Table.Name, fk.Columns, "fkey")
		add(ddl("ALTER TABLE "+t+" ADD CONSTRAINT "+name.Quote()+" "+foreignKeyDef(fk), c.Describe()))
		rev(ddl("ALTER TABLE "+t+" DROP CONSTRAINT "+name.Quote(), "Drop foreign key "+name.String()))
		if cur != nil || d.current == nil {
			// A table created in this plan is empty: nothing to check.
			warn(d.sized(cur, changeset.Info), changeset.RiskFullScan, "every row of "+c.Table.String()+" is checked against "+fk.References.String()+"; both tables are locked meanwhile", name.String())
		}

	case schema.DropForeignKey:
		add(ddl("ALTER TABLE "+t+" DROP CONSTRAINT "+c.Name.Quote(), c.Describe()))
		if fk := findFKByName(cur, c.Name); fk != nil {
			rev(ddl("ALTER TABLE "+t+" ADD CONSTRAINT "+c.Name.Quote()+" "+foreignKeyDef(fk), "Restore foreign key "+c.Name.String()))
		} else {
			irreversible("the definition of foreign key "+c.Name.String()+" is not known", c.Name.String())
		}

	case schema.SetTableComment:
		add(ddl("COMMENT ON TABLE "+t+" IS "+commentLiteral(c.Comment), c.Describe()))
		if cur != nil {
			rev(ddl("COMMENT ON TABLE "+t+" IS "+commentLiteral(cur.Comment), "Restore the comment of "+c.Table.String()))
		}

	case schema.SetColumnComment:
		if c.Column == nil {
			return nil, nil, nil, errs.New(errs.Invalid, "set_column_comment needs the column")
		}
		col := t + "." + c.Column.Name.Quote()
		add(ddl("COMMENT ON COLUMN "+col+" IS "+commentLiteral(c.Comment), c.Describe()))
		if cur != nil {
			if old := cur.Column(c.Column.Name); old != nil {
				rev(ddl("COMMENT ON COLUMN "+col+" IS "+commentLiteral(old.Comment), "Restore the comment of "+c.Table.String()+"."+c.Column.Name.String()))
			}
		}

	default:
		return nil, nil, nil, errs.Newf(errs.Unsupported, "change kind %q is not supported by the PostgreSQL dialect", c.Kind)
	}
	return up, down, risks, nil
}

// createTable renders CREATE TABLE with its columns, primary key, unique
// and check constraints, then one CREATE INDEX per index. Foreign keys are
// separate changes so that every table exists before they are added.
func createTable(q ident.Qualified, t *schema.Table) []changeset.Statement {
	var parts []string
	for i := range t.Columns {
		parts = append(parts, "  "+columnDef(&t.Columns[i]))
	}
	if t.PrimaryKey != nil && len(t.PrimaryKey.Columns) > 0 {
		name := keyName(t.PrimaryKey.Name, q.Name, nil, "pkey")
		parts = append(parts, "  CONSTRAINT "+name.Quote()+" PRIMARY KEY ("+quoteList(t.PrimaryKey.Columns)+")")
	}
	for _, u := range t.Uniques {
		name := keyName(u.Name, q.Name, u.Columns, "key")
		parts = append(parts, "  CONSTRAINT "+name.Quote()+" UNIQUE ("+quoteList(u.Columns)+")")
	}
	for _, c := range t.Checks {
		name := c.Name
		if name == "" {
			name = keyName("", q.Name, nil, "check")
		}
		parts = append(parts, "  CONSTRAINT "+name.Quote()+" CHECK ("+c.Expression+")")
	}
	out := []changeset.Statement{ddl("CREATE TABLE "+q.Quote()+" (\n"+strings.Join(parts, ",\n")+"\n)", "Create table "+q.String())}
	if t.Comment != "" {
		out = append(out, ddl("COMMENT ON TABLE "+q.Quote()+" IS "+quoteLiteral(t.Comment), "Comment on "+q.String()))
	}
	for i := range t.Columns {
		if c := &t.Columns[i]; c.Comment != "" {
			out = append(out, ddl("COMMENT ON COLUMN "+q.Quote()+"."+c.Name.Quote()+" IS "+quoteLiteral(c.Comment), "Comment on "+q.String()+"."+c.Name.String()))
		}
	}
	for i := range t.Indexes {
		out = append(out, createIndex(q, &t.Indexes[i]))
	}
	return out
}

// columnDef renders "name type [NOT NULL] [DEFAULT …] [GENERATED … AS IDENTITY]".
func columnDef(c *schema.Column) string {
	b := c.Name.Quote() + " " + c.Type
	if !c.Nullable {
		b += " NOT NULL"
	}
	if c.Default != nil && *c.Default != "" {
		b += " DEFAULT " + *c.Default
	}
	switch c.Identity {
	case schema.IdentityAlways:
		b += " GENERATED ALWAYS AS IDENTITY"
	case schema.IdentityByDefault:
		b += " GENERATED BY DEFAULT AS IDENTITY"
	}
	return b
}

func createIndex(q ident.Qualified, ix *schema.Index) changeset.Statement {
	b := "CREATE "
	if ix.Unique {
		b += "UNIQUE "
	}
	b += "INDEX " + ix.Name.Quote() + " ON " + q.Quote()
	if ix.Method != "" && !strings.EqualFold(ix.Method, "btree") {
		b += " USING " + ix.Method
	}
	if ix.Expression != "" {
		b += " (" + ix.Expression + ")"
	} else {
		b += " (" + quoteList(ix.Columns) + ")"
	}
	if ix.Predicate != "" {
		b += " WHERE " + ix.Predicate
	}
	return ddl(b, "Create index "+ix.Name.String()+" on "+q.String())
}

func foreignKeyDef(fk *schema.ForeignKey) string {
	b := "FOREIGN KEY (" + quoteList(fk.Columns) + ") REFERENCES " + fk.References.Quote() + " (" + quoteList(fk.RefColumns) + ")"
	if a := strings.ToUpper(strings.TrimSpace(fk.OnDelete)); a != "" && a != "NO ACTION" {
		b += " ON DELETE " + a
	}
	if a := strings.ToUpper(strings.TrimSpace(fk.OnUpdate)); a != "" && a != "NO ACTION" {
		b += " ON UPDATE " + a
	}
	return b
}

// keyName returns the given constraint name, or PostgreSQL's conventional
// one: table_col1_col2_suffix, truncated to the 63-byte identifier limit.
func keyName(given, table ident.Name, cols []ident.Name, suffix string) ident.Name {
	if given != "" {
		return given
	}
	parts := []string{table.String()}
	for _, c := range cols {
		parts = append(parts, c.String())
	}
	parts = append(parts, suffix)
	name := strings.Join(parts, "_")
	if len(name) > 63 {
		keep := 63 - len(suffix) - 1
		name = strings.Join(parts[:len(parts)-1], "_")[:keep] + "_" + suffix
	}
	return ident.Name(name)
}

func literalList(values []string) string {
	out := make([]string, len(values))
	for i, v := range values {
		out[i] = quoteLiteral(v)
	}
	return strings.Join(out, ", ")
}

func commentLiteral(s string) string {
	if s == "" {
		return "NULL"
	}
	return quoteLiteral(s)
}

// sized raises a risk level to Warning when the table is large.
func (d dialect) sized(t *schema.Table, base changeset.RiskLevel) changeset.RiskLevel {
	if t != nil && t.RowEstimate >= bigTable && base == changeset.Info {
		return changeset.Warning
	}
	return base
}

func dropRisk(q ident.Qualified, t *schema.Table) changeset.Risk {
	msg := "every row of " + q.String() + " is lost"
	if t.RowEstimate > 0 {
		msg = fmt.Sprintf("about %s rows of %s are lost", humanCount(float64(t.RowEstimate)), q.String())
	}
	return risk(changeset.Danger, changeset.RiskDataLoss, msg, q.String())
}

func (d dialect) enum(sch, name ident.Name) *schema.Enum {
	if d.current == nil {
		return nil
	}
	s := d.current.Schema(sch)
	if s == nil {
		return nil
	}
	return s.Enum(name)
}

func (d dialect) previousDefault(cur *schema.Table, col ident.Name) *string {
	if cur == nil {
		return nil
	}
	if c := cur.Column(col); c != nil && c.Default != nil && *c.Default != "" {
		return c.Default
	}
	return nil
}

func (d dialect) restoreDefault(t string, cur *schema.Table, col ident.Name, obj string) changeset.Statement {
	if prev := d.previousDefault(cur, col); prev != nil {
		return ddl("ALTER TABLE "+t+" ALTER COLUMN "+col.Quote()+" SET DEFAULT "+*prev, "Restore the default of "+obj)
	}
	return ddl("ALTER TABLE "+t+" ALTER COLUMN "+col.Quote()+" DROP DEFAULT", "Remove the default of "+obj)
}

func curUniques(t *schema.Table) []schema.Key {
	if t == nil {
		return nil
	}
	return t.Uniques
}

func findKeyByName(keys []schema.Key, name ident.Name) *schema.Key {
	for i := range keys {
		if keys[i].Name == name {
			return &keys[i]
		}
	}
	return nil
}

func findIndexByName(t *schema.Table, name ident.Name) *schema.Index {
	if t == nil {
		return nil
	}
	for i := range t.Indexes {
		if t.Indexes[i].Name == name {
			return &t.Indexes[i]
		}
	}
	return nil
}

func findFKByName(t *schema.Table, name ident.Name) *schema.ForeignKey {
	if t == nil {
		return nil
	}
	for i := range t.ForeignKeys {
		if t.ForeignKeys[i].Name == name {
			return &t.ForeignKeys[i]
		}
	}
	return nil
}

// binaryCoercible reports type changes PostgreSQL applies without
// rewriting the table: widening varchar, varchar → text, and the same type.
func binaryCoercible(oldT, newT string) bool {
	o, n := parseTypeName(oldT), parseTypeName(newT)
	if o.array != n.array {
		return false
	}
	switch {
	case o.base == n.base:
		if o.base == "varchar" || o.base == "numeric" {
			// Widening or dropping the modifier needs no rewrite.
			return len(n.mods) == 0 || (len(o.mods) > 0 && n.mods[0] >= o.mods[0] && (len(n.mods) < 2 || len(o.mods) < 2 || n.mods[1] >= o.mods[1]))
		}
		return true
	case o.base == "varchar" && n.base == "text":
		return true
	case (o.base == "timestamp" && n.base == "timestamptz") || (o.base == "timestamptz" && n.base == "timestamp"):
		// Rewrite-free only when the session time zone is UTC; assume not.
		return false
	}
	return false
}
