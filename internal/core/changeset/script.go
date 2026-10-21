package changeset

import (
	"regexp"
	"strings"

	"github.com/felipemalacarne/mesa/internal/core/ident"
)

// FromScript turns a SQL script someone wrote by hand into a plan: one
// statement per top-level semicolon, with the risks that can be read from
// the text itself. There is no down script: hand-written SQL cannot be
// reversed automatically.
func FromScript(database ident.Name, src string) *Plan {
	p := &Plan{Database: database}
	for _, sql := range SplitScript(src) {
		kind := DDL
		if isDML(sql) {
			kind = DML
		}
		p.Statements = append(p.Statements, Statement{Kind: kind, SQL: sql, Describe: firstLine(sql)})
		p.Risks = append(p.Risks, scriptRisks(sql)...)
		if kind == DML {
			p.Summary.Updated++
		} else {
			p.Summary.SchemaChanges++
		}
	}
	if len(p.Statements) > 0 {
		p.Risks = append(p.Risks, Risk{Level: Info, Code: RiskIrreversible, Message: "a hand-written script has no automatic rollback"})
	}
	return p
}

// SplitScript cuts a script on top-level semicolons. Semicolons inside
// string literals, quoted identifiers, dollar-quoted bodies and comments do
// not split. Empty statements and comment-only statements are dropped.
func SplitScript(src string) []string {
	var out []string
	var b strings.Builder
	flush := func() {
		s := strings.TrimSpace(b.String())
		b.Reset()
		if s != "" && strings.TrimSpace(stripComments(s)) != "" {
			out = append(out, s)
		}
	}
	for i := 0; i < len(src); i++ {
		c := src[i]
		switch {
		case c == '\'' || c == '"':
			j := endOfQuoted(src, i, c)
			b.WriteString(src[i:j])
			i = j - 1
		case c == '-' && i+1 < len(src) && src[i+1] == '-':
			j := strings.IndexByte(src[i:], '\n')
			if j < 0 {
				j = len(src) - i
			}
			b.WriteString(src[i : i+j])
			i += j - 1
		case c == '/' && i+1 < len(src) && src[i+1] == '*':
			j := strings.Index(src[i+2:], "*/")
			end := len(src)
			if j >= 0 {
				end = i + 2 + j + 2
			}
			b.WriteString(src[i:end])
			i = end - 1
		case c == '$':
			if tag := dollarTag(src[i:]); tag != "" {
				j := strings.Index(src[i+len(tag):], tag)
				end := len(src)
				if j >= 0 {
					end = i + len(tag) + j + len(tag)
				}
				b.WriteString(src[i:end])
				i = end - 1
			} else {
				b.WriteByte(c)
			}
		case c == ';':
			flush()
		default:
			b.WriteByte(c)
		}
	}
	flush()
	return out
}

// endOfQuoted returns the index just past the quoted run starting at i,
// treating a doubled quote as an escaped one.
func endOfQuoted(s string, i int, q byte) int {
	for j := i + 1; j < len(s); j++ {
		if s[j] == q {
			if j+1 < len(s) && s[j+1] == q {
				j++
				continue
			}
			return j + 1
		}
	}
	return len(s)
}

var dollarTagRE = regexp.MustCompile(`^\$[A-Za-z_]?[A-Za-z0-9_]*\$`)

func dollarTag(s string) string { return dollarTagRE.FindString(s) }

var commentRE = regexp.MustCompile(`(?s)--[^\n]*|/\*.*?\*/`)

func stripComments(s string) string { return commentRE.ReplaceAllString(s, " ") }

// normalized lowercases the statement without comments and collapses
// whitespace, for keyword matching.
func normalized(sql string) string {
	return strings.Join(strings.Fields(strings.ToLower(stripComments(sql))), " ")
}

func isDML(sql string) bool {
	n := normalized(sql)
	for _, k := range []string{"insert ", "update ", "delete ", "merge ", "with ", "copy "} {
		if strings.HasPrefix(n, k) {
			return true
		}
	}
	return false
}

var (
	reDropObject  = regexp.MustCompile(`^drop (table|schema|database|materialized view|view|type|sequence|function|index)\b`)
	reDropWhat    = regexp.MustCompile(`\bdrop (column )?(if exists )?("?[a-z_][^ ,;]*)`)
	reTruncate    = regexp.MustCompile(`^truncate\b`)
	reDeleteAll   = regexp.MustCompile(`^delete from [^ ]+( as [^ ]+| [^ w]+)?$`)
	reUpdateAll   = regexp.MustCompile(`^update `)
	reAlterType   = regexp.MustCompile(`^alter table .* alter (column )?[^ ]+ (set data )?type `)
	reSetNotNull  = regexp.MustCompile(`^alter table .* set not null`)
	reCreateIndex = regexp.MustCompile(`^create (unique )?index (?:if not exists )?`)
)

// scriptRisks reads the risks a reviewer must see from the statement text.
func scriptRisks(sql string) []Risk {
	n := normalized(sql)
	obj := firstLine(sql)
	var out []Risk
	add := func(level RiskLevel, code RiskCode, msg string) {
		out = append(out, Risk{Level: level, Code: code, Message: msg, Object: obj})
	}
	switch {
	case reDropObject.MatchString(n):
		add(Danger, RiskDataLoss, "drops an object and everything in it")
	case reTruncate.MatchString(n):
		add(Danger, RiskDataLoss, "removes every row of the table")
	case strings.HasPrefix(n, "alter table") && dropsColumn(n):
		add(Danger, RiskDataLoss, "drops a column and its values")
	case reDeleteAll.MatchString(n):
		add(Danger, RiskManyRows, "DELETE without WHERE removes every row")
	case reUpdateAll.MatchString(n) && !strings.Contains(n, " where "):
		add(Danger, RiskManyRows, "UPDATE without WHERE changes every row")
	case reAlterType.MatchString(n):
		add(Warning, RiskTableRewrite, "changing a column type can rewrite the table under an exclusive lock")
	case reSetNotNull.MatchString(n):
		add(Info, RiskFullScan, "every row is checked for nulls")
	case reCreateIndex.MatchString(n) && !strings.Contains(n, " concurrently "):
		add(Info, RiskExclusiveLock, "building the index blocks writes to the table until it is done")
	}
	return out
}

func firstLine(s string) string {
	line, _, _ := strings.Cut(strings.TrimSpace(s), "\n")
	if len(line) > 72 {
		return line[:72] + "…"
	}
	return line
}

// dropsColumn reports whether an ALTER TABLE drops a column, as opposed to
// a constraint, a default, NOT NULL, an identity or a generation expression.
func dropsColumn(n string) bool {
	for _, m := range reDropWhat.FindAllStringSubmatch(n, -1) {
		if m[1] != "" {
			return true
		}
		switch m[3] {
		case "constraint", "default", "not", "identity", "expression":
			continue
		}
		return true
	}
	return false
}
