package postgres

import (
	"regexp"
	"slices"
	"strings"
)

// shortTypes maps the SQL-standard spellings format_type returns to the
// short names people write. Both are valid PostgreSQL, so DDL built from
// either works; the short form reads better in the grid, the structure
// view and DBML.
var shortTypes = []struct{ long, short string }{
	{"character varying", "varchar"},
	{"timestamp with time zone", "timestamptz"},
	{"timestamp without time zone", "timestamp"},
	{"time with time zone", "timetz"},
	{"time without time zone", "time"},
	{"double precision", "float8"},
	{"bit varying", "varbit"},
	{"character", "char"},
}

// shortType rewrites a format_type spelling to its short form, keeping
// modifiers and array brackets: "character varying(120)[]" → "varchar(120)[]".
// "timestamp(3) with time zone" becomes "timestamptz(3)".
func shortType(t string) string {
	for _, m := range shortTypes {
		if strings.HasPrefix(t, m.long) {
			rest := t[len(m.long):]
			if rest == "" || rest[0] == '(' || rest[0] == '[' {
				return m.short + rest
			}
		}
	}
	// "timestamp(3) with time zone" and friends put the modifier mid-name.
	if i := strings.IndexByte(t, '('); i > 0 {
		if j := strings.IndexByte(t[i:], ')'); j > 0 {
			name := t[:i] + t[i+j+1:]
			mod := t[i : i+j+1]
			for _, m := range shortTypes {
				if strings.HasPrefix(name, m.long) {
					rest := name[len(m.long):]
					if rest == "" || rest[0] == '[' {
						return m.short + mod + rest
					}
				}
			}
		}
	}
	return t
}

// SameType implements engine.Dialect.
func (s *Session) SameType(a, b string) bool { return sameType(a, b) }

// SameDefault implements engine.Dialect.
func (s *Session) SameDefault(a, b string) bool { return sameDefault(a, b) }

func sameType(a, b string) bool {
	if a == b {
		return true
	}
	x, y := parseTypeName(unqualify(shortType(a))), parseTypeName(unqualify(shortType(b)))
	if x.base == "serial" || x.base == "bigserial" || y.base == "serial" || y.base == "bigserial" {
		return false
	}
	return x.base == y.base && x.array == y.array && slices.Equal(x.mods, y.mods)
}

// unqualify drops quotes and the public schema from a type name.
func unqualify(t string) string {
	t = strings.ReplaceAll(t, `"`, "")
	return strings.TrimPrefix(t, "public.")
}

var trailingCast = regexp.MustCompile(`::[A-Za-z_][\w\s."]*(\(\d+(,\s*\d+)?\))?(\[\])?$`)

// nowAliases are the spellings of "the transaction's start time".
var nowAliases = map[string]bool{
	"now()": true, "current_timestamp": true, "transaction_timestamp()": true,
}

func sameDefault(a, b string) bool {
	return normalizeDefault(a) == normalizeDefault(b)
}

func normalizeDefault(e string) string {
	e = strings.TrimSpace(e)
	for {
		trimmed := e
		if strings.HasPrefix(trimmed, "(") && strings.HasSuffix(trimmed, ")") && balanced(trimmed[1:len(trimmed)-1]) {
			trimmed = strings.TrimSpace(trimmed[1 : len(trimmed)-1])
		}
		trimmed = strings.TrimSpace(trailingCast.ReplaceAllString(trimmed, ""))
		if trimmed == e {
			break
		}
		e = trimmed
	}
	if nowAliases[strings.ToLower(e)] {
		return "now()"
	}
	if !strings.HasPrefix(e, "'") {
		e = strings.ToLower(e)
	}
	return e
}

// balanced reports whether parentheses outside string literals balance.
func balanced(s string) bool {
	depth, inStr := 0, false
	for _, r := range s {
		switch {
		case r == '\'':
			inStr = !inStr
		case inStr:
		case r == '(':
			depth++
		case r == ')':
			depth--
			if depth < 0 {
				return false
			}
		}
	}
	return depth == 0 && !inStr
}
