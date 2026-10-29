package accounts

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/felipemalacarne/mesa/internal/core/account"
	"github.com/felipemalacarne/mesa/internal/core/errs"
	"github.com/felipemalacarne/mesa/internal/store"
	"github.com/felipemalacarne/mesa/internal/store/sqlite"
)

const redirect = "https://mesa.example.com/auth/callback"

type fixture struct {
	svc *Service
	st  store.Store
	now time.Time
	idp *fakeProvider
}

func setup(t *testing.T, o Options) *fixture {
	t.Helper()
	st, err := sqlite.Open(context.Background(), sqlite.Memory)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	f := &fixture{st: st, now: time.Date(2026, time.October, 29, 12, 0, 0, 0, time.UTC), idp: &fakeProvider{}}
	o.Provider = ProviderConfig{Issuer: "https://auth.example.com", ClientID: "mesa"}
	o.Now = func() time.Time { return f.now }
	o.Providers = func(context.Context, ProviderConfig) (Provider, error) {
		if f.idp.down {
			return nil, errors.New("dial tcp: connection refused")
		}
		return f.idp, nil
	}
	f.svc = New(st, o)
	return f
}

type fakeProvider struct {
	down     bool
	identity Identity
	verifier string
}

func (p *fakeProvider) AuthCodeURL(redirectURL, state, _, verifier string) string {
	p.verifier = verifier
	return "https://auth.example.com/authorize?redirect_uri=" + redirectURL + "&state=" + state
}

func (p *fakeProvider) Exchange(_ context.Context, _, code, verifier, _ string) (*Identity, error) {
	if code != "good" || verifier != p.verifier {
		return nil, errors.New("invalid_grant")
	}
	id := p.identity
	return &id, nil
}

// signIn runs the redirect round trip for the provider's current identity.
func (f *fixture) signIn(t *testing.T) (*SignedIn, error) {
	t.Helper()
	to, flow, err := f.svc.Begin(context.Background(), redirect)
	if err != nil {
		t.Fatalf("Begin: %v", err)
	}
	if !strings.Contains(to, flow.State) {
		t.Fatalf("auth URL %q lacks the state", to)
	}
	return f.svc.Finish(context.Background(), redirect, "good", flow.State, flow)
}

func zitadel(sub, email string, roles ...string) Identity {
	r := map[string]any{}
	for _, name := range roles {
		r[name] = map[string]any{"123": "gielinor.auth.example.com"}
	}
	return Identity{
		Issuer: "https://auth.example.com", Subject: sub, Email: email,
		Claims: map[string]any{"urn:zitadel:iam:org:project:roles": r},
	}
}

func TestSignInWithRoles(t *testing.T) {
	f := setup(t, Options{Label: "Zitadel"})
	if f.svc.Label() != "Zitadel" {
		t.Errorf("Label = %q", f.svc.Label())
	}
	f.idp.identity = zitadel("100", "Felipe@example.com", "viewer", "admin")
	in, err := f.signIn(t)
	if err != nil {
		t.Fatalf("sign-in: %v", err)
	}
	if in.User.Role != account.Admin || in.User.Email != "felipe@example.com" || in.User.Subject != "https://auth.example.com 100" {
		t.Fatalf("user = %+v", in.User)
	}
	if !in.Expires.Equal(f.now.Add(SessionTTL)) || len(in.Session) < 40 {
		t.Errorf("session %q until %v", in.Session, in.Expires)
	}
	u, _, err := f.svc.Authenticate(context.Background(), in.Session)
	if err != nil || u.ID != in.User.ID {
		t.Fatalf("Authenticate: %+v %v", u, err)
	}

	// The provider changes the role: the next sign-in brings it, same user.
	f.idp.identity = zitadel("100", "felipe@example.com", "viewer")
	again, err := f.signIn(t)
	if err != nil || again.User.ID != in.User.ID || again.User.Role != account.Viewer {
		t.Fatalf("second sign-in: %+v %v", again, err)
	}

	// Nobody without a Mesa role gets in.
	f.idp.identity = zitadel("200", "eve@example.com", "billing")
	if _, err := f.signIn(t); !errs.Is(err, errs.Forbidden) || !strings.Contains(err.Error(), "eve@example.com") {
		t.Fatalf("no role: %v", err)
	}
	if people, _ := f.st.Users().List(context.Background()); len(people) != 1 {
		t.Fatalf("a refused sign-in stored a user: %d", len(people))
	}
}

func TestDefaultRole(t *testing.T) {
	f := setup(t, Options{DefaultRole: account.Viewer})
	f.idp.identity = Identity{Issuer: "https://auth.example.com", Subject: "300", Name: "Bo"}
	in, err := f.signIn(t)
	if err != nil || in.User.Role != account.Viewer || in.User.Name != "Bo" {
		t.Fatalf("default role: %+v %v", in, err)
	}
}

func TestFlowChecks(t *testing.T) {
	f := setup(t, Options{})
	f.idp.identity = zitadel("100", "felipe@example.com", "admin")
	_, flow, err := f.svc.Begin(context.Background(), redirect)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.Finish(context.Background(), redirect, "good", "tampered", flow); !errs.Is(err, errs.Unauthenticated) {
		t.Fatalf("wrong state: %v", err)
	}
	if _, err := f.svc.Finish(context.Background(), redirect, "good", flow.State, nil); !errs.Is(err, errs.Unauthenticated) {
		t.Fatalf("no flow cookie: %v", err)
	}
	if _, err := f.svc.Finish(context.Background(), redirect, "bad", flow.State, flow); !errs.Is(err, errs.Unauthenticated) {
		t.Fatalf("bad code: %v", err)
	}
	down := setup(t, Options{})
	down.idp.down = true
	if _, _, err := down.svc.Begin(context.Background(), redirect); !errs.Is(err, errs.Unavailable) {
		t.Fatalf("provider down: %v", err)
	}
}

func TestSessionsAndTokens(t *testing.T) {
	f := setup(t, Options{})
	ctx := context.Background()
	f.idp.identity = zitadel("100", "felipe@example.com", "editor")
	in, err := f.signIn(t)
	if err != nil {
		t.Fatal(err)
	}
	u, tokID, err := f.svc.Authenticate(ctx, in.Session)
	if err != nil {
		t.Fatal(err)
	}
	me := account.With(ctx, account.Of(u))
	_ = tokID

	tok, err := f.svc.CreateToken(me, "claude", 0)
	if err != nil || !strings.HasPrefix(tok.Secret, apiTokenPrefix) {
		t.Fatalf("CreateToken: %+v %v", tok, err)
	}
	if got, _, err := f.svc.Authenticate(ctx, tok.Secret); err != nil || got.Role != account.Editor {
		t.Fatalf("token authenticates: %+v %v", got, err)
	}
	if list, _ := f.svc.Tokens(me); len(list) != 1 || list[0].Name != "claude" {
		t.Fatalf("Tokens = %+v", list)
	}
	if _, err := f.svc.People(me); !errs.Is(err, errs.Forbidden) {
		t.Fatalf("an editor lists people: %v", err)
	}
	if err := f.svc.RevokeToken(me, tok.ID); err != nil {
		t.Fatal(err)
	}
	if _, _, err := f.svc.Authenticate(ctx, tok.Secret); !errs.Is(err, errs.Unauthenticated) {
		t.Fatalf("revoked token: %v", err)
	}
	if err := f.svc.SignOut(ctx, in.Session); err != nil {
		t.Fatal(err)
	}
	if _, _, err := f.svc.Authenticate(ctx, in.Session); !errs.Is(err, errs.Unauthenticated) {
		t.Fatalf("after SignOut: %v", err)
	}
	short, _ := f.svc.CreateToken(me, "ci", time.Hour)
	f.now = f.now.Add(2 * time.Hour)
	if _, _, err := f.svc.Authenticate(ctx, short.Secret); !errs.Is(err, errs.Unauthenticated) {
		t.Fatalf("expired token: %v", err)
	}
}

func TestRoleFrom(t *testing.T) {
	cases := []struct {
		claims map[string]any
		claim  string
		want   account.Role
		ok     bool
	}{
		{map[string]any{"urn:zitadel:iam:org:project:2981:roles": map[string]any{"editor": map[string]any{}}}, "", account.Editor, true},
		{map[string]any{"roles": []any{"viewer", "mesa-admin"}}, "", account.Admin, true},
		{map[string]any{"groups": []any{"mesa:editor", "staff"}}, "", account.Editor, true},
		{map[string]any{"mesa_roles": "viewer"}, "mesa_roles", account.Viewer, true},
		{map[string]any{"roles": []any{"owner"}}, "", "", false},
		{map[string]any{}, "", "", false},
	}
	for i, c := range cases {
		got, ok := RoleFrom(c.claims, c.claim)
		if got != c.want || ok != c.ok {
			t.Errorf("case %d: RoleFrom = %q %v, want %q %v", i, got, ok, c.want, c.ok)
		}
	}
}
