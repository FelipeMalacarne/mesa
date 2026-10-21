package mysql

import (
	"context"
	"slices"
	"sort"
	"strings"

	"github.com/felipemalacarne/mesa/internal/core/changeset"
	"github.com/felipemalacarne/mesa/internal/core/ident"
	"github.com/felipemalacarne/mesa/internal/core/schema"
	"github.com/felipemalacarne/mesa/internal/core/value"
)

// bigTable is the row estimate above which a copying change becomes a
// warning instead of a note.
const bigTable = 10000

// Compile implements engine.Dialect.
func (s *Session) Compile(_ context.Context, current *schema.Database, cs *changeset.ChangeSet) (*changeset.Plan, error) {
	return newDialect(current).compile(cs)
}

// dialect turns change sets into MySQL plans. It is a pure function of the
// current model, so it is testable offline. MySQL redefines a column as a
// whole (MODIFY COLUMN), so the dialect keeps a working copy of every
// column it touches and renders each change from it.
type dialect struct {
	current *schema.Database // may be nil
	columns map[[2]ident.Name]*schema.Column
}

func newDialect(current *schema.Database) *dialect {
	return &dialect{current: current, columns: map[[2]ident.Name]*schema.Column{}}
}

// compile renders schema changes first, then row changes. Down statements
// run in reverse order.
func (d *dialect) compile(cs *changeset.ChangeSet) (*changeset.Plan, error) {
	if err := cs.Validate(); err != nil {
		return nil, err
	}
	p := &changeset.Plan{Database: cs.Database, Summary: cs.Summary()}
	var downs [][]changeset.Statement
	for _, c := range cs.Schema {
		up, down, risks, err := d.schemaChange(c)
		if err != nil {
			return nil, err
		}
		p.Statements = append(p.Statements, up...)
		downs = append(downs, down)
		p.Risks = append(p.Risks, risks...)
	}
	if len(p.Statements) > 0 {
		p.Risks = append(p.Risks, risk(changeset.Info, changeset.RiskNoTransaction,
			"MySQL commits each schema change on its own: if a later statement fails, the earlier ones stay applied", ""))
	}
	for _, r := range cs.Rows {
		up, down, risks := d.rowChange(r)
		p.Statements = append(p.Statements, up...)
		downs = append(downs, down)
		p.Risks = append(p.Risks, risks...)
	}
	slices.Reverse(downs)
	for _, down := range downs {
		p.Down = append(p.Down, down...)
	}
	return p, nil
}

// table looks a table up in the current model.
func (d *dialect) table(q ident.Qualified) *schema.Table {
	if d.current == nil {
		return nil
	}
	return d.current.Table(q)
}

func (d *dialect) rowChange(r changeset.RowChange) (up, down []changeset.Statement, risks []changeset.Risk) {
	t := d.table(r.Table)
	name := table(r.Table)
	switch r.Kind {
	case changeset.Insert:
		up = []changeset.Statement{insertStatement(r.Table, r.Values, "Insert into "+r.Table.String())}
		if t != nil && t.PrimaryKey != nil && hasAll(r.Values, t.PrimaryKey.Columns) {
			where, args := whereKey(pick(r.Values, t.PrimaryKey.Columns))
			down = []changeset.Statement{dml("DELETE FROM "+name+where, args, "Delete the inserted row from "+r.Table.String())}
		} else {
			risks = append(risks, risk(changeset.Info, changeset.RiskIrreversible,
				"the inserted row cannot be removed automatically: its key is not known before the insert", r.Table.String()))
		}
	case changeset.Update:
		sets := sortedNames(r.Values)
		var args []value.Value
		var assign []string
		for _, c := range sets {
			args = append(args, r.Values[c])
			assign = append(assign, quote(c)+" = ?")
		}
		where, whereArgs := whereKey(r.Key)
		up = []changeset.Statement{dml("UPDATE "+name+" SET "+strings.Join(assign, ", ")+where,
			append(args, whereArgs...), "Update a row in "+r.Table.String())}
		if hasAll(r.Key, sets) {
			after := map[ident.Name]value.Value{}
			for k, v := range r.Key {
				after[k] = v
			}
			var dargs []value.Value
			var dassign []string
			for _, c := range sets {
				after[c] = r.Values[c]
				dargs = append(dargs, r.Key[c])
				dassign = append(dassign, quote(c)+" = ?")
			}
			dwhere, dwhereArgs := whereKey(after)
			down = []changeset.Statement{dml("UPDATE "+name+" SET "+strings.Join(dassign, ", ")+dwhere,
				append(dargs, dwhereArgs...), "Restore the previous values in "+r.Table.String())}
		} else {
			risks = append(risks, risk(changeset.Info, changeset.RiskIrreversible,
				"the previous values are not part of the change set, so the update cannot be reversed", r.Table.String()))
		}
		if t != nil && t.PrimaryKey == nil {
			risks = append(risks, noKeyRisk(r.Table))
		}
	case changeset.Delete:
		where, args := whereKey(r.Key)
		up = []changeset.Statement{dml("DELETE FROM "+name+where+" LIMIT 1", args, "Delete a row from "+r.Table.String())}
		if t != nil && len(t.Columns) > 0 && hasAll(r.Key, columnNames(t)) {
			down = []changeset.Statement{insertStatement(r.Table, r.Key, "Re-insert the deleted row into "+r.Table.String())}
		} else {
			risks = append(risks, risk(changeset.Info, changeset.RiskIrreversible,
				"the deleted row cannot be re-inserted automatically: the change set does not carry every column", r.Table.String()))
		}
		if t != nil && t.PrimaryKey == nil {
			risks = append(risks, noKeyRisk(r.Table))
		}
	}
	return up, down, risks
}

func noKeyRisk(q ident.Qualified) changeset.Risk {
	return risk(changeset.Danger, changeset.RiskNoKey,
		q.String()+" has no primary key: the WHERE clause matches every row with the same values", q.String())
}

func insertStatement(q ident.Qualified, values map[ident.Name]value.Value, describe string) changeset.Statement {
	cols := sortedNames(values)
	args := make([]value.Value, len(cols))
	holders := make([]string, len(cols))
	for i, c := range cols {
		args[i] = values[c]
		holders[i] = "?"
	}
	return dml("INSERT INTO "+table(q)+" ("+quoteList(cols)+") VALUES ("+strings.Join(holders, ", ")+")", args, describe)
}

// whereKey renders " WHERE c1 = ? AND c2 IS NULL" for the key values.
func whereKey(key map[ident.Name]value.Value) (string, []value.Value) {
	var parts []string
	var args []value.Value
	for _, c := range sortedNames(key) {
		v := key[c]
		if v.IsNull() {
			parts = append(parts, quote(c)+" IS NULL")
			continue
		}
		args = append(args, v)
		parts = append(parts, quote(c)+" = ?")
	}
	if len(parts) == 0 {
		return "", nil
	}
	return " WHERE " + strings.Join(parts, " AND "), args
}

// dml builds a row statement; each one must touch exactly one row.
func dml(sql string, args []value.Value, describe string) changeset.Statement {
	return changeset.Statement{Kind: changeset.DML, SQL: sql, Args: args, Describe: describe, ExpectRows: 1}
}

// ddl builds a DDL statement without arguments.
func ddl(sql, describe string) changeset.Statement {
	return changeset.Statement{Kind: changeset.DDL, SQL: sql, Describe: describe}
}

func risk(level changeset.RiskLevel, code changeset.RiskCode, msg, object string) changeset.Risk {
	return changeset.Risk{Level: level, Code: code, Message: msg, Object: object}
}

func sortedNames(m map[ident.Name]value.Value) []ident.Name {
	out := make([]ident.Name, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

func hasAll(m map[ident.Name]value.Value, cols []ident.Name) bool {
	for _, c := range cols {
		if _, ok := m[c]; !ok {
			return false
		}
	}
	return true
}

func pick(m map[ident.Name]value.Value, cols []ident.Name) map[ident.Name]value.Value {
	out := make(map[ident.Name]value.Value, len(cols))
	for _, c := range cols {
		out[c] = m[c]
	}
	return out
}

func columnNames(t *schema.Table) []ident.Name {
	out := make([]ident.Name, len(t.Columns))
	for i, c := range t.Columns {
		out[i] = c.Name
	}
	return out
}

func quoteList(cols []ident.Name) string {
	parts := make([]string, len(cols))
	for i, c := range cols {
		parts[i] = quote(c)
	}
	return strings.Join(parts, ", ")
}
