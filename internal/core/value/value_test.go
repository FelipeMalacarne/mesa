package value

import (
	"encoding/json"
	"math"
	"testing"
	"time"
)

func roundTrip(t *testing.T, v Value) Value {
	t.Helper()
	data, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal %v: %v", v, err)
	}
	var out Value
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatalf("unmarshal %s: %v", data, err)
	}
	if !Equal(v, out) {
		t.Fatalf("round trip changed %s: %v → %v", data, v, out)
	}
	return out
}

func TestRoundTrip(t *testing.T) {
	ts := time.Date(2026, 10, 20, 14, 2, 11, 123456000, time.FixedZone("", -3*3600))
	for _, v := range []Value{
		Null(), Bool(true), Int(42), Int(math.MaxInt64), Int(math.MinInt64), BigInt("123456789012345678901234567890"),
		Float(1.5), Float(math.NaN()), Decimal("19900.00"), Text("Ana Souza"), Bytes([]byte{0, 1, 255}),
		UUID("2F1E0A7C-9F2B-4C7E-9E5A-0A1B2C3D4E5F"), JSON(`{"a":[1,2,{"b":null}]}`), Time(ts), TimeText("1 day 02:00:00"),
		Array([]Value{Int(1), Null(), Text("x")}), Other("inet", "10.0.0.1/32"),
	} {
		roundTrip(t, v)
	}
}

func TestWireShape(t *testing.T) {
	data, _ := json.Marshal(Int(7).WithType("int8"))
	if string(data) != `{"k":"int","v":7,"t":"int8"}` {
		t.Errorf("wire form = %s", data)
	}
	data, _ = json.Marshal(Int(1 << 60))
	if string(data) != `{"k":"int","v":"1152921504606846976"}` {
		t.Errorf("large int wire form = %s", data)
	}
	data, _ = json.Marshal(Null())
	if string(data) != `{"k":"null"}` {
		t.Errorf("null wire form = %s", data)
	}
}

func TestText(t *testing.T) {
	if Bytes([]byte{0xde, 0xad}).Text() != `\xdead` {
		t.Error("bytes text")
	}
	if Array([]Value{Int(1), Text("a")}).Text() != "{1,a}" {
		t.Error("array text")
	}
}

func TestParseText(t *testing.T) {
	v, err := ParseText(KindInt, "int4", " 19900 ")
	if err != nil || v.AsInt() != 19900 || v.Type != "int4" {
		t.Fatalf("ParseText int: %v %v", v, err)
	}
	if _, err := ParseText(KindInt, "int4", "abc"); err == nil {
		t.Error("expected error for bad int")
	}
	v, _ = ParseText(KindBool, "bool", "yes")
	if !v.AsBool() {
		t.Error("bool yes")
	}
	v, _ = ParseText(KindTime, "timestamptz", "2026-10-20T14:02:11Z")
	if v.AsTime().IsZero() {
		t.Error("time parse")
	}
	if _, err := ParseText(KindJSON, "jsonb", "{bad"); err == nil {
		t.Error("bad json accepted")
	}
}
