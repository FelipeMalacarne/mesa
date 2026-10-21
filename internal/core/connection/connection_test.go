package connection

import (
	"testing"
	"time"
)

func TestNewDefaults(t *testing.T) {
	c, err := New(Input{Name: " billing ", Engine: "PostgreSQL", Host: "10.0.4.12", User: "mesa"}, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if c.Name != "billing" || c.Engine != Postgres || c.Port != 5432 || c.Database != "postgres" || c.TLS != TLSPrefer || c.Environment != Local || c.ReadOnly {
		t.Errorf("defaults wrong: %+v", c)
	}
}

func TestProductionIsReadOnly(t *testing.T) {
	no := false
	c, err := New(Input{Name: "b", Engine: "postgres", Environment: "production", Host: "h", User: "u", ReadOnly: &no}, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if !c.ReadOnly {
		t.Error("production must be read-only even when the input says otherwise")
	}
}

func TestValidation(t *testing.T) {
	bad := []Input{
		{Engine: "postgres", Host: "h", User: "u"},
		{Name: "x", Engine: "oracle", Host: "h", User: "u"},
		{Name: "x", Engine: "postgres", User: "u"},
		{Name: "x", Engine: "postgres", Host: "h", User: "u", Port: 70000},
		{Name: "x", Engine: "postgres", Host: "h", User: "u", TLS: "maybe"},
		{Name: "x", Engine: "postgres", Host: "h", User: "u", Environment: "prod"},
	}
	for i, in := range bad {
		if _, err := New(in, time.Now()); err == nil {
			t.Errorf("case %d accepted %+v", i, in)
		}
	}
}
