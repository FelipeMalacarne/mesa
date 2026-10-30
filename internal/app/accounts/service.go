// Package accounts is how people get into a Mesa server: they sign in
// through an OpenID Connect provider (Zitadel, Keycloak, …), which also says
// their role; Mesa keeps a session for the browser and API tokens for agents
// and scripts. Mesa stores no passwords and manages no members.
//
// Only a server with a provider configured has accounts. The desktop app,
// the CLI, the MCP server on stdio, and a server without a provider run as
// the local principal (account.Local) and never reach this package.
package accounts

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/felipemalacarne/mesa/internal/core/account"
	"github.com/felipemalacarne/mesa/internal/core/errs"
	"github.com/felipemalacarne/mesa/internal/store"
)

const (
	// SessionTTL is how long a browser stays signed in. Short, because the
	// provider decides roles: a role taken away there ends here within a
	// working day. Signing in again is one redirect while the provider's
	// own session lasts.
	SessionTTL = 12 * time.Hour
	// apiTokenPrefix marks Mesa's API tokens, so a leaked one is easy to
	// recognise in a log or a secret scanner.
	apiTokenPrefix = "mesa_"
	// touchEvery bounds how often a request records last-used times.
	touchEvery = time.Minute
)

// Provider is an OpenID Connect provider, ready to send people to and to
// take them back from.
type Provider interface {
	// AuthCodeURL is where to send the browser. verifier is the PKCE
	// secret; the provider sees only its hash.
	AuthCodeURL(redirectURL, state, nonce, verifier string) string
	// Exchange trades the code the browser came back with for the verified
	// identity of the person.
	Exchange(ctx context.Context, redirectURL, code, verifier, nonce string) (*Identity, error)
}

// Identity is who the provider says signed in, with every claim it shared
// (from the ID token and the userinfo endpoint).
type Identity struct {
	Issuer  string
	Subject string
	Email   string
	Name    string
	Claims  map[string]any
}

// ProviderConfig is what a Provider needs.
type ProviderConfig struct {
	Issuer       string
	ClientID     string
	ClientSecret string // empty for a public client (PKCE only)
	// Scopes are asked for beyond openid, email and profile; Zitadel needs
	// urn:zitadel:iam:org:projects:roles to share project roles.
	Scopes []string
}

// ProviderFactory discovers a provider; it reaches the issuer.
type ProviderFactory func(ctx context.Context, c ProviderConfig) (Provider, error)

// Options configure New.
type Options struct {
	Provider ProviderConfig
	// Label names the provider on the sign-in button.
	Label string
	// RolesClaim is the claim that lists roles; empty reads Zitadel's
	// project roles, then "roles", then "groups".
	RolesClaim string
	// DefaultRole is given to people the provider names no Mesa role for;
	// empty keeps them out.
	DefaultRole account.Role
	// Providers builds the provider (oidc.New).
	Providers ProviderFactory
	// Now is the clock; time.Now when nil.
	Now func() time.Time
}

// Service is the accounts use case.
type Service struct {
	users  store.Users
	tokens store.Tokens
	o      Options
	now    func() time.Time

	mu       sync.Mutex
	provider Provider
}

// New wires the service over Mesa's store. The provider is discovered at
// the first sign-in, so Mesa starts while the provider is down.
func New(st store.Store, o Options) *Service {
	now := o.Now
	if now == nil {
		now = time.Now
	}
	if strings.TrimSpace(o.Label) == "" {
		o.Label = "single sign-on"
	}
	return &Service{users: st.Users(), tokens: st.Tokens(), o: o, now: now}
}

// Label names the provider on the sign-in button.
func (s *Service) Label() string { return s.o.Label }

// SignedIn is the outcome of a sign-in: the user and the session secret the
// transport hands to the browser (a cookie), never to page scripts.
type SignedIn struct {
	User    account.User
	Session string
	Expires time.Time
}

// --- signing in through the provider ----------------------------------------------

// Flow is what the browser keeps (in a short-lived cookie) between leaving
// for the provider and coming back.
type Flow struct {
	State    string `json:"s"`
	Nonce    string `json:"n"`
	Verifier string `json:"v"`
}

func (s *Service) discover(ctx context.Context) (Provider, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.provider != nil {
		return s.provider, nil
	}
	if s.o.Providers == nil {
		return nil, errs.New(errs.Unsupported, "no identity provider is configured")
	}
	p, err := s.o.Providers(ctx, s.o.Provider)
	if err != nil {
		return nil, errs.Wrap(errs.Unavailable, "cannot reach the identity provider at "+s.o.Provider.Issuer, err)
	}
	s.provider = p
	return p, nil
}

// Begin starts a sign-in: where to send the browser, and what it must bring
// back.
func (s *Service) Begin(ctx context.Context, redirectURL string) (string, *Flow, error) {
	p, err := s.discover(ctx)
	if err != nil {
		return "", nil, err
	}
	var f Flow
	for _, v := range []*string{&f.State, &f.Nonce, &f.Verifier} {
		if *v, err = randomSecret(); err != nil {
			return "", nil, err
		}
	}
	return p.AuthCodeURL(redirectURL, f.State, f.Nonce, f.Verifier), &f, nil
}

// Finish completes a sign-in. The provider's claims decide the role; a
// person it names no Mesa role for stays out (unless DefaultRole is set).
func (s *Service) Finish(ctx context.Context, redirectURL, code, state string, f *Flow) (*SignedIn, error) {
	if f == nil || !sameSecret(state, f.State) {
		return nil, errs.New(errs.Unauthenticated, "the sign-in expired or came from elsewhere; start again")
	}
	p, err := s.discover(ctx)
	if err != nil {
		return nil, err
	}
	id, err := p.Exchange(ctx, redirectURL, code, f.Verifier, f.Nonce)
	if err != nil {
		return nil, errs.Wrap(errs.Unauthenticated, "the identity provider did not confirm the sign-in", err)
	}
	role, ok := RoleFrom(id.Claims, s.o.RolesClaim)
	if !ok {
		if s.o.DefaultRole == "" {
			who := id.Email
			if who == "" {
				who = id.Name
			}
			return nil, errs.Newf(errs.Forbidden, "%s has no Mesa role; ask an admin to grant admin, editor or viewer in the identity provider", who)
		}
		role = s.o.DefaultRole
	}
	in := account.Input{Subject: id.Issuer + " " + id.Subject, Email: id.Email, Name: id.Name, Role: role}
	now := s.now()
	u, err := s.users.GetBySubject(ctx, in.Subject)
	switch {
	case errs.Is(err, errs.NotFound):
		nu, err := account.New(in, now)
		if err != nil {
			return nil, err
		}
		u = &nu
	case err != nil:
		return nil, err
	default:
		if err := u.Update(in, now); err != nil {
			return nil, err
		}
	}
	if err := s.users.Put(ctx, *u); err != nil {
		return nil, err
	}
	return s.startSession(ctx, *u)
}

// RoleFrom reads the Mesa role in a provider's claims: the highest of admin,
// editor and viewer it finds. Names may carry a "mesa" prefix (mesa-admin,
// mesa:admin, mesa_admin, mesa.admin), for a provider shared by many apps.
// With claim empty it reads Zitadel's project roles
// (urn:zitadel:iam:org:project:roles and urn:zitadel:iam:org:project:<id>:roles),
// then "roles", then "groups".
func RoleFrom(claims map[string]any, claim string) (account.Role, bool) {
	var names []string
	if claim != "" {
		names = claimNames(claims[claim])
	} else {
		for k, v := range claims {
			if k == "urn:zitadel:iam:org:project:roles" ||
				(strings.HasPrefix(k, "urn:zitadel:iam:org:project:") && strings.HasSuffix(k, ":roles")) {
				names = append(names, claimNames(v)...)
			}
		}
		if len(names) == 0 {
			names = append(claimNames(claims["roles"]), claimNames(claims["groups"])...)
		}
	}
	best, found := account.Role(""), false
	rank := map[account.Role]int{account.Viewer: 1, account.Editor: 2, account.Admin: 3}
	for _, n := range names {
		n = strings.ToLower(strings.TrimSpace(n))
		for _, sep := range []string{"mesa-", "mesa:", "mesa_", "mesa."} {
			n = strings.TrimPrefix(n, sep)
		}
		r := account.Role(n)
		if rank[r] > rank[best] {
			best, found = r, true
		}
	}
	return best, found
}

// claimNames reads a list of names or the keys of an object.
func claimNames(v any) []string {
	var out []string
	switch t := v.(type) {
	case []any:
		for _, e := range t {
			if s, ok := e.(string); ok {
				out = append(out, s)
			}
		}
	case []string:
		out = append(out, t...)
	case map[string]any:
		for k := range t {
			out = append(out, k)
		}
	case string:
		out = append(out, strings.Fields(t)...)
	}
	return out
}

// --- sessions and tokens -----------------------------------------------------------

// Authenticate turns a session or API token into the user it stands for,
// and the token's ID.
func (s *Service) Authenticate(ctx context.Context, credential string) (account.User, uuid.UUID, error) {
	ended := errs.New(errs.Unauthenticated, "your session has ended; sign in again")
	if credential == "" {
		return account.User{}, uuid.Nil, errs.New(errs.Unauthenticated, "sign in to Mesa first")
	}
	tok, err := s.tokens.ByHash(ctx, hashToken(credential))
	if errs.Is(err, errs.NotFound) {
		return account.User{}, uuid.Nil, ended
	}
	if err != nil {
		return account.User{}, uuid.Nil, err
	}
	now := s.now()
	if tok.ExpiresAt != nil && !now.Before(*tok.ExpiresAt) {
		_ = s.tokens.Delete(ctx, tok.ID)
		return account.User{}, uuid.Nil, ended
	}
	u, err := s.users.Get(ctx, tok.UserID)
	if errs.Is(err, errs.NotFound) {
		return account.User{}, uuid.Nil, ended
	}
	if err != nil {
		return account.User{}, uuid.Nil, err
	}
	if tok.LastUsedAt == nil || now.Sub(*tok.LastUsedAt) >= touchEvery {
		_ = s.tokens.Touch(ctx, tok.ID, now)
		_ = s.users.TouchSeen(ctx, u.ID, now)
	}
	return *u, tok.ID, nil
}

// SignOut ends the session a credential belongs to.
func (s *Service) SignOut(ctx context.Context, credential string) error {
	tok, err := s.tokens.ByHash(ctx, hashToken(credential))
	if errs.Is(err, errs.NotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	return s.tokens.Delete(ctx, tok.ID)
}

func (s *Service) startSession(ctx context.Context, u account.User) (*SignedIn, error) {
	secret, err := randomSecret()
	if err != nil {
		return nil, err
	}
	now := s.now()
	expires := now.Add(SessionTTL)
	id, err := uuid.NewV7()
	if err != nil {
		return nil, err
	}
	err = s.tokens.Create(ctx, store.Token{
		ID: id, UserID: u.ID, Kind: store.SessionToken, Hash: hashToken(secret),
		CreatedAt: now, ExpiresAt: &expires, LastUsedAt: &now,
	})
	if err != nil {
		return nil, err
	}
	_ = s.users.TouchSeen(ctx, u.ID, now)
	_ = s.tokens.Prune(ctx, now)
	seen := now
	u.LastSeenAt = &seen
	return &SignedIn{User: u, Session: secret, Expires: expires}, nil
}

// Me returns the signed-in user.
func (s *Service) Me(ctx context.Context) (*account.User, error) {
	p, ok := account.From(ctx)
	if !ok || p.User == nil {
		return nil, errs.New(errs.Unauthenticated, "sign in to Mesa first")
	}
	u := *p.User
	return &u, nil
}

// NewToken is a token just made: Secret is shown once and never again.
type NewToken struct {
	store.Token
	Secret string
}

// CreateToken makes an API token for the signed-in user. It acts with the
// role the provider gave at their last sign-in. expiresIn of zero never
// expires.
func (s *Service) CreateToken(ctx context.Context, name string, expiresIn time.Duration) (*NewToken, error) {
	me, err := s.Me(ctx)
	if err != nil {
		return nil, err
	}
	name = strings.TrimSpace(name)
	if name == "" || len(name) > 100 {
		return nil, errs.New(errs.Invalid, "give the token a name of up to 100 characters")
	}
	if expiresIn < 0 {
		return nil, errs.New(errs.Invalid, "the expiry must be in the future")
	}
	raw, err := randomSecret()
	if err != nil {
		return nil, err
	}
	secret := apiTokenPrefix + raw
	id, err := uuid.NewV7()
	if err != nil {
		return nil, err
	}
	now := s.now()
	tok := store.Token{ID: id, UserID: me.ID, Kind: store.APIToken, Name: name, Hash: hashToken(secret), CreatedAt: now}
	if expiresIn > 0 {
		exp := now.Add(expiresIn)
		tok.ExpiresAt = &exp
	}
	if err := s.tokens.Create(ctx, tok); err != nil {
		return nil, err
	}
	return &NewToken{Token: tok, Secret: secret}, nil
}

// Tokens lists the signed-in user's API tokens.
func (s *Service) Tokens(ctx context.Context) ([]store.Token, error) {
	me, err := s.Me(ctx)
	if err != nil {
		return nil, err
	}
	return s.tokens.List(ctx, me.ID, store.APIToken)
}

// RevokeToken deletes one of the signed-in user's API tokens.
func (s *Service) RevokeToken(ctx context.Context, id uuid.UUID) error {
	me, err := s.Me(ctx)
	if err != nil {
		return err
	}
	list, err := s.tokens.List(ctx, me.ID, store.APIToken)
	if err != nil {
		return err
	}
	for _, t := range list {
		if t.ID == id {
			return s.tokens.Delete(ctx, id)
		}
	}
	return errs.New(errs.NotFound, "no such token")
}

// People lists everyone who has signed in, with the role the provider last
// gave them (admins).
func (s *Service) People(ctx context.Context) ([]account.User, error) {
	if err := account.Require(ctx, account.Manage); err != nil {
		return nil, err
	}
	return s.users.List(ctx)
}

func randomSecret() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func hashToken(secret string) []byte {
	sum := sha256.Sum256([]byte(secret))
	return sum[:]
}

func sameSecret(a, b string) bool {
	return a != "" && subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}
