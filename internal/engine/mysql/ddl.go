package mysql

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/felipemalacarne/mesa/internal/core/changeset"
	"github.com/felipemalacarne/mesa/internal/core/errs"
	"github.com/felipemalacarne/mesa/internal/core/ident"
	"github.com/felipemalacarne/mesa/internal/core/schema"
)

// column returns the working copy of a column: the last definition this
// plan gave it, else the live one. nil when neither is known.
func (d *dialect) column(q ident.Qualified, name ident.Name) *schema.Column {
	key := [2]ident.Name{q.Name, name}
	if c, ok := d.columns[key]; ok {
		return c
	}
	if t := d.table(q); t != nil {
		if c := t.Column(name); c != nil {
			cp := *c
			d.columns[key] = &cp
			return &cp
		}
	}
	return nil
}

func (d *dialect) setColumn(q ident.Qualified, c schema.Column) {
	d.columns[[2]ident.Name{q.Name, c.Name}] = &c
}

// modify redefines a column from its working copy after edit, returning
// the statement and the one that restores the previous definition.
func (d *dialect) modify(q ident.Qualified, name ident.Name, describe string, edit func(*schema.Column)) (up, down changeset.Statement, err error) {
	cur := d.column(q, name)
	if cur == nil {
		return up, down, errs.Newf(errs.Invalid, "column %s.%s is not known; MySQL needs its full definition to change it", q, name)
	}
	before := *cur
	after := *cur
	edit(&after)
	d.setColumn(q, after)
	t := table(q)
	up = ddl("ALTER TABLE "+t+" MODIFY COLUMN "+columnDef(&after), describe)
	down = ddl("ALTER TABLE "+t+" MODIFY COLUMN "+columnDef(&before), "Restore the definition of "+q.String()+"."+name.String())
	return up, down, nil
}

// schemaChange renders one structural change as DDL, its reverse, and the
// risks a reviewer should know about.
func (d *dialect) schemaChange(c schema.Change) (up, down []changeset.Statement, risks []changeset.Risk, err error) {
	t := table(c.Table)
	cur := d.table(c.Table)
	add := func(s changeset.Statement) { up = append(up, s) }
	rev := func(s changeset.Statement) { down = append(down, s) }
	warn := func(level changeset.RiskLevel, code changeset.RiskCode, msg, object string) {
		risks = append(risks, risk(level, code, msg, object))
	}
	irreversible := func(msg, object string) { warn(changeset.Info, changeset.RiskIrreversible, msg, object) }
	modify := func(name ident.Name, edit func(*schema.Column)) error {
		u, dn, err := d.modify(c.Table, name, c.Describe(), edit)
		if err != nil {
			return err
		}
		add(u)
		rev(dn)
		return nil
	}
	obj := func(col ident.Name) string { return c.Table.String() + "." + col.String() }

	switch c.Kind {
	case schema.CreateSchema, schema.DropSchema:
		return nil, nil, nil, errs.New(errs.Unsupported, "MySQL has no schemas inside a database; create or drop a database instead")

	case schema.CreateEnum, schema.DropEnum, schema.AddEnumValue:
		return nil, nil, nil, errs.New(errs.Unsupported, "MySQL has no enum types: write enum('a','b') as the column type instead")

	case schema.CreateTable:
		if c.NewTable == nil {
			return nil, nil, nil, errs.New(errs.Invalid, "create_table needs the table")
		}
		add(createTable(c.Table, c.NewTable))
		for _, col := range c.NewTable.Columns {
			d.setColumn(c.Table, col)
		}
		rev(ddl("DROP TABLE "+t, "Drop table "+c.Table.String()))

	case schema.DropTable:
		add(ddl("DROP TABLE "+t, c.Describe()))
		if cur != nil {
			rev(createTable(c.Table, cur))
			for _, fk := range cur.ForeignKeys {
				rev(ddl("ALTER TABLE "+t+" ADD CONSTRAINT "+quote(fk.Name)+" "+foreignKeyDef(&fk), "Restore foreign key "+fk.Name.String()))
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
		rev(ddl("ALTER TABLE "+t+" DROP COLUMN "+quote(col.Name), "Drop column "+obj(col.Name)))
		d.setColumn(c.Table, *col)
		if !col.Nullable && col.Default == nil && col.Identity == schema.IdentityNone {
			warn(changeset.Warning, changeset.RiskFullScan, "existing rows of "+c.Table.String()+" get the type's zero value in "+col.Name.String()+", or the change fails in strict mode", obj(col.Name))
		}

	case schema.DropColumn:
		add(ddl("ALTER TABLE "+t+" DROP COLUMN "+quote(c.Name), c.Describe()))
		old := c.OldColumn
		if old == nil {
			old = d.column(c.Table, c.Name)
		}
		if old != nil {
			rev(ddl("ALTER TABLE "+t+" ADD COLUMN "+columnDef(old), "Re-add column "+obj(c.Name)+" (its values are gone)"))
		}
		warn(changeset.Danger, changeset.RiskDataLoss, "the values of "+obj(c.Name)+" are lost", obj(c.Name))

	case schema.AlterColumnType:
		if c.Column == nil {
			return nil, nil, nil, errs.New(errs.Invalid, "alter_column_type needs the column")
		}
		col := c.Column
		if d.column(c.Table, col.Name) == nil && c.OldColumn != nil {
			d.setColumn(c.Table, *c.OldColumn)
		}
		var oldType string
		if old := d.column(c.Table, col.Name); old != nil {
			oldType = old.Type
		}
		if err := modify(col.Name, func(x *schema.Column) { x.Type = col.Type }); err != nil {
			return nil, nil, nil, err
		}
		if narrowing(oldType, col.Type) {
			warn(changeset.Danger, changeset.RiskDataLoss, "changing "+obj(col.Name)+" from "+oldType+" to "+col.Type+" can truncate or reject existing values", obj(col.Name))
		}
		warn(d.sized(cur), changeset.RiskTableRewrite, "changing the type of "+obj(col.Name)+" copies "+c.Table.String()+"; writes wait meanwhile", obj(col.Name))

	case schema.SetNotNull:
		if err := modify(c.Name, func(x *schema.Column) { x.Nullable = false }); err != nil {
			return nil, nil, nil, err
		}
		warn(d.sized(cur), changeset.RiskFullScan, "every row of "+c.Table.String()+" is checked; the change fails if "+obj(c.Name)+" holds a null", obj(c.Name))

	case schema.DropNotNull:
		if err := modify(c.Name, func(x *schema.Column) { x.Nullable = true }); err != nil {
			return nil, nil, nil, err
		}

	case schema.SetDefault:
		if c.Column == nil || c.Column.Default == nil {
			return nil, nil, nil, errs.New(errs.Invalid, "set_default needs the column and its default")
		}
		def := *c.Column.Default
		add(ddl("ALTER TABLE "+t+" ALTER COLUMN "+quote(c.Column.Name)+" SET DEFAULT "+defaultExpr(def), c.Describe()))
		rev(d.restoreDefault(c.Table, c.Column.Name))
		if col := d.column(c.Table, c.Column.Name); col != nil {
			col.Default = &def
		}

	case schema.DropDefault:
		add(ddl("ALTER TABLE "+t+" ALTER COLUMN "+quote(c.Name)+" DROP DEFAULT", c.Describe()))
		rev(d.restoreDefault(c.Table, c.Name))
		if col := d.column(c.Table, c.Name); col != nil {
			col.Default = nil
		}

	case schema.AddPrimaryKey:
		if c.Key == nil {
			return nil, nil, nil, errs.New(errs.Invalid, "add_primary_key needs the key")
		}
		add(ddl("ALTER TABLE "+t+" ADD PRIMARY KEY ("+quoteList(c.Key.Columns)+")", c.Describe()))
		rev(ddl("ALTER TABLE "+t+" DROP PRIMARY KEY", "Drop the primary key of "+c.Table.String()))
		warn(d.sized(cur), changeset.RiskTableRewrite, "adding a primary key rebuilds "+c.Table.String(), c.Table.String())

	case schema.DropPrimaryKey:
		add(ddl("ALTER TABLE "+t+" DROP PRIMARY KEY", c.Describe()))
		if cur != nil && cur.PrimaryKey != nil {
			rev(ddl("ALTER TABLE "+t+" ADD PRIMARY KEY ("+quoteList(cur.PrimaryKey.Columns)+")", "Restore the primary key of "+c.Table.String()))
		} else {
			irreversible("the columns of the primary key of "+c.Table.String()+" are not known", c.Table.String())
		}
		warn(d.sized(cur), changeset.RiskTableRewrite, "dropping the primary key rebuilds "+c.Table.String(), c.Table.String())

	case schema.AddUnique:
		if c.Key == nil {
			return nil, nil, nil, errs.New(errs.Invalid, "add_unique needs the key")
		}
		name := keyName(c.Key.Name, c.Table.Name, c.Key.Columns, "key")
		add(ddl("ALTER TABLE "+t+" ADD CONSTRAINT "+quote(name)+" UNIQUE ("+quoteList(c.Key.Columns)+")", c.Describe()))
		rev(ddl("ALTER TABLE "+t+" DROP INDEX "+quote(name), "Drop unique "+name.String()))
		warn(d.sized(cur), changeset.RiskFullScan, "building the unique index reads every row of "+c.Table.String()+"; duplicates make it fail", c.Table.String())

	case schema.DropUnique:
		add(ddl("ALTER TABLE "+t+" DROP INDEX "+quote(c.Name), c.Describe()))
		if k := findKeyByName(cur, c.Name); k != nil {
			rev(ddl("ALTER TABLE "+t+" ADD CONSTRAINT "+quote(c.Name)+" UNIQUE ("+quoteList(k.Columns)+")", "Restore unique "+c.Name.String()))
		} else {
			irreversible("the columns of unique constraint "+c.Name.String()+" are not known", c.Name.String())
		}

	case schema.AddCheck:
		if c.Check == nil {
			return nil, nil, nil, errs.New(errs.Invalid, "add_check needs the check")
		}
		name := c.Check.Name
		if name == "" {
			name = keyName("", c.Table.Name, nil, "chk")
		}
		add(ddl("ALTER TABLE "+t+" ADD CONSTRAINT "+quote(name)+" CHECK ("+c.Check.Expression+")", c.Describe()))
		rev(ddl("ALTER TABLE "+t+" DROP CHECK "+quote(name), "Drop check "+name.String()))
		warn(d.sized(cur), changeset.RiskFullScan, "every row of "+c.Table.String()+" is checked against the constraint", c.Table.String())

	case schema.DropCheck:
		add(ddl("ALTER TABLE "+t+" DROP CHECK "+quote(c.Name), c.Describe()))
		if cur != nil {
			for _, ch := range cur.Checks {
				if ch.Name == c.Name {
					rev(ddl("ALTER TABLE "+t+" ADD CONSTRAINT "+quote(c.Name)+" CHECK ("+ch.Expression+")", "Restore check "+c.Name.String()))
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
		st, err := createIndex(c.Table, c.Index)
		if err != nil {
			return nil, nil, nil, err
		}
		add(st)
		rev(ddl("DROP INDEX "+quote(c.Index.Name)+" ON "+t, "Drop index "+c.Index.Name.String()))
		warn(d.sized(cur), changeset.RiskFullScan, "building the index reads every row of "+c.Table.String(), c.Index.Name.String())

	case schema.DropIndex:
		add(ddl("DROP INDEX "+quote(c.Name)+" ON "+t, c.Describe()))
		if ix := findIndexByName(cur, c.Name); ix != nil {
			if st, err := createIndex(c.Table, ix); err == nil {
				rev(st)
			}
		} else {
			irreversible("the definition of index "+c.Name.String()+" is not known", c.Name.String())
		}

	case schema.AddForeignKey:
		if c.ForeignKey == nil {
			return nil, nil, nil, errs.New(errs.Invalid, "add_foreign_key needs the foreign key")
		}
		fk := c.ForeignKey
		name := keyName(fk.Name, c.Table.Name, fk.Columns, "fkey")
		add(ddl("ALTER TABLE "+t+" ADD CONSTRAINT "+quote(name)+" "+foreignKeyDef(fk), c.Describe()))
		rev(ddl("ALTER TABLE "+t+" DROP FOREIGN KEY "+quote(name), "Drop foreign key "+name.String()))
		if cur != nil {
			warn(d.sized(cur), changeset.RiskFullScan, "every row of "+c.Table.String()+" is checked against "+fk.References.String(), name.String())
		}

	case schema.DropForeignKey:
		add(ddl("ALTER TABLE "+t+" DROP FOREIGN KEY "+quote(c.Name), c.Describe()))
		if fk := findFKByName(cur, c.Name); fk != nil {
			rev(ddl("ALTER TABLE "+t+" ADD CONSTRAINT "+quote(c.Name)+" "+foreignKeyDef(fk), "Restore foreign key "+c.Name.String()))
		} else {
			irreversible("the definition of foreign key "+c.Name.String()+" is not known", c.Name.String())
		}

	case schema.SetTableComment:
		add(ddl("ALTER TABLE "+t+" COMMENT = "+quoteLiteral(c.Comment), c.Describe()))
		if cur != nil {
			rev(ddl("ALTER TABLE "+t+" COMMENT = "+quoteLiteral(cur.Comment), "Restore the comment of "+c.Table.String()))
		}

	case schema.SetColumnComment:
		if c.Column == nil {
			return nil, nil, nil, errs.New(errs.Invalid, "set_column_comment needs the column")
		}
		if d.column(c.Table, c.Column.Name) == nil {
			d.setColumn(c.Table, *c.Column)
		}
		if err := modify(c.Column.Name, func(x *schema.Column) { x.Comment = c.Comment }); err != nil {
			return nil, nil, nil, err
		}

	default:
		return nil, nil, nil, errs.Newf(errs.Unsupported, "change kind %q is not supported by the MySQL dialect", c.Kind)
	}
	return up, down, risks, nil
}

// createTable renders CREATE TABLE with columns, keys, checks and indexes.
// Foreign keys are separate changes so every table exists before them.
func createTable(q ident.Qualified, t *schema.Table) changeset.Statement {
	var parts []string
	for i := range t.Columns {
		parts = append(parts, "  "+columnDef(&t.Columns[i]))
	}
	if t.PrimaryKey != nil && len(t.PrimaryKey.Columns) > 0 {
		parts = append(parts, "  PRIMARY KEY ("+quoteList(t.PrimaryKey.Columns)+")")
	}
	for _, u := range t.Uniques {
		name := keyName(u.Name, q.Name, u.Columns, "key")
		parts = append(parts, "  CONSTRAINT "+quote(name)+" UNIQUE ("+quoteList(u.Columns)+")")
	}
	for _, c := range t.Checks {
		name := c.Name
		if name == "" {
			name = keyName("", q.Name, nil, "chk")
		}
		parts = append(parts, "  CONSTRAINT "+quote(name)+" CHECK ("+c.Expression+")")
	}
	for i := range t.Indexes {
		ix := &t.Indexes[i]
		parts = append(parts, "  "+indexDef(ix))
	}
	sql := "CREATE TABLE " + table(q) + " (\n" + strings.Join(parts, ",\n") + "\n)"
	if t.Comment != "" {
		sql += " COMMENT = " + quoteLiteral(t.Comment)
	}
	return ddl(sql, "Create table "+q.String())
}

// columnDef renders "name type [NOT NULL] [DEFAULT …] [AUTO_INCREMENT] [COMMENT …]".
func columnDef(c *schema.Column) string {
	b := quote(c.Name) + " " + mysqlType(c.Type)
	if c.Nullable {
		b += " NULL"
	} else {
		b += " NOT NULL"
	}
	if c.Default != nil && *c.Default != "" {
		b += " DEFAULT " + defaultExpr(*c.Default)
	}
	if c.Identity != schema.IdentityNone {
		b += " AUTO_INCREMENT"
	}
	if c.Comment != "" {
		b += " COMMENT " + quoteLiteral(c.Comment)
	}
	return b
}

// defaultExpr wraps expression defaults in the parentheses MySQL requires;
// literals, NULL and CURRENT_TIMESTAMP stand as they are.
func defaultExpr(def string) string {
	d := strings.TrimSpace(def)
	u := strings.ToUpper(d)
	switch {
	case d == "", strings.HasPrefix(d, "("), strings.HasPrefix(d, "'"), strings.HasPrefix(d, "\""),
		u == "NULL", u == "TRUE", u == "FALSE", strings.HasPrefix(u, "CURRENT_TIMESTAMP"), strings.HasPrefix(u, "B'"), strings.HasPrefix(u, "X'"):
		return d
	}
	if _, err := strconv.ParseFloat(d, 64); err == nil {
		return d
	}
	if u == "NOW()" || u == "LOCALTIMESTAMP" || u == "LOCALTIMESTAMP()" {
		return "CURRENT_TIMESTAMP"
	}
	return "(" + d + ")"
}

// indexDef renders the "[UNIQUE] INDEX name (…) [USING …]" part.
func indexDef(ix *schema.Index) string {
	b := "INDEX "
	if ix.Unique {
		b = "UNIQUE INDEX "
	}
	b += quote(ix.Name) + " (" + indexKeys(ix) + ")"
	if m := strings.ToUpper(ix.Method); m == "HASH" || m == "BTREE" {
		b += " USING " + m
	}
	return b
}

func indexKeys(ix *schema.Index) string {
	if ix.Expression != "" {
		return ix.Expression
	}
	return quoteList(ix.Columns)
}

func createIndex(q ident.Qualified, ix *schema.Index) (changeset.Statement, error) {
	if ix.Predicate != "" {
		return changeset.Statement{}, errs.Newf(errs.Unsupported, "MySQL has no partial indexes (index %s has a WHERE clause)", ix.Name)
	}
	b := "CREATE "
	if ix.Unique {
		b += "UNIQUE "
	}
	b += "INDEX " + quote(ix.Name) + " ON " + table(q) + " (" + indexKeys(ix) + ")"
	switch m := strings.ToUpper(ix.Method); m {
	case "", "BTREE":
	case "HASH":
		b += " USING HASH"
	case "FULLTEXT", "SPATIAL":
		b = "CREATE " + m + " INDEX " + quote(ix.Name) + " ON " + table(q) + " (" + indexKeys(ix) + ")"
	}
	return ddl(b, "Create index "+ix.Name.String()+" on "+q.String()), nil
}

func foreignKeyDef(fk *schema.ForeignKey) string {
	b := "FOREIGN KEY (" + quoteList(fk.Columns) + ") REFERENCES " + table(fk.References) + " (" + quoteList(fk.RefColumns) + ")"
	if a := strings.ToUpper(strings.TrimSpace(fk.OnDelete)); a != "" && a != "NO ACTION" {
		b += " ON DELETE " + a
	}
	if a := strings.ToUpper(strings.TrimSpace(fk.OnUpdate)); a != "" && a != "NO ACTION" {
		b += " ON UPDATE " + a
	}
	return b
}

// keyName returns the given name, or table_col1_col2_suffix within MySQL's
// 64-character identifier limit.
func keyName(given, tbl ident.Name, cols []ident.Name, suffix string) ident.Name {
	if given != "" {
		return given
	}
	parts := []string{tbl.String()}
	for _, c := range cols {
		parts = append(parts, c.String())
	}
	name := strings.Join(parts, "_")
	if len(name)+len(suffix)+1 > 64 {
		name = name[:64-len(suffix)-1]
	}
	return ident.Name(name + "_" + suffix)
}

// sized is Info, raised to Warning when the table is large.
func (d *dialect) sized(t *schema.Table) changeset.RiskLevel {
	if t != nil && t.RowEstimate >= bigTable {
		return changeset.Warning
	}
	return changeset.Info
}

func dropRisk(q ident.Qualified, t *schema.Table) changeset.Risk {
	msg := "every row of " + q.String() + " is lost"
	if t.RowEstimate > 0 {
		msg = fmt.Sprintf("about %s rows of %s are lost", humanCount(float64(t.RowEstimate)), q.String())
	}
	return risk(changeset.Danger, changeset.RiskDataLoss, msg, q.String())
}

func (d *dialect) restoreDefault(q ident.Qualified, col ident.Name) changeset.Statement {
	t := table(q)
	obj := q.String() + "." + col.String()
	if c := d.column(q, col); c != nil && c.Default != nil && *c.Default != "" {
		return ddl("ALTER TABLE "+t+" ALTER COLUMN "+quote(col)+" SET DEFAULT "+defaultExpr(*c.Default), "Restore the default of "+obj)
	}
	return ddl("ALTER TABLE "+t+" ALTER COLUMN "+quote(col)+" DROP DEFAULT", "Remove the default of "+obj)
}

func findKeyByName(t *schema.Table, name ident.Name) *schema.Key {
	if t == nil {
		return nil
	}
	for i := range t.Uniques {
		if t.Uniques[i].Name == name {
			return &t.Uniques[i]
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

func itoa(n int64) string { return strconv.FormatInt(n, 10) }
