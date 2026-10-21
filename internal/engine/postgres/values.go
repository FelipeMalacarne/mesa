package postgres

import (
	"context"
	"math"
	"strconv"
	"strings"
	"sync"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/felipemalacarne/mesa/internal/core/value"
)

// Parameters and results travel in PostgreSQL's text format. The server
// parses the text of every type exactly as it would a literal typed into a
// SQL editor, so one encoder covers every column type, including enums,
// domains and extension types, and the decoder keeps the exact digits of
// numerics and the zone of timestamps.

// typeInfo is what the decoder needs to know about one type OID.
type typeInfo struct {
	Name     string // as pg_type spells it: int8, varchar, _text
	Category byte   // pg_type.typcategory ('A' for arrays, 'E' for enums…)
	Elem     uint32 // element type of an array
}

// typeRegistry resolves type OIDs. pgx knows the built-in types; anything
// else (enums, extension types, their arrays) is looked up in pg_type on
// the connection that produced the result and cached for the session.
type typeRegistry struct {
	builtin *pgtype.Map
	mu      sync.Mutex
	cache   map[uint32]typeInfo
}

func newTypeRegistry() *typeRegistry {
	return &typeRegistry{builtin: pgtype.NewMap(), cache: map[uint32]typeInfo{}}
}

const typeLookupSQL = "select t.typname::text, t.typcategory::text, t.typelem from pg_type t where t.oid = $1"

// lookup resolves oid, using pc for catalog lookups of unknown types.
func (r *typeRegistry) lookup(ctx context.Context, pc *pgconn.PgConn, oid uint32) typeInfo {
	if t, ok := r.builtin.TypeForOID(oid); ok {
		info := typeInfo{Name: t.Name}
		if ac, ok := t.Codec.(*pgtype.ArrayCodec); ok {
			info.Category = 'A'
			info.Elem = ac.ElementType.OID
		}
		return info
	}
	r.mu.Lock()
	info, ok := r.cache[oid]
	r.mu.Unlock()
	if ok {
		return info
	}
	info = typeInfo{Name: "oid:" + strconv.FormatUint(uint64(oid), 10)}
	if pc == nil {
		return info
	}
	rr := pc.ExecParams(ctx, typeLookupSQL, [][]byte{[]byte(strconv.FormatUint(uint64(oid), 10))}, nil, nil, nil)
	var row [][]byte
	if rr.NextRow() {
		row = copyRow(rr.Values())
	}
	if _, err := rr.Close(); err != nil || len(row) != 3 {
		return info
	}
	info.Name = string(row[0])
	if len(row[1]) > 0 {
		info.Category = row[1][0]
	}
	if elem, err := strconv.ParseUint(string(row[2]), 10, 32); err == nil {
		info.Elem = uint32(elem)
	}
	r.mu.Lock()
	r.cache[oid] = info
	r.mu.Unlock()
	return info
}

// kind is the value kind a type decodes to.
func (t typeInfo) kind() value.Kind {
	if t.Category == 'A' || (strings.HasPrefix(t.Name, "_") && t.Elem != 0) {
		return value.KindArray
	}
	switch t.Name {
	case "bool":
		return value.KindBool
	case "int2", "int4", "int8":
		return value.KindInt
	case "float4", "float8":
		return value.KindFloat
	case "numeric":
		return value.KindDecimal
	case "text", "varchar", "bpchar", "char", "name":
		return value.KindText
	case "bytea":
		return value.KindBytes
	case "uuid":
		return value.KindUUID
	case "json", "jsonb":
		return value.KindJSON
	case "date", "timestamp", "timestamptz", "time", "timetz", "interval":
		return value.KindTime
	}
	return value.KindOther
}

// displayName is the type name shown to people: arrays as elem[].
func (r *typeRegistry) displayName(ctx context.Context, pc *pgconn.PgConn, t typeInfo) string {
	if t.kind() == value.KindArray && t.Elem != 0 {
		return r.displayName(ctx, pc, r.lookup(ctx, pc, t.Elem)) + "[]"
	}
	return t.Name
}

// decode turns the text form of one cell into a value.
func (r *typeRegistry) decode(ctx context.Context, pc *pgconn.PgConn, t typeInfo, src []byte) value.Value {
	typ := r.displayName(ctx, pc, t)
	if src == nil {
		return value.Null().WithType(typ)
	}
	s := string(src)
	switch t.kind() {
	case value.KindBool:
		return value.Bool(s == "t" || s == "true").WithType(typ)
	case value.KindInt:
		if i, err := strconv.ParseInt(s, 10, 64); err == nil {
			return value.Int(i).WithType(typ)
		}
		return value.BigInt(s).WithType(typ)
	case value.KindFloat:
		if f, err := strconv.ParseFloat(s, 64); err == nil {
			return value.Float(f).WithType(typ)
		}
		return value.Other(typ, s)
	case value.KindDecimal:
		return value.Decimal(s).WithType(typ)
	case value.KindText:
		return value.Text(s).WithType(typ)
	case value.KindBytes:
		var b []byte
		if err := r.builtin.Scan(pgtype.ByteaOID, pgtype.TextFormatCode, src, &b); err != nil {
			return value.Other(typ, s)
		}
		return value.Bytes(b).WithType(typ)
	case value.KindUUID:
		return value.UUID(s).WithType(typ)
	case value.KindJSON:
		return value.JSON(s).WithType(typ)
	case value.KindTime:
		return r.decodeTime(t.Name, src).WithType(typ)
	case value.KindArray:
		node, err := parseArrayText(s)
		if err != nil {
			return value.Other(typ, s)
		}
		elem := r.lookup(ctx, pc, t.Elem)
		return r.decodeArray(ctx, pc, elem, node).WithType(typ)
	}
	return value.Other(typ, s)
}

func (r *typeRegistry) decodeTime(name string, src []byte) value.Value {
	switch name {
	case "date":
		var d pgtype.Date
		if err := r.builtin.Scan(pgtype.DateOID, pgtype.TextFormatCode, src, &d); err == nil && d.Valid && d.InfinityModifier == pgtype.Finite {
			return value.Time(d.Time)
		}
	case "timestamp":
		var ts pgtype.Timestamp
		if err := r.builtin.Scan(pgtype.TimestampOID, pgtype.TextFormatCode, src, &ts); err == nil && ts.Valid && ts.InfinityModifier == pgtype.Finite {
			return value.Time(ts.Time)
		}
	case "timestamptz":
		var ts pgtype.Timestamptz
		if err := r.builtin.Scan(pgtype.TimestamptzOID, pgtype.TextFormatCode, src, &ts); err == nil && ts.Valid && ts.InfinityModifier == pgtype.Finite {
			return value.Time(ts.Time)
		}
	}
	// time, timetz, interval and anything that is not an instant.
	return value.TimeText(string(src))
}

func (r *typeRegistry) decodeArray(ctx context.Context, pc *pgconn.PgConn, elem typeInfo, node arrayNode) value.Value {
	if node.null {
		return value.Null()
	}
	if !node.array {
		return r.decode(ctx, pc, elem, []byte(node.text))
	}
	items := make([]value.Value, len(node.items))
	for i, it := range node.items {
		items[i] = r.decodeArray(ctx, pc, elem, it)
	}
	return value.Array(items)
}

// encodeArgs renders parameters in the text format; nil is SQL NULL.
func encodeArgs(args []value.Value) [][]byte {
	if len(args) == 0 {
		return nil
	}
	out := make([][]byte, len(args))
	for i, a := range args {
		out[i] = encodeArg(a)
	}
	return out
}

func encodeArg(v value.Value) []byte {
	if v.IsNull() {
		return nil
	}
	return []byte(paramText(v))
}

// paramText is the text PostgreSQL parses for v. It differs from v.Text()
// only where the display form is not valid input.
func paramText(v value.Value) string {
	switch v.Kind {
	case value.KindFloat:
		f := v.AsFloat()
		switch {
		case math.IsNaN(f):
			return "NaN"
		case math.IsInf(f, 1):
			return "Infinity"
		case math.IsInf(f, -1):
			return "-Infinity"
		}
		return strconv.FormatFloat(f, 'g', -1, 64)
	case value.KindArray:
		return arrayLiteral(v)
	}
	return v.Text()
}

// arrayLiteral renders an array value as PostgreSQL array input: every
// element double-quoted and escaped, NULL bare, nested arrays in braces.
func arrayLiteral(v value.Value) string {
	var b strings.Builder
	writeArrayLiteral(&b, v)
	return b.String()
}

func writeArrayLiteral(b *strings.Builder, v value.Value) {
	b.WriteByte('{')
	for i, it := range v.Items() {
		if i > 0 {
			b.WriteByte(',')
		}
		switch it.Kind {
		case value.KindNull:
			b.WriteString("NULL")
		case value.KindArray:
			writeArrayLiteral(b, it)
		default:
			b.WriteByte('"')
			for _, c := range []byte(paramText(it)) {
				if c == '"' || c == '\\' {
					b.WriteByte('\\')
				}
				b.WriteByte(c)
			}
			b.WriteByte('"')
		}
	}
	b.WriteByte('}')
}

// arrayNode is one element of a parsed array literal.
type arrayNode struct {
	array bool
	items []arrayNode
	null  bool
	text  string
}

// parseArrayText parses PostgreSQL's array output ({a,"b c",NULL,{1,2}}),
// including an optional dimension prefix ([1:2]={…}).
func parseArrayText(s string) (arrayNode, error) {
	if strings.HasPrefix(s, "[") {
		if i := strings.IndexByte(s, '='); i >= 0 {
			s = s[i+1:]
		}
	}
	p := &arrayParser{s: s}
	node, err := p.parseArray()
	if err != nil {
		return arrayNode{}, err
	}
	if p.pos != len(p.s) {
		return arrayNode{}, errArrayText
	}
	return node, nil
}

type arrayTextError struct{}

func (arrayTextError) Error() string { return "malformed array text" }

var errArrayText error = arrayTextError{}

type arrayParser struct {
	s   string
	pos int
}

func (p *arrayParser) parseArray() (arrayNode, error) {
	if p.pos >= len(p.s) || p.s[p.pos] != '{' {
		return arrayNode{}, errArrayText
	}
	p.pos++
	node := arrayNode{array: true, items: []arrayNode{}}
	if p.pos < len(p.s) && p.s[p.pos] == '}' {
		p.pos++
		return node, nil
	}
	for {
		item, err := p.parseItem()
		if err != nil {
			return arrayNode{}, err
		}
		node.items = append(node.items, item)
		if p.pos >= len(p.s) {
			return arrayNode{}, errArrayText
		}
		switch p.s[p.pos] {
		case ',':
			p.pos++
		case '}':
			p.pos++
			return node, nil
		default:
			return arrayNode{}, errArrayText
		}
	}
}

func (p *arrayParser) parseItem() (arrayNode, error) {
	if p.pos >= len(p.s) {
		return arrayNode{}, errArrayText
	}
	switch p.s[p.pos] {
	case '{':
		return p.parseArray()
	case '"':
		p.pos++
		var b strings.Builder
		for p.pos < len(p.s) {
			c := p.s[p.pos]
			switch c {
			case '\\':
				if p.pos+1 >= len(p.s) {
					return arrayNode{}, errArrayText
				}
				b.WriteByte(p.s[p.pos+1])
				p.pos += 2
			case '"':
				p.pos++
				return arrayNode{text: b.String()}, nil
			default:
				b.WriteByte(c)
				p.pos++
			}
		}
		return arrayNode{}, errArrayText
	}
	start := p.pos
	for p.pos < len(p.s) && p.s[p.pos] != ',' && p.s[p.pos] != '}' {
		p.pos++
	}
	text := p.s[start:p.pos]
	if strings.EqualFold(text, "NULL") {
		return arrayNode{null: true}, nil
	}
	return arrayNode{text: text}, nil
}

// copyRow duplicates a row buffer pgconn reuses between rows.
func copyRow(src [][]byte) [][]byte {
	out := make([][]byte, len(src))
	for i, c := range src {
		if c != nil {
			out[i] = append([]byte(nil), c...)
		}
	}
	return out
}
