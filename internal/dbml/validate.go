package dbml

import (
	"fmt"

	"github.com/felipemalacarne/mesa/internal/core/ident"
	"github.com/felipemalacarne/mesa/internal/core/schema"
)

// Problem is something wrong or questionable in a model, for the designer
// to show next to the text.
type Problem struct {
	// Line is where the object was declared in the DBML source, or 0 when
	// unknown (a model that did not come from Parse).
	Line    int    `json:"line,omitempty"`
	Level   string `json:"level"` // LevelError or LevelWarning
	Message string `json:"message"`
}

const (
	LevelError   = "error"
	LevelWarning = "warning"
)

// Validate reports what the engine would reject (foreign keys to unknown
// tables or columns, duplicate names, keys on unknown columns) as errors and
// what is probably a mistake (a table without a primary key) as warnings.
// Lines are 0; use Check to validate DBML text with lines.
func Validate(db *schema.Database) []Problem {
	return validate(db, nil)
}

// Check parses src and validates the result. A syntax error is returned as
// the only problem, with its line.
func Check(src string) []Problem {
	db, lines, serr := parse(src)
	if serr != nil {
		return []Problem{{Line: serr.line, Level: LevelError, Message: serr.msg}}
	}
	return validate(db, lines)
}

func validate(db *schema.Database, lines positions) []Problem {
	var out []Problem
	report := func(level, posKey, format string, args ...any) {
		out = append(out, Problem{Line: lines[posKey], Level: level, Message: fmt.Sprintf(format, args...)})
	}
	for si := range db.Schemas {
		s := &db.Schemas[si]
		seenTables := map[ident.Name]bool{}
		for ti := range s.Tables {
			t := &s.Tables[ti]
			q := ident.Q(s.Name, t.Name)
			if seenTables[t.Name] {
				report(LevelError, posTable(q), "table %s is defined twice", q)
				continue
			}
			seenTables[t.Name] = true

			if len(t.Columns) == 0 {
				report(LevelWarning, posTable(q), "table %s has no columns", q)
			}
			seenCols := map[ident.Name]bool{}
			for i := range t.Columns {
				c := &t.Columns[i]
				if seenCols[c.Name] {
					report(LevelError, posColumn(q, c.Name), "column %s of table %s is defined twice", c.Name, q)
				}
				seenCols[c.Name] = true
				if c.Type == "" {
					report(LevelError, posColumn(q, c.Name), "column %s of table %s has no type", c.Name, q)
				}
			}

			if t.PrimaryKey == nil {
				report(LevelWarning, posTable(q), "table %s has no primary key", q)
			} else {
				for _, c := range t.PrimaryKey.Columns {
					if !seenCols[c] {
						report(LevelError, posPrimaryKey(q), "primary key of table %s uses unknown column %s", q, c)
					}
				}
			}
			for i := range t.Uniques {
				u := &t.Uniques[i]
				for _, c := range u.Columns {
					if !seenCols[c] {
						report(LevelError, posUnique(q, u.Columns), "unique constraint %s of table %s uses unknown column %s", keyLabel(u.Name, u.Columns), q, c)
					}
				}
			}
			for i := range t.Indexes {
				ix := &t.Indexes[i]
				if ix.Name == "" {
					report(LevelError, posIndex(q, ix.Name), "an index of table %s has no name", q)
				}
				for _, c := range ix.Columns {
					if !seenCols[c] {
						report(LevelError, posIndex(q, ix.Name), "index %s of table %s uses unknown column %s", ix.Name, q, c)
					}
				}
			}
			for i := range t.ForeignKeys {
				fk := &t.ForeignKeys[i]
				label := keyLabel(fk.Name, fk.Columns)
				pos := posForeignKey(q, fk)
				for _, c := range fk.Columns {
					if !seenCols[c] {
						report(LevelError, pos, "foreign key %s of table %s uses unknown column %s", label, q, c)
					}
				}
				target := db.Table(fk.References)
				if target == nil {
					report(LevelError, pos, "foreign key %s of table %s references unknown table %s", label, q, fk.References)
					continue
				}
				if len(fk.Columns) != len(fk.RefColumns) {
					report(LevelError, pos, "foreign key %s of table %s joins %d columns to %d", label, q, len(fk.Columns), len(fk.RefColumns))
				}
				for _, c := range fk.RefColumns {
					if target.Column(c) == nil {
						report(LevelError, pos, "foreign key %s of table %s references unknown column %s of table %s", label, q, c, fk.References)
					}
				}
			}
		}
	}
	return out
}

// keyLabel names a constraint by its name, or by its columns when unnamed.
func keyLabel(name ident.Name, cols []ident.Name) string {
	if name != "" {
		return name.String()
	}
	return "(" + joinNames(cols) + ")"
}
