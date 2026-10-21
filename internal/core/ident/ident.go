// Package ident holds database identifiers and the rules for validating and
// quoting them. Identifiers are kept exactly as the database spells them.
package ident

import (
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/felipemalacarne/mesa/internal/core/errs"
)

// MaxLen is the longest identifier Mesa accepts (PostgreSQL's limit is 63 bytes).
const MaxLen = 63

// Name is a single identifier: a database, schema, table, column, index or role name.
type Name string

// Parse validates s as an identifier.
func Parse(s string) (Name, error) {
	if s == "" {
		return "", errs.New(errs.Invalid, "identifier is empty")
	}
	if len(s) > MaxLen {
		return "", errs.Newf(errs.Invalid, "identifier %q is longer than %d bytes", s, MaxLen)
	}
	if !utf8.ValidString(s) {
		return "", errs.Newf(errs.Invalid, "identifier %q is not valid UTF-8", s)
	}
	for _, r := range s {
		if r == 0 || unicode.IsControl(r) {
			return "", errs.Newf(errs.Invalid, "identifier %q contains a control character", s)
		}
	}
	return Name(s), nil
}

// MustParse is Parse for identifiers known to be valid (tests, constants).
func MustParse(s string) Name {
	n, err := Parse(s)
	if err != nil {
		panic(err)
	}
	return n
}

func (n Name) String() string { return string(n) }

// IsSimple reports whether n needs no quoting in SQL: lowercase letters,
// digits and underscores, not starting with a digit.
func (n Name) IsSimple() bool {
	if n == "" {
		return false
	}
	for i, r := range n {
		switch {
		case r == '_', r >= 'a' && r <= 'z':
		case r >= '0' && r <= '9':
			if i == 0 {
				return false
			}
		default:
			return false
		}
	}
	return true
}

// Quote returns n as a double-quoted SQL identifier, escaping embedded quotes.
// It always quotes, which is correct for every engine Mesa targets.
func (n Name) Quote() string {
	return `"` + strings.ReplaceAll(string(n), `"`, `""`) + `"`
}

// Qualified names an object inside a schema.
type Qualified struct {
	Schema Name
	Name   Name
}

// Q builds a qualified name from already-validated parts.
func Q(schema, name Name) Qualified { return Qualified{Schema: schema, Name: name} }

// String renders schema.name without quoting, for display.
func (q Qualified) String() string {
	if q.Schema == "" {
		return q.Name.String()
	}
	return q.Schema.String() + "." + q.Name.String()
}

// Quote renders the name for SQL, quoting each part.
func (q Qualified) Quote() string {
	if q.Schema == "" {
		return q.Name.Quote()
	}
	return q.Schema.Quote() + "." + q.Name.Quote()
}

// ParseQualified splits "schema.name" (or "name") into a Qualified. Dots inside
// quoted parts are not supported; callers with such names pass the parts directly.
func ParseQualified(s string, defaultSchema Name) (Qualified, error) {
	parts := strings.SplitN(s, ".", 2)
	if len(parts) == 1 {
		n, err := Parse(parts[0])
		if err != nil {
			return Qualified{}, err
		}
		return Qualified{Schema: defaultSchema, Name: n}, nil
	}
	schema, err := Parse(parts[0])
	if err != nil {
		return Qualified{}, err
	}
	n, err := Parse(parts[1])
	if err != nil {
		return Qualified{}, err
	}
	return Qualified{Schema: schema, Name: n}, nil
}
