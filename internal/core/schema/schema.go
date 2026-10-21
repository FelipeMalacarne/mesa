// Package schema is the engine-neutral model of a database's structure. The
// live catalog, DBML, the designer, the diff and the DDL generator all read
// and write this model.
package schema

import (
	"sort"

	"github.com/felipemalacarne/mesa/internal/core/ident"
)

// Database is one database: its schemas and what they hold.
type Database struct {
	Name    ident.Name `json:"name"`
	Schemas []Schema   `json:"schemas"`
}

// Schema is a namespace inside a database.
type Schema struct {
	Name      ident.Name `json:"name"`
	Tables    []Table    `json:"tables"`
	Views     []View     `json:"views,omitempty"`
	Enums     []Enum     `json:"enums,omitempty"`
	Functions []Function `json:"functions,omitempty"`
	Sequences []Sequence `json:"sequences,omitempty"`
}

// Table is a base table.
type Table struct {
	Name        ident.Name   `json:"name"`
	Columns     []Column     `json:"columns"`
	PrimaryKey  *Key         `json:"primaryKey,omitempty"`
	Uniques     []Key        `json:"uniques,omitempty"`
	Indexes     []Index      `json:"indexes,omitempty"`
	ForeignKeys []ForeignKey `json:"foreignKeys,omitempty"`
	Checks      []Check      `json:"checks,omitempty"`
	Comment     string       `json:"comment,omitempty"`
	// Stats are read-only facts from the live catalog; the diff ignores them.
	RowEstimate int64 `json:"rowEstimate,omitempty"`
	SizeBytes   int64 `json:"sizeBytes,omitempty"`
}

// Column is a table column. Type is the engine's type name as the catalog
// spells it (int8, timestamptz, varchar(120)); Mesa does not interpret it.
type Column struct {
	Name     ident.Name `json:"name"`
	Type     string     `json:"type"`
	Nullable bool       `json:"nullable"`
	Default  *string    `json:"default,omitempty"` // SQL expression, nil = no default
	Identity Identity   `json:"identity,omitempty"`
	Comment  string     `json:"comment,omitempty"`
}

// Identity marks a generated-identity column.
type Identity string

const (
	IdentityNone      Identity = ""
	IdentityByDefault Identity = "by default"
	IdentityAlways    Identity = "always"
)

// Key is a primary key or unique constraint.
type Key struct {
	Name    ident.Name   `json:"name,omitempty"`
	Columns []ident.Name `json:"columns"`
}

// Index is a secondary index.
type Index struct {
	Name       ident.Name   `json:"name"`
	Columns    []ident.Name `json:"columns,omitempty"`
	Expression string       `json:"expression,omitempty"` // when not a plain column list
	Unique     bool         `json:"unique,omitempty"`
	Method     string       `json:"method,omitempty"` // btree, gin…
	Predicate  string       `json:"predicate,omitempty"`
	// Stats are read-only facts from the live catalog; the diff ignores them.
	SizeBytes int64 `json:"sizeBytes,omitempty"`
	Scans     int64 `json:"scans,omitempty"`
}

// ForeignKey references another table.
type ForeignKey struct {
	Name       ident.Name      `json:"name,omitempty"`
	Columns    []ident.Name    `json:"columns"`
	References ident.Qualified `json:"references"`
	RefColumns []ident.Name    `json:"refColumns"`
	OnDelete   string          `json:"onDelete,omitempty"` // NO ACTION, RESTRICT, CASCADE, SET NULL, SET DEFAULT
	OnUpdate   string          `json:"onUpdate,omitempty"`
}

// Check is a check constraint.
type Check struct {
	Name       ident.Name `json:"name,omitempty"`
	Expression string     `json:"expression"`
}

// View is a view or materialized view.
type View struct {
	Name         ident.Name `json:"name"`
	Definition   string     `json:"definition,omitempty"`
	Columns      []Column   `json:"columns,omitempty"`
	Materialized bool       `json:"materialized,omitempty"`
}

// Enum is an enumerated type.
type Enum struct {
	Name   ident.Name `json:"name"`
	Values []string   `json:"values"`
}

// Function is a routine; Mesa lists them, it does not edit them.
type Function struct {
	Name      ident.Name `json:"name"`
	Signature string     `json:"signature,omitempty"`
	Returns   string     `json:"returns,omitempty"`
	Language  string     `json:"language,omitempty"`
}

// Sequence is a standalone sequence.
type Sequence struct {
	Name ident.Name `json:"name"`
}

// Lookups.

func (d *Database) Schema(name ident.Name) *Schema {
	for i := range d.Schemas {
		if d.Schemas[i].Name == name {
			return &d.Schemas[i]
		}
	}
	return nil
}

func (d *Database) Table(q ident.Qualified) *Table {
	s := d.Schema(q.Schema)
	if s == nil {
		return nil
	}
	return s.Table(q.Name)
}

func (s *Schema) Table(name ident.Name) *Table {
	for i := range s.Tables {
		if s.Tables[i].Name == name {
			return &s.Tables[i]
		}
	}
	return nil
}

func (s *Schema) Enum(name ident.Name) *Enum {
	for i := range s.Enums {
		if s.Enums[i].Name == name {
			return &s.Enums[i]
		}
	}
	return nil
}

func (t *Table) Column(name ident.Name) *Column {
	for i := range t.Columns {
		if t.Columns[i].Name == name {
			return &t.Columns[i]
		}
	}
	return nil
}

// IsPrimaryKey reports whether column c is part of the primary key.
func (t *Table) IsPrimaryKey(c ident.Name) bool {
	if t.PrimaryKey == nil {
		return false
	}
	for _, k := range t.PrimaryKey.Columns {
		if k == c {
			return true
		}
	}
	return false
}

// ForeignKeyFor returns the foreign key whose first column is c, if any.
func (t *Table) ForeignKeyFor(c ident.Name) *ForeignKey {
	for i := range t.ForeignKeys {
		for _, col := range t.ForeignKeys[i].Columns {
			if col == c {
				return &t.ForeignKeys[i]
			}
		}
	}
	return nil
}

// Normalize sorts everything by name so two models compare and print stably.
func (d *Database) Normalize() {
	sort.Slice(d.Schemas, func(i, j int) bool { return d.Schemas[i].Name < d.Schemas[j].Name })
	for si := range d.Schemas {
		s := &d.Schemas[si]
		sort.Slice(s.Tables, func(i, j int) bool { return s.Tables[i].Name < s.Tables[j].Name })
		sort.Slice(s.Views, func(i, j int) bool { return s.Views[i].Name < s.Views[j].Name })
		sort.Slice(s.Enums, func(i, j int) bool { return s.Enums[i].Name < s.Enums[j].Name })
		sort.Slice(s.Functions, func(i, j int) bool { return s.Functions[i].Name < s.Functions[j].Name })
		sort.Slice(s.Sequences, func(i, j int) bool { return s.Sequences[i].Name < s.Sequences[j].Name })
		for ti := range s.Tables {
			t := &s.Tables[ti]
			sort.Slice(t.Indexes, func(i, j int) bool { return t.Indexes[i].Name < t.Indexes[j].Name })
			sort.Slice(t.ForeignKeys, func(i, j int) bool { return t.ForeignKeys[i].Name < t.ForeignKeys[j].Name })
			sort.Slice(t.Uniques, func(i, j int) bool { return t.Uniques[i].Name < t.Uniques[j].Name })
			sort.Slice(t.Checks, func(i, j int) bool { return t.Checks[i].Name < t.Checks[j].Name })
		}
	}
}

// Counts summarises a database for status lines.
type Counts struct {
	Schemas, Tables, Views, Enums, Functions, ForeignKeys int
}

func (d *Database) Counts() Counts {
	var c Counts
	c.Schemas = len(d.Schemas)
	for _, s := range d.Schemas {
		c.Tables += len(s.Tables)
		c.Views += len(s.Views)
		c.Enums += len(s.Enums)
		c.Functions += len(s.Functions)
		for _, t := range s.Tables {
			c.ForeignKeys += len(t.ForeignKeys)
		}
	}
	return c
}
