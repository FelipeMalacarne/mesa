package dbml

import (
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"
)

// syntaxError is where parsing stopped and why. Parse reports it as an
// errs.Invalid error; Check reports it as a Problem with its line.
type syntaxError struct {
	line, col int
	msg       string
}

func (e *syntaxError) Error() string {
	return fmt.Sprintf("line %d, column %d: %s", e.line, e.col, e.msg)
}

type tokenKind int

const (
	tokEOF    tokenKind = iota
	tokWord             // identifier or keyword; also a colour such as #3498DB
	tokNumber           // 12, 3.14
	tokString           // '…', "…" or '''…'''; text holds the decoded value
	tokExpr             // `…`; text holds the raw expression
	tokSymbol           // one of { } [ ] ( ) , : . < > - <>
)

type token struct {
	kind tokenKind
	text string
	line int
	col  int
	// start and end are byte offsets into the source, so the parser can take
	// a span verbatim (column types).
	start, end int
	// adjacent is true when nothing (no whitespace, no comment) separates the
	// token from the one before it, which tells int[] from int [pk].
	adjacent bool
}

func (t token) is(kind tokenKind, text string) bool {
	return t.kind == kind && t.text == text
}

func (t token) isSymbol(s string) bool { return t.is(tokSymbol, s) }

// isWord reports whether t is the keyword w; keywords are case-insensitive.
func (t token) isWord(w string) bool {
	return t.kind == tokWord && strings.EqualFold(t.text, w)
}

// describe names a token for an error message.
func (t token) describe() string {
	switch t.kind {
	case tokEOF:
		return "end of input"
	case tokString:
		return fmt.Sprintf("string %q", t.text)
	case tokExpr:
		return "expression `" + t.text + "`"
	default:
		return "'" + t.text + "'"
	}
}

type lexer struct {
	src  string
	pos  int
	line int
	col  int
}

// lex splits src into tokens; the last one is always tokEOF.
func lex(src string) ([]token, *syntaxError) {
	lx := &lexer{src: src, line: 1, col: 1}
	var toks []token
	for {
		t, err := lx.next()
		if err != nil {
			return nil, err
		}
		toks = append(toks, t)
		if t.kind == tokEOF {
			return toks, nil
		}
	}
}

func (lx *lexer) errorf(line, col int, format string, args ...any) *syntaxError {
	return &syntaxError{line: line, col: col, msg: fmt.Sprintf(format, args...)}
}

// step consumes one rune, keeping line and column up to date.
func (lx *lexer) step() {
	r, size := utf8.DecodeRuneInString(lx.src[lx.pos:])
	lx.pos += size
	if r == '\n' {
		lx.line++
		lx.col = 1
	} else {
		lx.col++
	}
}

func (lx *lexer) stepN(n int) {
	for i := 0; i < n && lx.pos < len(lx.src); i++ {
		lx.step()
	}
}

func (lx *lexer) peekByte() byte {
	if lx.pos >= len(lx.src) {
		return 0
	}
	return lx.src[lx.pos]
}

func (lx *lexer) hasPrefix(s string) bool { return strings.HasPrefix(lx.src[lx.pos:], s) }

func (lx *lexer) next() (token, *syntaxError) {
	adjacent, err := lx.skipSpace()
	if err != nil {
		return token{}, err
	}
	tok := token{line: lx.line, col: lx.col, start: lx.pos, adjacent: adjacent}
	if lx.pos >= len(lx.src) {
		tok.kind = tokEOF
		tok.end = lx.pos
		return tok, nil
	}
	c := lx.peekByte()
	r, _ := utf8.DecodeRuneInString(lx.src[lx.pos:])
	switch {
	case lx.hasPrefix("'''"):
		tok.kind = tokString
		tok.text, err = lx.multiline()
	case c == '\'' || c == '"':
		tok.kind = tokString
		tok.text, err = lx.quoted(c)
	case c == '`':
		tok.kind = tokExpr
		tok.text, err = lx.expression()
	case c >= '0' && c <= '9':
		tok.kind = tokNumber
		tok.text = lx.number()
	case c == '#':
		tok.kind = tokWord
		tok.text = lx.word()
	case r == '_' || unicode.IsLetter(r):
		tok.kind = tokWord
		tok.text = lx.word()
	case strings.IndexByte("{}[](),:.<>-", c) >= 0:
		tok.kind = tokSymbol
		if lx.hasPrefix("<>") {
			tok.text = "<>"
			lx.stepN(2)
		} else {
			tok.text = string(c)
			lx.step()
		}
	default:
		return token{}, lx.errorf(tok.line, tok.col, "unexpected character %q", r)
	}
	if err != nil {
		return token{}, err
	}
	tok.end = lx.pos
	return tok, nil
}

// skipSpace skips whitespace and comments and reports whether nothing was
// skipped (the next token is adjacent to the previous one).
func (lx *lexer) skipSpace() (adjacent bool, err *syntaxError) {
	adjacent = true
	for lx.pos < len(lx.src) {
		switch {
		case lx.peekByte() == ' ' || lx.peekByte() == '\t' || lx.peekByte() == '\r' || lx.peekByte() == '\n':
			lx.step()
		case lx.hasPrefix("//"):
			for lx.pos < len(lx.src) && lx.peekByte() != '\n' {
				lx.step()
			}
		case lx.hasPrefix("/*"):
			line, col := lx.line, lx.col
			end := strings.Index(lx.src[lx.pos+2:], "*/")
			if end < 0 {
				return false, lx.errorf(line, col, "unterminated block comment")
			}
			lx.stepN(utf8.RuneCountInString(lx.src[lx.pos : lx.pos+2+end+2]))
		default:
			return adjacent, nil
		}
		adjacent = false
	}
	return adjacent, nil
}

func (lx *lexer) word() string {
	start := lx.pos
	lx.step() // first rune was checked by the caller
	for lx.pos < len(lx.src) {
		r, _ := utf8.DecodeRuneInString(lx.src[lx.pos:])
		if r != '_' && !unicode.IsLetter(r) && !unicode.IsDigit(r) {
			break
		}
		lx.step()
	}
	return lx.src[start:lx.pos]
}

func (lx *lexer) number() string {
	start := lx.pos
	digits := func() {
		for lx.pos < len(lx.src) && lx.peekByte() >= '0' && lx.peekByte() <= '9' {
			lx.step()
		}
	}
	digits()
	if lx.peekByte() == '.' && lx.pos+1 < len(lx.src) && lx.src[lx.pos+1] >= '0' && lx.src[lx.pos+1] <= '9' {
		lx.step()
		digits()
	}
	return lx.src[start:lx.pos]
}

// quoted reads a single- or double-quoted string, decoding backslash escapes.
func (lx *lexer) quoted(q byte) (string, *syntaxError) {
	line, col := lx.line, lx.col
	lx.step() // opening quote
	var b strings.Builder
	for lx.pos < len(lx.src) {
		c := lx.peekByte()
		switch {
		case c == q:
			lx.step()
			return b.String(), nil
		case c == '\n':
			return "", lx.errorf(line, col, "unterminated string")
		case c == '\\' && lx.pos+1 < len(lx.src):
			lx.step()
			switch e := lx.peekByte(); e {
			case 'n':
				b.WriteByte('\n')
			case 't':
				b.WriteByte('\t')
			case 'r':
				b.WriteByte('\r')
			case '\\', '\'', '"':
				b.WriteByte(e)
			default:
				// Not an escape: keep the backslash and let the loop read
				// the next character normally.
				b.WriteByte('\\')
				continue
			}
			lx.step()
		default:
			_, size := utf8.DecodeRuneInString(lx.src[lx.pos:])
			b.WriteString(lx.src[lx.pos : lx.pos+size])
			lx.step()
		}
	}
	return "", lx.errorf(line, col, "unterminated string")
}

// multiline reads a triple-quoted string. As in DBML, the newline after the opening
// quotes and the line holding the closing quotes are dropped, and the
// indentation shared by all lines is removed.
func (lx *lexer) multiline() (string, *syntaxError) {
	line, col := lx.line, lx.col
	body := lx.src[lx.pos+3:]
	end := strings.Index(body, "'''")
	if end < 0 {
		return "", lx.errorf(line, col, "unterminated multi-line string")
	}
	raw := body[:end]
	lx.stepN(utf8.RuneCountInString(raw) + 6)
	return dedent(raw), nil
}

func dedent(raw string) string {
	raw = strings.ReplaceAll(raw, "\r\n", "\n")
	raw = strings.TrimPrefix(raw, "\n")
	lines := strings.Split(raw, "\n")
	if n := len(lines); n > 0 && strings.TrimSpace(lines[n-1]) == "" {
		lines = lines[:n-1]
	}
	indent := -1
	for _, l := range lines {
		if strings.TrimSpace(l) == "" {
			continue
		}
		n := len(l) - len(strings.TrimLeft(l, " \t"))
		if indent < 0 || n < indent {
			indent = n
		}
	}
	for i, l := range lines {
		cut := len(l) - len(strings.TrimLeft(l, " \t"))
		if cut > indent {
			cut = indent
		}
		lines[i] = l[cut:]
	}
	return strings.Join(lines, "\n")
}

func (lx *lexer) expression() (string, *syntaxError) {
	line, col := lx.line, lx.col
	body := lx.src[lx.pos+1:]
	end := strings.IndexByte(body, '`')
	if end < 0 {
		return "", lx.errorf(line, col, "unterminated expression")
	}
	raw := body[:end]
	lx.stepN(utf8.RuneCountInString(raw) + 2)
	return raw, nil
}
