// Package account holds the people who use a Mesa server: their roles,
// what each role may do, and the principal a request runs as. People sign in
// through an identity provider, which also says their role. A principal
// travels in the context; every use case that changes something asks it for
// permission, so no surface can skip the check.
package account

import (
	"context"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/felipemalacarne/mesa/internal/core/errs"
)

// Role is what a person may do on this Mesa.
type Role string

const (
	// Viewer browses, runs read-only queries and watches Monitor.
	Viewer Role = "viewer"
	// Editor also applies changes (still reviewed by policy), unlocks
	// read-only connections, saves queries and cancels sessions.
	Editor Role = "editor"
	// Admin also manages connections and provisions databases.
	Admin Role = "admin"
)

// ParseRole accepts the three roles.
func ParseRole(s string) (Role, error) {
	switch r := Role(strings.ToLower(strings.TrimSpace(s))); r {
	case Viewer, Editor, Admin:
		return r, nil
	}
	return "", errs.Newf(errs.Invalid, "unknown role %q (viewer, editor or admin)", s)
}

// Permission is one kind of action a use case guards.
type Permission int

const (
	// Read covers everything that only looks.
	Read Permission = iota
	// Write covers changes to databases and to shared Mesa data.
	Write
	// Manage covers connections and provisioning databases.
	Manage
)

func (p Permission) String() string {
	switch p {
	case Write:
		return "write"
	case Manage:
		return "manage"
	default:
		return "read"
	}
}

// Can reports whether the role grants p.
func (r Role) Can(p Permission) bool {
	switch r {
	case Admin:
		return true
	case Editor:
		return p <= Write
	case Viewer:
		return p == Read
	}
	return false
}

// User is a person who signed in through the identity provider. Mesa keeps
// no passwords: the provider decides who may sign in and with which role,
// and Mesa records what it last said.
type User struct {
	ID    uuid.UUID `json:"id"`
	Email string    `json:"email"`
	Name  string    `json:"name"`
	Role  Role      `json:"role"`
	// Subject is the provider's stable ID for the person: "<issuer> <sub>".
	Subject    string     `json:"-"`
	CreatedAt  time.Time  `json:"createdAt"`
	UpdatedAt  time.Time  `json:"updatedAt"`
	LastSeenAt *time.Time `json:"lastSeenAt,omitempty"`
}

// Input is what the identity provider says about a person.
type Input struct {
	Subject string
	Email   string
	Name    string
	Role    Role
}

// New returns a user with a fresh ID.
func New(in Input, now time.Time) (User, error) {
	id, err := uuid.NewV7()
	if err != nil {
		return User{}, err
	}
	u := User{ID: id, CreatedAt: now}
	return u, u.Update(in, now)
}

// Update records what the provider says now: email, name and role change
// there, never in Mesa.
func (u *User) Update(in Input, now time.Time) error {
	if strings.TrimSpace(in.Subject) == "" {
		return errs.New(errs.Invalid, "the identity provider gave no subject")
	}
	if _, err := ParseRole(string(in.Role)); err != nil {
		return err
	}
	email := strings.ToLower(strings.TrimSpace(in.Email))
	name := strings.TrimSpace(in.Name)
	if name == "" {
		name, _, _ = strings.Cut(email, "@")
	}
	if name == "" {
		name = in.Subject
	}
	if len(name) > 200 || len(email) > 320 {
		return errs.New(errs.Invalid, "name or email is too long")
	}
	u.Subject, u.Email, u.Name, u.Role, u.UpdatedAt = in.Subject, email, name, in.Role, now
	return nil
}

// Label is how the audit log names the user: the email, else the name.
func (u User) Label() string {
	if u.Email != "" {
		return u.Email
	}
	return u.Name
}

// Principal is who a request runs as: a signed-in user, or the person at the
// keyboard of a local surface (desktop, CLI, MCP on stdio), who may do
// everything.
type Principal struct {
	User  *User
	local bool
}

// Local is the principal of a surface without accounts.
func Local() Principal { return Principal{local: true} }

// Of is the principal of a signed-in user.
func Of(u User) Principal { return Principal{User: &u} }

// IsLocal reports whether p is the local principal.
func (p Principal) IsLocal() bool { return p.local }

// Can reports whether p may do what perm covers.
func (p Principal) Can(perm Permission) bool {
	if p.local {
		return true
	}
	return p.User != nil && p.User.Role.Can(perm)
}

type principalKey struct{}

// With returns ctx carrying p.
func With(ctx context.Context, p Principal) context.Context {
	return context.WithValue(ctx, principalKey{}, p)
}

// From returns the principal in ctx, if any.
func From(ctx context.Context) (Principal, bool) {
	p, ok := ctx.Value(principalKey{}).(Principal)
	return p, ok
}

// Require returns nil when the principal in ctx may do what perm covers. A
// context without a principal is refused: every surface sets one.
func Require(ctx context.Context, perm Permission) error {
	p, ok := From(ctx)
	if !ok || (!p.local && p.User == nil) {
		return errs.New(errs.Unauthenticated, "sign in to Mesa first")
	}
	if !p.Can(perm) {
		return errs.Newf(errs.Forbidden, "your role (%s) cannot %s; ask an admin", p.User.Role, perm.verb())
	}
	return nil
}

func (p Permission) verb() string {
	switch p {
	case Write:
		return "change data or settings of a database"
	case Manage:
		return "manage connections or provision databases"
	default:
		return "read"
	}
}

// Actor names the principal in ctx for the audit log: the user, or fallback
// for a local surface.
func Actor(ctx context.Context, fallback string) string {
	if p, ok := From(ctx); ok && p.User != nil {
		return p.User.Label()
	}
	return fallback
}
