package postgres

import (
	"context"
	"slices"
	"sort"
	"strconv"
	"strings"

	"github.com/felipemalacarne/mesa/internal/core/changeset"
	"github.com/felipemalacarne/mesa/internal/core/ident"
	"github.com/felipemalacarne/mesa/internal/core/schema"
	"github.com/felipemalacarne/mesa/internal/core/value"
)

// bigTable is the row estimate above which a scanning or locking change
// becomes a warning instead of a note.
const bigTable = 10000

// Compile implements engine.Dialect.
func (s *Session) Compile(_ context.Context, current *schema.Database, cs *changeset.ChangeSet) (*changeset.Plan, error) {
	return dialect{version: s.major(), current: current}.compile(cs)
}

// dialect turns change sets into PostgreSQL plans. It is a pure function of
// the server version and the current model, so it is testable offline.
type dialect struct {
	version int              // server major version, 0 when unknown
	current *schema.Database // may be nil
}

// compile renders schema changes first, then row changes, so rows can
// target structures the same change set creates. Down statements run in
// reverse order.
func (d dialect) compile(cs *changeset.ChangeSet) (*changeset.Plan, error) {
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
func (d dialect) table(q ident.Qualified) *schema.Table {
	if d.current == nil {
		return nil
	}
	return d.current.Table(q)
}

func (d dialect) rowChange(r changeset.RowChange) (up, down []changeset.Statement, risks []changeset.Risk) {
	t := d.table(r.Table)
	switch r.Kind {
	case changeset.Insert:
		up = []changeset.Statement{insertStatement(r.Table, r.Values, "Insert into "+r.Table.String())}
		if t != nil && t.PrimaryKey != nil && hasAll(r.Values, t.PrimaryKey.Columns) {
			key := pick(r.Values, t.PrimaryKey.Columns)
			where, args := whereKey(key, 1)
			down = []changeset.Statement{dml("DELETE FROM "+r.Table.Quote()+where, args, "Delete the inserted row from "+r.Table.String())}
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
			assign = append(assign, c.Quote()+" = $"+strconv.Itoa(len(args)))
		}
		where, whereArgs := whereKey(r.Key, len(args)+1)
		up = []changeset.Statement{dml("UPDATE "+r.Table.Quote()+" SET "+strings.Join(assign, ", ")+where,
			append(args, whereArgs...), "Update a row in "+r.Table.String())}
		if hasAll(r.Key, sets) {
			// Old values are in the key, so the reverse update is known.
			after := map[ident.Name]value.Value{}
			for k, v := range r.Key {
				after[k] = v
			}
			var dargs []value.Value
			var dassign []string
			for _, c := range sets {
				after[c] = r.Values[c]
				dargs = append(dargs, r.Key[c])
				dassign = append(dassign, c.Quote()+" = $"+strconv.Itoa(len(dargs)))
			}
			dwhere, dwhereArgs := whereKey(after, len(dargs)+1)
			down = []changeset.Statement{dml("UPDATE "+r.Table.Quote()+" SET "+strings.Join(dassign, ", ")+dwhere,
				append(dargs, dwhereArgs...), "Restore the previous values in "+r.Table.String())}
		} else {
			risks = append(risks, risk(changeset.Info, changeset.RiskIrreversible,
				"the previous values are not part of the change set, so the update cannot be reversed", r.Table.String()))
		}
		if t != nil && t.PrimaryKey == nil {
			risks = append(risks, noKeyRisk(r.Table))
		}
	case changeset.Delete:
		where, args := whereKey(r.Key, 1)
		up = []changeset.Statement{dml("DELETE FROM "+r.Table.Quote()+where, args, "Delete a row from "+r.Table.String())}
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
		holders[i] = "$" + strconv.Itoa(i+1)
	}
	return dml("INSERT INTO "+q.Quote()+" ("+quoteList(cols)+") VALUES ("+strings.Join(holders, ", ")+")", args, describe)
}

// whereKey renders " WHERE c1 = $n AND c2 IS NULL" for the key values, with
// placeholders numbered from start.
func whereKey(key map[ident.Name]value.Value, start int) (string, []value.Value) {
	var parts []string
	var args []value.Value
	for _, c := range sortedNames(key) {
		v := key[c]
		if v.IsNull() {
			parts = append(parts, c.Quote()+" IS NULL")
			continue
		}
		args = append(args, v)
		parts = append(parts, c.Quote()+" = $"+strconv.Itoa(start+len(args)-1))
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
		parts[i] = c.Quote()
	}
	return strings.Join(parts, ", ")
}

// --- type heuristics ------------------------------------------------------

// typeSpec is a parsed type name: base (normalized), modifiers, array flag.
type typeSpec struct {
	base  string
	mods  []int
	array bool
}

// typeAliases maps SQL spellings to PostgreSQL's internal names.
func typeAliases() map[string]string {
	return map[string]string{
		"bigint": "int8", "int": "int4", "integer": "int4", "smallint": "int2",
		"double precision": "float8", "real": "float4",
		"character varying": "varchar", "character": "bpchar", "char": "bpchar",
		"decimal": "numeric", "boolean": "bool",
		"timestamp with time zone": "timestamptz", "timestamp without time zone": "timestamp",
		"time with time zone": "timetz", "time without time zone": "time",
	}
}

func parseTypeName(s string) typeSpec {
	s = strings.ToLower(strings.TrimSpace(s))
	var spec typeSpec
	if strings.HasSuffix(s, "[]") {
		spec.array = true
		s = strings.TrimSpace(strings.TrimSuffix(s, "[]"))
	}
	if i := strings.IndexByte(s, '('); i >= 0 && strings.HasSuffix(s, ")") {
		for _, m := range strings.Split(s[i+1:len(s)-1], ",") {
			if n, err := strconv.Atoi(strings.TrimSpace(m)); err == nil {
				spec.mods = append(spec.mods, n)
			}
		}
		s = strings.TrimSpace(s[:i])
	}
	if a, ok := typeAliases()[s]; ok {
		s = a
	}
	spec.base = s
	return spec
}

// narrowing reports whether changing a column from oldT to newT can lose
// data: a smaller integer or float, a shorter string, less numeric
// precision, fractions or time dropped.
func narrowing(oldT, newT string) bool {
	o, n := parseTypeName(oldT), parseTypeName(newT)
	if o.array != n.array {
		return false
	}
	intRank := map[string]int{"int2": 2, "int4": 4, "int8": 8}
	floatRank := map[string]int{"float4": 4, "float8": 8}
	isInt := func(t typeSpec) bool { _, ok := intRank[t.base]; return ok }
	isFloat := func(t typeSpec) bool { _, ok := floatRank[t.base]; return ok }
	isString := func(t typeSpec) bool { return t.base == "text" || t.base == "varchar" || t.base == "bpchar" }
	mod := func(t typeSpec, i int) int {
		if i < len(t.mods) {
			return t.mods[i]
		}
		return 0
	}
	switch {
	case isInt(o) && isInt(n):
		return intRank[n.base] < intRank[o.base]
	case isFloat(o) && isFloat(n):
		return floatRank[n.base] < floatRank[o.base]
	case (o.base == "numeric" || isFloat(o)) && isInt(n):
		return true
	case o.base == "numeric" && n.base == "numeric":
		if len(n.mods) == 0 {
			return false
		}
		if len(o.mods) == 0 {
			return true
		}
		return mod(n, 0) < mod(o, 0) || mod(n, 1) < mod(o, 1)
	case isString(o) && isString(n):
		if len(n.mods) == 0 {
			return false
		}
		if len(o.mods) == 0 {
			return true
		}
		return mod(n, 0) < mod(o, 0)
	case (o.base == "timestamp" || o.base == "timestamptz") && n.base == "date":
		return true
	case o.base == "timestamptz" && n.base == "timestamp":
		return true
	}
	return false
}

// volatileDefault guesses whether a default expression differs per row,
// which forces a table rewrite when a NOT NULL column is added.
func volatileDefault(expr string) bool {
	e := strings.ToLower(expr)
	for _, f := range []string{"random(", "gen_random_uuid(", "uuid_generate", "nextval(", "clock_timestamp(", "timeofday(", "statement_timestamp(", "txid_current("} {
		if strings.Contains(e, f) {
			return true
		}
	}
	return false
}
