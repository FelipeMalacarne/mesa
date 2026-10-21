// Package value is the single representation of a cell value between a
// driver and a transport. Values keep their database meaning (an int8 is not a
// float, a numeric keeps every digit, a timestamptz keeps its zone) and have
// one JSON wire form, documented on Encode.
package value

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/felipemalacarne/mesa/internal/core/errs"
)

// Kind is the logical type of a value.
type Kind string

const (
	KindNull    Kind = "null"
	KindBool    Kind = "bool"
	KindInt     Kind = "int"     // any integer; carried as int64 (strings beyond that)
	KindFloat   Kind = "float"   // float4/float8
	KindDecimal Kind = "decimal" // numeric/decimal, exact digits as text
	KindText    Kind = "text"
	KindBytes   Kind = "bytes"
	KindUUID    Kind = "uuid"
	KindJSON    Kind = "json" // json/jsonb, raw JSON text
	KindTime    Kind = "time" // date, time, timestamp, timestamptz, interval as text when not instant
	KindArray   Kind = "array"
	KindOther   Kind = "other" // engine type Mesa does not model; textual form
)

// Value is one cell. Exactly one of the payload fields is meaningful for a kind.
type Value struct {
	Kind Kind
	// Type is the engine's type name (int8, timestamptz, jsonb…) when known.
	Type string

	b bool
	i int64
	f float64
	s string
	t time.Time
	a []Value
	// Text holds the textual form for Decimal, UUID, JSON, Other, Time-as-text
	// and Int values that overflow int64.
}

// Constructors.

func Null() Value                       { return Value{Kind: KindNull} }
func Bool(v bool) Value                 { return Value{Kind: KindBool, b: v} }
func Int(v int64) Value                 { return Value{Kind: KindInt, i: v} }
func BigInt(digits string) Value        { return Value{Kind: KindInt, s: digits} }
func Float(v float64) Value             { return Value{Kind: KindFloat, f: v} }
func Decimal(digits string) Value       { return Value{Kind: KindDecimal, s: digits} }
func Text(v string) Value               { return Value{Kind: KindText, s: v} }
func Bytes(v []byte) Value              { return Value{Kind: KindBytes, s: string(v)} }
func UUID(v string) Value               { return Value{Kind: KindUUID, s: strings.ToLower(v)} }
func JSON(raw string) Value             { return Value{Kind: KindJSON, s: raw} }
func Time(v time.Time) Value            { return Value{Kind: KindTime, t: v} }
func TimeText(v string) Value           { return Value{Kind: KindTime, s: v} }
func Array(items []Value) Value         { return Value{Kind: KindArray, a: items} }
func Other(typ, text string) Value      { return Value{Kind: KindOther, Type: typ, s: text} }
func (v Value) WithType(t string) Value { v.Type = t; return v }

// Accessors.

func (v Value) IsNull() bool      { return v.Kind == KindNull }
func (v Value) AsBool() bool      { return v.b }
func (v Value) AsInt() int64      { return v.i }
func (v Value) AsFloat() float64  { return v.f }
func (v Value) AsTime() time.Time { return v.t }
func (v Value) AsBytes() []byte   { return []byte(v.s) }
func (v Value) Items() []Value    { return v.a }

// Text returns the textual form of any value, the way a person would type it
// into a cell. Null returns "".
func (v Value) Text() string {
	switch v.Kind {
	case KindNull:
		return ""
	case KindBool:
		if v.b {
			return "true"
		}
		return "false"
	case KindInt:
		if v.s != "" {
			return v.s
		}
		return strconv.FormatInt(v.i, 10)
	case KindFloat:
		return strconv.FormatFloat(v.f, 'g', -1, 64)
	case KindTime:
		if v.s != "" {
			return v.s
		}
		return v.t.Format(time.RFC3339Nano)
	case KindBytes:
		return "\\x" + fmt.Sprintf("%x", []byte(v.s))
	case KindArray:
		parts := make([]string, len(v.a))
		for i, it := range v.a {
			parts[i] = it.Text()
		}
		return "{" + strings.Join(parts, ",") + "}"
	default:
		return v.s
	}
}

// Equal reports whether two values are the same cell value.
func Equal(a, b Value) bool {
	if a.Kind != b.Kind {
		return false
	}
	switch a.Kind {
	case KindNull:
		return true
	case KindBool:
		return a.b == b.b
	case KindInt:
		return a.i == b.i && a.s == b.s
	case KindFloat:
		return a.f == b.f || (math.IsNaN(a.f) && math.IsNaN(b.f))
	case KindTime:
		if a.s != "" || b.s != "" {
			return a.s == b.s
		}
		return a.t.Equal(b.t)
	case KindArray:
		if len(a.a) != len(b.a) {
			return false
		}
		for i := range a.a {
			if !Equal(a.a[i], b.a[i]) {
				return false
			}
		}
		return true
	default:
		return a.s == b.s
	}
}

// wire is the JSON form: {"k": kind, "v": payload, "t": type?}. Payloads:
// bool → bool; int → number when it fits in a float64 exactly, else string;
// float → number (NaN/Inf as string); decimal, uuid, json, other, text → string
// (json carries the raw JSON text, not parsed, so numbers stay exact);
// bytes → base64 string; time → RFC 3339 string or the textual form; array →
// list of wire values. Null is {"k":"null"}.
type wire struct {
	K Kind            `json:"k"`
	V json.RawMessage `json:"v,omitempty"`
	T string          `json:"t,omitempty"`
}

const maxExactFloat = 1 << 53

// MarshalJSON encodes the wire form.
func (v Value) MarshalJSON() ([]byte, error) {
	w := wire{K: v.Kind, T: v.Type}
	var payload any
	switch v.Kind {
	case KindNull:
		return json.Marshal(w)
	case KindBool:
		payload = v.b
	case KindInt:
		switch {
		case v.s != "":
			payload = v.s
		case v.i > -maxExactFloat && v.i < maxExactFloat:
			payload = v.i
		default:
			payload = strconv.FormatInt(v.i, 10)
		}
	case KindFloat:
		if math.IsNaN(v.f) || math.IsInf(v.f, 0) {
			payload = strconv.FormatFloat(v.f, 'g', -1, 64)
		} else {
			payload = v.f
		}
	case KindBytes:
		payload = base64.StdEncoding.EncodeToString([]byte(v.s))
	case KindTime:
		payload = v.Text()
	case KindArray:
		payload = v.a
	default:
		payload = v.s
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	w.V = raw
	return json.Marshal(w)
}

// UnmarshalJSON decodes the wire form.
func (v *Value) UnmarshalJSON(data []byte) error {
	var w wire
	if err := json.Unmarshal(data, &w); err != nil {
		return errs.Wrap(errs.Invalid, "value is not valid", err)
	}
	out, err := decode(w)
	if err != nil {
		return err
	}
	*v = out
	return nil
}

func decode(w wire) (Value, error) {
	v := Value{Kind: w.K, Type: w.T}
	if w.K == KindNull {
		return v, nil
	}
	if w.K == "" {
		return v, errs.New(errs.Invalid, "value has no kind")
	}
	switch w.K {
	case KindBool:
		return v, json.Unmarshal(w.V, &v.b)
	case KindInt:
		var n json.Number
		d := json.NewDecoder(strings.NewReader(string(w.V)))
		d.UseNumber()
		if err := d.Decode(&n); err != nil {
			var s string
			if err2 := json.Unmarshal(w.V, &s); err2 != nil {
				return v, errs.Wrap(errs.Invalid, "int value", err)
			}
			n = json.Number(s)
		}
		i, err := strconv.ParseInt(n.String(), 10, 64)
		if err != nil {
			if _, ok := new(big).SetString(n.String()); !ok {
				return v, errs.Newf(errs.Invalid, "int value %q", n.String())
			}
			v.s = n.String()
			return v, nil
		}
		v.i = i
		return v, nil
	case KindFloat:
		if err := json.Unmarshal(w.V, &v.f); err != nil {
			var s string
			if err2 := json.Unmarshal(w.V, &s); err2 != nil {
				return v, errs.Wrap(errs.Invalid, "float value", err)
			}
			f, err3 := strconv.ParseFloat(s, 64)
			if err3 != nil {
				return v, errs.Wrap(errs.Invalid, "float value", err3)
			}
			v.f = f
		}
		return v, nil
	case KindBytes:
		var s string
		if err := json.Unmarshal(w.V, &s); err != nil {
			return v, errs.Wrap(errs.Invalid, "bytes value", err)
		}
		b, err := base64.StdEncoding.DecodeString(s)
		if err != nil {
			return v, errs.Wrap(errs.Invalid, "bytes value", err)
		}
		v.s = string(b)
		return v, nil
	case KindTime:
		var s string
		if err := json.Unmarshal(w.V, &s); err != nil {
			return v, errs.Wrap(errs.Invalid, "time value", err)
		}
		if t, err := time.Parse(time.RFC3339Nano, s); err == nil {
			v.t = t
		} else {
			v.s = s
		}
		return v, nil
	case KindArray:
		var items []Value
		if err := json.Unmarshal(w.V, &items); err != nil {
			return v, errs.Wrap(errs.Invalid, "array value", err)
		}
		v.a = items
		return v, nil
	case KindDecimal, KindText, KindUUID, KindJSON, KindOther:
		var s string
		if err := json.Unmarshal(w.V, &s); err != nil {
			return v, errs.Wrap(errs.Invalid, string(w.K)+" value", err)
		}
		v.s = s
		if w.K == KindUUID {
			v.s = strings.ToLower(v.s)
		}
		return v, nil
	default:
		return v, errs.Newf(errs.Invalid, "unknown value kind %q", w.K)
	}
}

// big is a minimal digit-string validator so int values beyond int64 round-trip.
type big struct{}

func (big) SetString(s string) (big, bool) {
	if s == "" {
		return big{}, false
	}
	for i, r := range s {
		if (r == '-' || r == '+') && i == 0 {
			continue
		}
		if r < '0' || r > '9' {
			return big{}, false
		}
	}
	return big{}, true
}

// Row is one result row.
type Row []Value

// Column describes a result column.
type Column struct {
	Name string
	Type string // engine type name
	Kind Kind   // the kind values in this column decode to
}

// ParseText turns typed-in text into a value of the given kind, the inverse
// of Text. It is what the grid editor uses when a person edits a cell.
func ParseText(kind Kind, typ, text string) (Value, error) {
	switch kind {
	case KindNull:
		return Null().WithType(typ), nil
	case KindBool:
		switch strings.ToLower(strings.TrimSpace(text)) {
		case "true", "t", "yes", "1":
			return Bool(true).WithType(typ), nil
		case "false", "f", "no", "0":
			return Bool(false).WithType(typ), nil
		}
		return Value{}, errs.Newf(errs.Invalid, "%q is not a boolean", text)
	case KindInt:
		i, err := strconv.ParseInt(strings.TrimSpace(text), 10, 64)
		if err != nil {
			if _, ok := (big{}).SetString(strings.TrimSpace(text)); ok {
				return BigInt(strings.TrimSpace(text)).WithType(typ), nil
			}
			return Value{}, errs.Newf(errs.Invalid, "%q is not an integer", text)
		}
		return Int(i).WithType(typ), nil
	case KindFloat:
		f, err := strconv.ParseFloat(strings.TrimSpace(text), 64)
		if err != nil {
			return Value{}, errs.Newf(errs.Invalid, "%q is not a number", text)
		}
		return Float(f).WithType(typ), nil
	case KindDecimal:
		s := strings.TrimSpace(text)
		if _, err := strconv.ParseFloat(s, 64); err != nil {
			return Value{}, errs.Newf(errs.Invalid, "%q is not a decimal", text)
		}
		return Decimal(s).WithType(typ), nil
	case KindUUID:
		s := strings.ToLower(strings.TrimSpace(text))
		if len(s) != 36 {
			return Value{}, errs.Newf(errs.Invalid, "%q is not a UUID", text)
		}
		return UUID(s).WithType(typ), nil
	case KindJSON:
		if !json.Valid([]byte(text)) {
			return Value{}, errs.New(errs.Invalid, "not valid JSON")
		}
		return JSON(text).WithType(typ), nil
	case KindTime:
		s := strings.TrimSpace(text)
		if t, err := time.Parse(time.RFC3339Nano, s); err == nil {
			return Time(t).WithType(typ), nil
		}
		return TimeText(s).WithType(typ), nil
	case KindBytes:
		s := strings.TrimPrefix(strings.TrimSpace(text), "\\x")
		b := make([]byte, len(s)/2)
		if _, err := fmt.Sscanf(s, "%x", &b); err != nil && s != "" {
			return Value{}, errs.Newf(errs.Invalid, "%q is not hex bytes", text)
		}
		return Bytes(b).WithType(typ), nil
	case KindArray:
		return Value{}, errs.New(errs.Unsupported, "editing arrays as text is not supported yet")
	default:
		return Other(typ, text), nil
	}
}
