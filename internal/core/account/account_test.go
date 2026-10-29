package account

import (
	"context"
	"testing"
	"time"

	"github.com/felipemalacarne/mesa/internal/core/errs"
)

var now = time.Date(2026, time.October, 29, 12, 0, 0, 0, time.UTC)

func TestRoles(t *testing.T) {
	cases := []struct {
		role               Role
		read, write, admin bool
	}{
		{Viewer, true, false, false},
		{Editor, true, true, false},
		{Admin, true, true, true},
		{Role("root"), false, false, false},
	}
	for _, c := range cases {
		if c.role.Can(Read) != c.read || c.role.Can(Write) != c.write || c.role.Can(Manage) != c.admin {
			t.Errorf("%s: read %v write %v manage %v", c.role, c.role.Can(Read), c.role.Can(Write), c.role.Can(Manage))
		}
	}
	if _, err := ParseRole(" Editor "); err != nil {
		t.Error(err)
	}
	if _, err := ParseRole("owner"); !errs.Is(err, errs.Invalid) {
		t.Errorf("unknown role: %v", err)
	}
}

func TestNewUser(t *testing.T) {
	u, err := New(Input{Subject: "https://auth.example.com 42", Email: " Ana@Example.com ", Role: Viewer}, now)
	if err != nil {
		t.Fatal(err)
	}
	if u.Email != "ana@example.com" || u.Name != "ana" || u.Role != Viewer || u.Label() != "ana@example.com" {
		t.Errorf("user = %+v", u)
	}
	if err := u.Update(Input{Subject: u.Subject, Name: "Ana Lima", Role: Admin}, now); err != nil || u.Label() != "Ana Lima" || u.Role != Admin {
		t.Errorf("update = %+v %v", u, err)
	}
	if _, err := New(Input{Email: "a@b.c", Role: Viewer}, now); !errs.Is(err, errs.Invalid) {
		t.Errorf("no subject: %v", err)
	}
	if _, err := New(Input{Subject: "s", Role: "boss"}, now); !errs.Is(err, errs.Invalid) {
		t.Errorf("role: %v", err)
	}
}

func TestRequire(t *testing.T) {
	ctx := context.Background()
	if err := Require(ctx, Read); !errs.Is(err, errs.Unauthenticated) {
		t.Errorf("no principal: %v", err)
	}
	if err := Require(With(ctx, Local()), Manage); err != nil {
		t.Errorf("local principal: %v", err)
	}
	viewer, _ := New(Input{Subject: "s1", Email: "v@example.com", Role: Viewer}, now)
	vctx := With(ctx, Of(viewer))
	if err := Require(vctx, Read); err != nil {
		t.Errorf("viewer read: %v", err)
	}
	if err := Require(vctx, Write); !errs.Is(err, errs.Forbidden) {
		t.Errorf("viewer write: %v", err)
	}
	if got := Actor(vctx, "web"); got != "v@example.com" {
		t.Errorf("Actor = %q", got)
	}
	if got := Actor(With(ctx, Local()), "cli"); got != "cli" {
		t.Errorf("local Actor = %q", got)
	}
}
