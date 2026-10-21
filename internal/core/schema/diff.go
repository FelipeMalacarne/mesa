package schema

import (
	"github.com/felipemalacarne/mesa/internal/core/ident"
)

// ChangeKind names one structural change.
type ChangeKind string

const (
	CreateSchema     ChangeKind = "create_schema"
	DropSchema       ChangeKind = "drop_schema"
	CreateEnum       ChangeKind = "create_enum"
	DropEnum         ChangeKind = "drop_enum"
	AddEnumValue     ChangeKind = "add_enum_value"
	CreateTable      ChangeKind = "create_table"
	DropTable        ChangeKind = "drop_table"
	AddColumn        ChangeKind = "add_column"
	DropColumn       ChangeKind = "drop_column"
	AlterColumnType  ChangeKind = "alter_column_type"
	SetNotNull       ChangeKind = "set_not_null"
	DropNotNull      ChangeKind = "drop_not_null"
	SetDefault       ChangeKind = "set_default"
	DropDefault      ChangeKind = "drop_default"
	AddPrimaryKey    ChangeKind = "add_primary_key"
	DropPrimaryKey   ChangeKind = "drop_primary_key"
	AddUnique        ChangeKind = "add_unique"
	DropUnique       ChangeKind = "drop_unique"
	AddCheck         ChangeKind = "add_check"
	DropCheck        ChangeKind = "drop_check"
	CreateIndex      ChangeKind = "create_index"
	DropIndex        ChangeKind = "drop_index"
	AddForeignKey    ChangeKind = "add_foreign_key"
	DropForeignKey   ChangeKind = "drop_foreign_key"
	SetTableComment  ChangeKind = "set_table_comment"
	SetColumnComment ChangeKind = "set_column_comment"
)

// Change is one structural difference between two models, with the data the
// dialect needs to express it. Only the fields relevant to Kind are set.
type Change struct {
	Kind   ChangeKind      `json:"kind"`
	Schema ident.Name      `json:"schema,omitempty"`
	Table  ident.Qualified `json:"table,omitempty"`

	NewTable   *Table      `json:"newTable,omitempty"`  // CreateTable
	Column     *Column     `json:"column,omitempty"`    // AddColumn, AlterColumnType, Set/DropDefault, comments
	OldColumn  *Column     `json:"oldColumn,omitempty"` // AlterColumnType, DropColumn
	Name       ident.Name  `json:"name,omitempty"`      // column / constraint / index / enum name
	Key        *Key        `json:"key,omitempty"`
	Index      *Index      `json:"index,omitempty"`
	ForeignKey *ForeignKey `json:"foreignKey,omitempty"`
	Check      *Check      `json:"check,omitempty"`
	Enum       *Enum       `json:"enum,omitempty"`
	Values     []string    `json:"values,omitempty"` // AddEnumValue
	Comment    string      `json:"comment,omitempty"`
}

// Describe gives a one-line, person-facing label for a change.
func (c Change) Describe() string {
	switch c.Kind {
	case CreateSchema:
		return "Create schema " + c.Schema.String()
	case DropSchema:
		return "Drop schema " + c.Schema.String()
	case CreateEnum:
		return "Create type " + c.Schema.String() + "." + c.Enum.Name.String()
	case DropEnum:
		return "Drop type " + c.Schema.String() + "." + c.Name.String()
	case AddEnumValue:
		return "Add values to type " + c.Schema.String() + "." + c.Name.String()
	case CreateTable:
		return "Create table " + c.Table.String()
	case DropTable:
		return "Drop table " + c.Table.String()
	case AddColumn:
		return "Add column " + c.Table.String() + "." + c.Column.Name.String()
	case DropColumn:
		return "Drop column " + c.Table.String() + "." + c.Name.String()
	case AlterColumnType:
		return "Change type of " + c.Table.String() + "." + c.Column.Name.String() + " to " + c.Column.Type
	case SetNotNull:
		return "Set " + c.Table.String() + "." + c.Name.String() + " not null"
	case DropNotNull:
		return "Allow null in " + c.Table.String() + "." + c.Name.String()
	case SetDefault:
		return "Set default of " + c.Table.String() + "." + c.Column.Name.String()
	case DropDefault:
		return "Drop default of " + c.Table.String() + "." + c.Name.String()
	case AddPrimaryKey:
		return "Add primary key on " + c.Table.String()
	case DropPrimaryKey:
		return "Drop primary key of " + c.Table.String()
	case AddUnique:
		return "Add unique constraint on " + c.Table.String()
	case DropUnique:
		return "Drop unique constraint " + c.Name.String() + " on " + c.Table.String()
	case AddCheck:
		return "Add check on " + c.Table.String()
	case DropCheck:
		return "Drop check " + c.Name.String() + " on " + c.Table.String()
	case CreateIndex:
		return "Create index " + c.Index.Name.String() + " on " + c.Table.String()
	case DropIndex:
		return "Drop index " + c.Name.String()
	case AddForeignKey:
		return "Add foreign key " + c.Table.String() + " → " + c.ForeignKey.References.String()
	case DropForeignKey:
		return "Drop foreign key " + c.Name.String() + " on " + c.Table.String()
	case SetTableComment:
		return "Comment on table " + c.Table.String()
	case SetColumnComment:
		return "Comment on column " + c.Table.String() + "." + c.Column.Name.String()
	}
	return string(c.Kind)
}

// DiffOptions let an engine say when two spellings mean the same thing, so
// "varchar(20)" against "character varying(20)" is not a change.
type DiffOptions struct {
	// SameType reports whether two column type spellings name the same type.
	SameType func(a, b string) bool
	// SameDefault reports whether two default expressions are equivalent.
	SameDefault func(a, b string) bool
}

func (o DiffOptions) sameType(a, b string) bool {
	if o.SameType != nil {
		return o.SameType(a, b)
	}
	return a == b
}

func (o DiffOptions) sameDefault(a, b string) bool {
	if o.SameDefault != nil {
		return o.SameDefault(a, b)
	}
	return a == b
}

// Diff computes the changes that turn from into to, comparing spellings
// literally. Statistics are ignored.
func Diff(from, to *Database) []Change { return DiffWith(from, to, DiffOptions{}) }

// DiffWith computes the changes that turn from into to. Statistics are
// ignored. The result is ordered so it can run top to bottom: schemas and
// enums first, drops of dependents before drops of their targets, foreign
// keys last.
func DiffWith(from, to *Database, opts DiffOptions) []Change {
	var pre, mid, fks, post []Change

	// Schemas.
	for _, ts := range to.Schemas {
		if from.Schema(ts.Name) == nil {
			pre = append(pre, Change{Kind: CreateSchema, Schema: ts.Name})
		}
	}
	// Enums.
	for _, ts := range to.Schemas {
		fs := from.Schema(ts.Name)
		for _, te := range ts.Enums {
			te := te
			var fe *Enum
			if fs != nil {
				fe = fs.Enum(te.Name)
			}
			if fe == nil {
				pre = append(pre, Change{Kind: CreateEnum, Schema: ts.Name, Enum: &te})
			} else if added := missingValues(fe.Values, te.Values); len(added) > 0 {
				pre = append(pre, Change{Kind: AddEnumValue, Schema: ts.Name, Name: te.Name, Values: added})
			}
		}
	}

	// Tables.
	for _, ts := range to.Schemas {
		fs := from.Schema(ts.Name)
		for _, tt := range ts.Tables {
			tt := tt
			q := ident.Q(ts.Name, tt.Name)
			var ft *Table
			if fs != nil {
				ft = fs.Table(tt.Name)
			}
			if ft == nil {
				mid = append(mid, Change{Kind: CreateTable, Table: q, NewTable: &tt})
				for _, fk := range tt.ForeignKeys {
					fk := fk
					fks = append(fks, Change{Kind: AddForeignKey, Table: q, ForeignKey: &fk})
				}
				continue
			}
			m, f := diffTable(q, ft, &tt, opts)
			mid = append(mid, m...)
			fks = append(fks, f...)
		}
	}
	// Dropped tables (after their foreign keys are gone).
	for _, fs := range from.Schemas {
		ts := to.Schema(fs.Name)
		for _, ft := range fs.Tables {
			if ts == nil || ts.Table(ft.Name) == nil {
				q := ident.Q(fs.Name, ft.Name)
				for _, fk := range ft.ForeignKeys {
					pre = append(pre, Change{Kind: DropForeignKey, Table: q, Name: fk.Name})
				}
				post = append(post, Change{Kind: DropTable, Table: q})
			}
		}
	}
	// Dropped enums and schemas.
	for _, fs := range from.Schemas {
		ts := to.Schema(fs.Name)
		for _, fe := range fs.Enums {
			if ts == nil || ts.Enum(fe.Name) == nil {
				post = append(post, Change{Kind: DropEnum, Schema: fs.Name, Name: fe.Name})
			}
		}
		if ts == nil {
			post = append(post, Change{Kind: DropSchema, Schema: fs.Name})
		}
	}

	out := make([]Change, 0, len(pre)+len(mid)+len(fks)+len(post))
	out = append(out, pre...)
	out = append(out, mid...)
	out = append(out, fks...)
	out = append(out, post...)
	return out
}

func diffTable(q ident.Qualified, from, to *Table, opts DiffOptions) (changes, fks []Change) {
	// Foreign keys dropped first so columns they use can go.
	for _, ffk := range from.ForeignKeys {
		if findFK(to.ForeignKeys, ffk) == nil {
			changes = append(changes, Change{Kind: DropForeignKey, Table: q, Name: ffk.Name})
		}
	}
	for _, fi := range from.Indexes {
		if findIndex(to.Indexes, fi) == nil {
			changes = append(changes, Change{Kind: DropIndex, Table: q, Name: fi.Name})
		}
	}
	for _, fu := range from.Uniques {
		if findKey(to.Uniques, fu) == nil {
			changes = append(changes, Change{Kind: DropUnique, Table: q, Name: fu.Name})
		}
	}
	for _, fc := range from.Checks {
		if findCheck(to.Checks, fc) == nil {
			changes = append(changes, Change{Kind: DropCheck, Table: q, Name: fc.Name})
		}
	}
	if from.PrimaryKey != nil && (to.PrimaryKey == nil || !sameNames(from.PrimaryKey.Columns, to.PrimaryKey.Columns)) {
		changes = append(changes, Change{Kind: DropPrimaryKey, Table: q, Name: from.PrimaryKey.Name})
	}
	// Columns.
	for _, fc := range from.Columns {
		if to.Column(fc.Name) == nil {
			fc := fc
			changes = append(changes, Change{Kind: DropColumn, Table: q, Name: fc.Name, OldColumn: &fc})
		}
	}
	for _, tc := range to.Columns {
		tc := tc
		fc := from.Column(tc.Name)
		if fc == nil {
			changes = append(changes, Change{Kind: AddColumn, Table: q, Column: &tc})
			continue
		}
		if !opts.sameType(fc.Type, tc.Type) {
			old := *fc
			changes = append(changes, Change{Kind: AlterColumnType, Table: q, Column: &tc, OldColumn: &old})
		}
		switch {
		case fc.Nullable && !tc.Nullable:
			changes = append(changes, Change{Kind: SetNotNull, Table: q, Name: tc.Name})
		case !fc.Nullable && tc.Nullable:
			changes = append(changes, Change{Kind: DropNotNull, Table: q, Name: tc.Name})
		}
		switch {
		case tc.Default == nil && fc.Default != nil:
			changes = append(changes, Change{Kind: DropDefault, Table: q, Name: tc.Name})
		case tc.Default != nil && (fc.Default == nil || !opts.sameDefault(*fc.Default, *tc.Default)):
			changes = append(changes, Change{Kind: SetDefault, Table: q, Column: &tc})
		}
		if fc.Comment != tc.Comment {
			changes = append(changes, Change{Kind: SetColumnComment, Table: q, Column: &tc, Comment: tc.Comment})
		}
	}
	if to.PrimaryKey != nil && (from.PrimaryKey == nil || !sameNames(from.PrimaryKey.Columns, to.PrimaryKey.Columns)) {
		changes = append(changes, Change{Kind: AddPrimaryKey, Table: q, Key: to.PrimaryKey})
	}
	for _, tu := range to.Uniques {
		if findKey(from.Uniques, tu) == nil {
			tu := tu
			changes = append(changes, Change{Kind: AddUnique, Table: q, Key: &tu})
		}
	}
	for _, tc := range to.Checks {
		if findCheck(from.Checks, tc) == nil {
			tc := tc
			changes = append(changes, Change{Kind: AddCheck, Table: q, Check: &tc})
		}
	}
	for _, ti := range to.Indexes {
		if findIndex(from.Indexes, ti) == nil {
			ti := ti
			changes = append(changes, Change{Kind: CreateIndex, Table: q, Index: &ti})
		}
	}
	for _, tfk := range to.ForeignKeys {
		if findFK(from.ForeignKeys, tfk) == nil {
			tfk := tfk
			fks = append(fks, Change{Kind: AddForeignKey, Table: q, ForeignKey: &tfk})
		}
	}
	if from.Comment != to.Comment {
		changes = append(changes, Change{Kind: SetTableComment, Table: q, Comment: to.Comment})
	}
	return changes, fks
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

func findKey(list []Key, k Key) *Key {
	for i := range list {
		if sameNames(list[i].Columns, k.Columns) {
			return &list[i]
		}
	}
	return nil
}

func findIndex(list []Index, x Index) *Index {
	for i := range list {
		c := list[i]
		if c.Name == x.Name && sameNames(c.Columns, x.Columns) && c.Unique == x.Unique && c.Expression == x.Expression && c.Predicate == x.Predicate && methodEq(c.Method, x.Method) {
			return &list[i]
		}
	}
	return nil
}

func methodEq(a, b string) bool {
	if a == "" {
		a = "btree"
	}
	if b == "" {
		b = "btree"
	}
	return a == b
}

func findFK(list []ForeignKey, fk ForeignKey) *ForeignKey {
	for i := range list {
		c := list[i]
		if sameNames(c.Columns, fk.Columns) && c.References == fk.References && sameNames(c.RefColumns, fk.RefColumns) && actionEq(c.OnDelete, fk.OnDelete) && actionEq(c.OnUpdate, fk.OnUpdate) {
			return &list[i]
		}
	}
	return nil
}

func actionEq(a, b string) bool {
	norm := func(s string) string {
		if s == "" {
			return "NO ACTION"
		}
		return s
	}
	return norm(a) == norm(b)
}

func findCheck(list []Check, c Check) *Check {
	for i := range list {
		if list[i].Expression == c.Expression {
			return &list[i]
		}
	}
	return nil
}

func missingValues(have, want []string) []string {
	set := map[string]bool{}
	for _, v := range have {
		set[v] = true
	}
	var out []string
	for _, v := range want {
		if !set[v] {
			out = append(out, v)
		}
	}
	return out
}
