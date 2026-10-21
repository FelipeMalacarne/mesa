// Package changeset describes what a person wants to change (a ChangeSet) and
// what will run to do it (a Plan). Every write in Mesa is one of these; the
// engine dialect turns the first into the second.
package changeset

import (
	"fmt"

	"github.com/felipemalacarne/mesa/internal/core/errs"
	"github.com/felipemalacarne/mesa/internal/core/ident"
	"github.com/felipemalacarne/mesa/internal/core/schema"
	"github.com/felipemalacarne/mesa/internal/core/value"
)

// RowKind is the kind of a row change.
type RowKind string

const (
	Insert RowKind = "insert"
	Update RowKind = "update"
	Delete RowKind = "delete"
)

// RowChange is one staged row edit. Key identifies the row (primary key
// values, or every original value when the table has no key); Values are the
// new cell values (Insert: all given cells; Update: changed cells only).
type RowChange struct {
	Kind   RowKind                    `json:"kind"`
	Table  ident.Qualified            `json:"table"`
	Key    map[ident.Name]value.Value `json:"key,omitempty"`
	Values map[ident.Name]value.Value `json:"values,omitempty"`
}

// ChangeSet is everything staged against one database.
type ChangeSet struct {
	Database ident.Name      `json:"database"`
	Rows     []RowChange     `json:"rows,omitempty"`
	Schema   []schema.Change `json:"schema,omitempty"`
}

func (cs *ChangeSet) IsEmpty() bool { return len(cs.Rows) == 0 && len(cs.Schema) == 0 }

// Summary counts the staged changes by kind for status lines.
type Summary struct {
	Inserted, Updated, Deleted int
	SchemaChanges              int
}

func (cs *ChangeSet) Summary() Summary {
	var s Summary
	for _, r := range cs.Rows {
		switch r.Kind {
		case Insert:
			s.Inserted++
		case Update:
			s.Updated++
		case Delete:
			s.Deleted++
		}
	}
	s.SchemaChanges = len(cs.Schema)
	return s
}

// Validate checks the change set is well formed before planning.
func (cs *ChangeSet) Validate() error {
	for i, r := range cs.Rows {
		if r.Table.Name == "" {
			return errs.Newf(errs.Invalid, "row change %d has no table", i)
		}
		switch r.Kind {
		case Insert:
			if len(r.Values) == 0 {
				return errs.Newf(errs.Invalid, "insert into %s has no values", r.Table)
			}
		case Update:
			if len(r.Key) == 0 {
				return errs.Newf(errs.Invalid, "update of %s has no key", r.Table)
			}
			if len(r.Values) == 0 {
				return errs.Newf(errs.Invalid, "update of %s changes nothing", r.Table)
			}
		case Delete:
			if len(r.Key) == 0 {
				return errs.Newf(errs.Invalid, "delete from %s has no key", r.Table)
			}
		default:
			return errs.Newf(errs.Invalid, "unknown row change kind %q", r.Kind)
		}
	}
	return nil
}

// StatementKind separates data changes from structure changes.
type StatementKind string

const (
	DML StatementKind = "dml"
	DDL StatementKind = "ddl"
)

// Statement is one SQL statement of a plan, with bound arguments so values
// are never interpolated into SQL text.
type Statement struct {
	Kind     StatementKind `json:"kind"`
	SQL      string        `json:"sql"`
	Args     []value.Value `json:"args,omitempty"`
	Describe string        `json:"describe"` // one line a person reads
}

// Display renders the statement with its arguments inlined as SQL literals,
// for review screens only; execution always uses SQL + Args.
func (s Statement) Display() string {
	if len(s.Args) == 0 {
		return s.SQL
	}
	out := []byte{}
	arg := 0
	for i := 0; i < len(s.SQL); i++ {
		if s.SQL[i] == '$' && i+1 < len(s.SQL) && s.SQL[i+1] >= '1' && s.SQL[i+1] <= '9' {
			j := i + 1
			n := 0
			for j < len(s.SQL) && s.SQL[j] >= '0' && s.SQL[j] <= '9' {
				n = n*10 + int(s.SQL[j]-'0')
				j++
			}
			if n >= 1 && n <= len(s.Args) {
				out = append(out, Literal(s.Args[n-1])...)
				arg++
				i = j - 1
				continue
			}
		}
		out = append(out, s.SQL[i])
	}
	return string(out)
}

// Literal renders a value as a SQL literal for display.
func Literal(v value.Value) string {
	switch v.Kind {
	case value.KindNull:
		return "NULL"
	case value.KindBool, value.KindInt, value.KindFloat, value.KindDecimal:
		return v.Text()
	case value.KindBytes:
		return "'" + v.Text() + "'"
	default:
		return "'" + escape(v.Text()) + "'"
	}
}

func escape(s string) string {
	out := make([]byte, 0, len(s)+2)
	for i := 0; i < len(s); i++ {
		if s[i] == '\'' {
			out = append(out, '\'')
		}
		out = append(out, s[i])
	}
	return string(out)
}

// RiskLevel grades a risk for the review screen.
type RiskLevel string

const (
	Info    RiskLevel = "info"
	Warning RiskLevel = "warning"
	Danger  RiskLevel = "danger"
)

// RiskCode names a kind of risk so the UI can icon and explain it.
type RiskCode string

const (
	RiskDataLoss      RiskCode = "data_loss"      // drop table/column, narrowing type
	RiskTableRewrite  RiskCode = "table_rewrite"  // type change, volatile default
	RiskExclusiveLock RiskCode = "exclusive_lock" // ACCESS EXCLUSIVE on a table
	RiskFullScan      RiskCode = "full_scan"      // SET NOT NULL, add check, validate FK
	RiskManyRows      RiskCode = "many_rows"      // DML touching many rows
	RiskNoKey         RiskCode = "no_key"         // row change on a table without a primary key
	RiskIrreversible  RiskCode = "irreversible"   // no down statement possible
)

// Risk is one thing the person should know before applying.
type Risk struct {
	Level   RiskLevel `json:"level"`
	Code    RiskCode  `json:"code"`
	Message string    `json:"message"`
	Object  string    `json:"object,omitempty"`
}

// Plan is what will run, in order, and how to undo it.
type Plan struct {
	Database   ident.Name  `json:"database"`
	Statements []Statement `json:"statements"`
	Down       []Statement `json:"down,omitempty"`
	Risks      []Risk      `json:"risks,omitempty"`
	Summary    Summary     `json:"summary"`
}

func (p *Plan) IsEmpty() bool { return len(p.Statements) == 0 }

// Worst returns the highest risk level in the plan.
func (p *Plan) Worst() RiskLevel {
	worst := Info
	for _, r := range p.Risks {
		switch {
		case r.Level == Danger:
			return Danger
		case r.Level == Warning:
			worst = Warning
		}
	}
	if len(p.Risks) == 0 {
		return ""
	}
	return worst
}

// Has reports whether the plan carries a risk with the given code.
func (p *Plan) Has(code RiskCode) bool {
	for _, r := range p.Risks {
		if r.Code == code {
			return true
		}
	}
	return false
}

// SQL renders the up statements as a script for display or download.
func (p *Plan) SQL() string {
	return script(p.Statements)
}

// DownSQL renders the down statements as a script.
func (p *Plan) DownSQL() string {
	return script(p.Down)
}

func script(stmts []Statement) string {
	out := ""
	for _, s := range stmts {
		out += s.Display()
		if out[len(out)-1] != ';' {
			out += ";"
		}
		out += "\n"
	}
	return out
}

// Title is a short label for the plan, used for migration file names.
func (p *Plan) Title() string {
	s := p.Summary
	switch {
	case s.SchemaChanges > 0 && len(p.Statements) > 0:
		return p.Statements[0].Describe
	case s.Inserted+s.Updated+s.Deleted > 0:
		return fmt.Sprintf("%d row changes", s.Inserted+s.Updated+s.Deleted)
	}
	return "empty"
}
