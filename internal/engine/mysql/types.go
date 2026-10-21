package mysql

import (
	"slices"
	"strconv"
	"strings"
)

// SameType implements engine.Dialect.
func (s *Session) SameType(a, b string) bool { return sameType(a, b) }

// SameDefault implements engine.Dialect.
func (s *Session) SameDefault(a, b string) bool { return sameDefault(a, b) }

// typeSpec is a parsed type name: base (normalized), modifiers, flags.
type typeSpec struct {
	base     string
	mods     []int
	unsigned bool
	rest     string // enum/set values and anything else, verbatim
}

var typeAliases = map[string]string{
	"integer": "int", "numeric": "decimal", "dec": "decimal", "fixed": "decimal",
	"double precision": "double", "real": "double", "bool": "tinyint(1)", "boolean": "tinyint(1)",
	"character varying": "varchar", "character": "char", "int4": "int", "int8": "bigint", "int2": "smallint",
	"float8": "double", "float4": "float", "text": "text", "timestamptz": "timestamp",
}

// displayWidth types ignore their (n) in MySQL 8, except tinyint(1).
var displayWidth = map[string]bool{"smallint": true, "mediumint": true, "int": true, "bigint": true}

func parseTypeName(s string) typeSpec {
	s = strings.ToLower(strings.TrimSpace(s))
	var spec typeSpec
	for _, flag := range []string{" zerofill", " unsigned", " signed"} {
		if strings.HasSuffix(s, flag) {
			spec.unsigned = spec.unsigned || flag == " unsigned"
			s = strings.TrimSpace(strings.TrimSuffix(s, flag))
		}
	}
	if a, ok := typeAliases[s]; ok {
		s = a
	}
	if i := strings.IndexByte(s, '('); i >= 0 && strings.HasSuffix(s, ")") {
		inner := s[i+1 : len(s)-1]
		base := strings.TrimSpace(s[:i])
		if a, ok := typeAliases[base]; ok && !strings.Contains(a, "(") {
			base = a
		}
		if base == "enum" || base == "set" {
			spec.base, spec.rest = base, inner
			return spec
		}
		for _, m := range strings.Split(inner, ",") {
			if n, err := strconv.Atoi(strings.TrimSpace(m)); err == nil {
				spec.mods = append(spec.mods, n)
			}
		}
		s = base
	}
	spec.base = s
	if displayWidth[s] {
		spec.mods = nil
	}
	if s == "decimal" && len(spec.mods) == 0 {
		spec.mods = []int{10, 0}
	}
	if s == "decimal" && len(spec.mods) == 1 {
		spec.mods = append(spec.mods, 0)
	}
	if s == "tinyint" && !slices.Equal(spec.mods, []int{1}) {
		spec.mods = nil
	}
	return spec
}

func sameType(a, b string) bool {
	if strings.EqualFold(a, b) {
		return true
	}
	x, y := parseTypeName(mysqlType(a)), parseTypeName(mysqlType(b))
	return x.base == y.base && x.unsigned == y.unsigned && slices.Equal(x.mods, y.mods) && x.rest == y.rest
}

// sameDefault compares default expressions after the spellings MySQL
// changes on the way in: TRUE becomes 1, numbers lose their quotes, now()
// becomes CURRENT_TIMESTAMP, expressions gain parentheses.
func sameDefault(a, b string) bool {
	return normDefault(a) == normDefault(b)
}

func normDefault(d string) string {
	d = strings.TrimSpace(d)
	for strings.HasPrefix(d, "(") && strings.HasSuffix(d, ")") && balanced(d[1:len(d)-1]) {
		d = strings.TrimSpace(d[1 : len(d)-1])
	}
	if len(d) >= 2 && d[0] == '\'' && d[len(d)-1] == '\'' {
		inner := d[1 : len(d)-1]
		if _, err := strconv.ParseFloat(inner, 64); err == nil {
			d = inner
		}
	}
	switch u := strings.ToUpper(d); u {
	case "TRUE":
		return "1"
	case "FALSE":
		return "0"
	case "NOW()", "CURRENT_TIMESTAMP()", "LOCALTIMESTAMP", "LOCALTIMESTAMP()", "CURRENT_TIMESTAMP":
		return "CURRENT_TIMESTAMP"
	}
	if f, err := strconv.ParseFloat(d, 64); err == nil {
		return strconv.FormatFloat(f, 'g', -1, 64)
	}
	return charsetIntroducer.ReplaceAllString(strings.ReplaceAll(d, "`", ""), "'")
}

// narrowing reports whether changing a column from oldT to newT can lose
// data: a smaller integer, a shorter string, less decimal precision, or
// time dropped.
func narrowing(oldT, newT string) bool {
	if oldT == "" {
		return false
	}
	o, n := parseTypeName(oldT), parseTypeName(newT)
	intRank := map[string]int{"tinyint": 1, "smallint": 2, "mediumint": 3, "int": 4, "bigint": 8}
	textRank := map[string]int{"tinytext": 255, "text": 65535, "mediumtext": 16777215, "longtext": 4294967295}
	mod := func(t typeSpec, i int) int {
		if i < len(t.mods) {
			return t.mods[i]
		}
		return 0
	}
	strLen := func(t typeSpec) (int, bool) {
		if r, ok := textRank[t.base]; ok {
			return r, true
		}
		if t.base == "varchar" || t.base == "char" {
			return mod(t, 0), true
		}
		return 0, false
	}
	_, oInt := intRank[o.base]
	_, nInt := intRank[n.base]
	switch {
	case oInt && nInt:
		return intRank[n.base] < intRank[o.base] || o.unsigned != n.unsigned
	case (o.base == "decimal" || o.base == "double" || o.base == "float") && nInt:
		return true
	case o.base == "double" && n.base == "float":
		return true
	case o.base == "decimal" && n.base == "decimal":
		return mod(n, 0)-mod(n, 1) < mod(o, 0)-mod(o, 1) || mod(n, 1) < mod(o, 1)
	case (o.base == "datetime" || o.base == "timestamp") && (n.base == "date" || n.base == "time"):
		return true
	case o.base == "enum" && n.base == "enum":
		return !strings.HasPrefix(n.rest, o.rest)
	}
	ol, oStr := strLen(o)
	nl, nStr := strLen(n)
	if oStr && nStr {
		return nl < ol
	}
	return false
}

// humanCount renders 1234567 as "1.2M".
func humanCount(n float64) string {
	switch {
	case n >= 1e9:
		return trimFloat(n/1e9, 1) + "B"
	case n >= 1e6:
		return trimFloat(n/1e6, 1) + "M"
	case n >= 1e3:
		return trimFloat(n/1e3, 1) + "k"
	}
	return trimFloat(n, 0)
}

// humanBytes renders a size with a binary unit.
func humanBytes(n int64) string {
	const unit = 1024
	if n < unit {
		return strconv.FormatInt(n, 10) + " B"
	}
	div, exp := int64(unit), 0
	for m := n / unit; m >= unit; m /= unit {
		div *= unit
		exp++
	}
	return trimFloat(float64(n)/float64(div), 1) + " " + string("KMGTPE"[exp]) + "iB"
}

func trimFloat(v float64, prec int) string {
	s := strconv.FormatFloat(v, 'f', prec, 64)
	if strings.Contains(s, ".") {
		s = strings.TrimRight(strings.TrimRight(s, "0"), ".")
	}
	return s
}

// pgSpellings maps type names people bring from PostgreSQL (and the
// designer's defaults) to MySQL's.
var pgSpellings = map[string]string{
	"timestamptz": "timestamp", "timestamp with time zone": "timestamp", "timestamp without time zone": "datetime",
	"uuid": "char(36)", "jsonb": "json", "bytea": "longblob", "double precision": "double",
	"float8": "double", "float4": "float", "int8": "bigint", "int4": "int", "int2": "smallint",
	"character varying": "varchar(255)", "varchar": "varchar(255)", "citext": "text", "inet": "varchar(45)",
}

// mysqlType rewrites a PostgreSQL spelling to MySQL, keeping modifiers:
// "timestamptz(3)" → "timestamp(3)", "character varying(80)" → "varchar(80)".
// Anything else is returned unchanged.
func mysqlType(t string) string {
	lower := strings.ToLower(strings.TrimSpace(t))
	if m, ok := pgSpellings[lower]; ok {
		return m
	}
	if i := strings.IndexByte(lower, '('); i > 0 {
		base, mods := strings.TrimSpace(lower[:i]), lower[i:]
		if m, ok := pgSpellings[base]; ok {
			if j := strings.IndexByte(m, '('); j > 0 {
				m = m[:j]
			}
			return m + mods
		}
	}
	return t
}
