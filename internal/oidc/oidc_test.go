package oidc

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	jose "github.com/go-jose/go-jose/v4"

	"github.com/felipemalacarne/mesa/internal/app/accounts"
)

// issuer is a tiny OpenID provider: discovery, keys, a token endpoint that
// checks PKCE, and userinfo.
type issuer struct {
	srv       *httptest.Server
	key       *rsa.PrivateKey
	challenge string
	nonce     string
	claims    map[string]any
	userinfo  map[string]any
}

func newIssuer(t *testing.T) *issuer {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	is := &issuer{key: key}
	mux := http.NewServeMux()
	mux.HandleFunc("/.well-known/openid-configuration", func(w http.ResponseWriter, _ *http.Request) {
		u := is.srv.URL
		_ = json.NewEncoder(w).Encode(map[string]any{
			"issuer": u, "authorization_endpoint": u + "/authorize", "token_endpoint": u + "/token",
			"jwks_uri": u + "/keys", "userinfo_endpoint": u + "/userinfo",
			"id_token_signing_alg_values_supported": []string{"RS256"},
		})
	})
	mux.HandleFunc("/keys", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(jose.JSONWebKeySet{Keys: []jose.JSONWebKey{{Key: &key.PublicKey, KeyID: "k1", Algorithm: "RS256", Use: "sig"}}})
	})
	mux.HandleFunc("/token", func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		sum := sha256.Sum256([]byte(r.Form.Get("code_verifier")))
		if r.Form.Get("code") != "good" || base64.RawURLEncoding.EncodeToString(sum[:]) != is.challenge {
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": "invalid_grant"})
			return
		}
		claims := map[string]any{
			"iss": is.srv.URL, "sub": "user-1", "aud": "mesa", "nonce": is.nonce,
			"iat": time.Now().Unix(), "exp": time.Now().Add(time.Hour).Unix(),
		}
		for k, v := range is.claims {
			claims[k] = v
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"access_token": "at", "token_type": "Bearer", "expires_in": 3600, "id_token": is.sign(t, claims),
		})
	})
	mux.HandleFunc("/userinfo", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer at" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(is.userinfo)
	})
	is.srv = httptest.NewServer(mux)
	t.Cleanup(is.srv.Close)
	return is
}

func (is *issuer) sign(t *testing.T, claims map[string]any) string {
	t.Helper()
	signer, err := jose.NewSigner(jose.SigningKey{Algorithm: jose.RS256, Key: is.key}, (&jose.SignerOptions{}).WithHeader("kid", "k1"))
	if err != nil {
		t.Fatal(err)
	}
	payload, _ := json.Marshal(claims)
	obj, err := signer.Sign(payload)
	if err != nil {
		t.Fatal(err)
	}
	s, err := obj.CompactSerialize()
	if err != nil {
		t.Fatal(err)
	}
	return s
}

// authorize plays the browser: it follows the auth URL's parameters the way
// the provider would remember them.
func (is *issuer) authorize(t *testing.T, authURL string) {
	t.Helper()
	u, err := url.Parse(authURL)
	if err != nil {
		t.Fatal(err)
	}
	q := u.Query()
	if q.Get("code_challenge_method") != "S256" || q.Get("client_id") != "mesa" || !strings.Contains(q.Get("scope"), "email") {
		t.Fatalf("auth URL = %s", authURL)
	}
	is.challenge, is.nonce = q.Get("code_challenge"), q.Get("nonce")
}

func TestSignIn(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	is := newIssuer(t)
	p, err := New(ctx, accounts.ProviderConfig{Issuer: is.srv.URL, ClientID: "mesa", ClientSecret: "s"})
	if err != nil {
		t.Fatal(err)
	}
	// The request that discovered the provider is long gone by the time
	// someone signs in.
	cancel()
	const redirect = "https://mesa.example.com/auth/sso/callback"

	is.claims = map[string]any{"email": "ana@example.com", "name": "Ana"}
	is.authorize(t, p.AuthCodeURL(redirect, "state", "nonce-1", "verifier-1"))
	id, err := p.Exchange(context.Background(), redirect, "good", "verifier-1", "nonce-1")
	if err != nil {
		t.Fatalf("Exchange: %v", err)
	}
	if id.Issuer != is.srv.URL || id.Subject != "user-1" || id.Email != "ana@example.com" || id.Name != "Ana" {
		t.Fatalf("identity = %+v", *id)
	}

	// What the ID token leaves out (Zitadel's email and project roles),
	// userinfo has; the ID token wins where both speak.
	is.claims = map[string]any{"name": "Ana from the token"}
	is.userinfo = map[string]any{
		"sub": "user-1", "email": "ana@example.com", "name": "Ana from userinfo", "preferred_username": "ana",
		"urn:zitadel:iam:org:project:roles": map[string]any{"admin": map[string]any{"1": "gielinor"}},
	}
	is.authorize(t, p.AuthCodeURL(redirect, "state", "nonce-2", "verifier-2"))
	if id, err = p.Exchange(context.Background(), redirect, "good", "verifier-2", "nonce-2"); err != nil {
		t.Fatalf("Exchange with userinfo: %v", err)
	}
	if id.Email != "ana@example.com" || id.Name != "Ana from the token" {
		t.Fatalf("userinfo identity = %+v", *id)
	}
	if role, ok := accounts.RoleFrom(id.Claims, ""); !ok || role != "admin" {
		t.Fatalf("roles from userinfo = %q %v (claims %v)", role, ok, id.Claims)
	}

	// A nonce from another sign-in, a wrong verifier and a stranger's
	// userinfo are refused.
	is.authorize(t, p.AuthCodeURL(redirect, "state", "nonce-3", "verifier-3"))
	if _, err := p.Exchange(context.Background(), redirect, "good", "verifier-3", "nonce-x"); err == nil {
		t.Error("nonce mismatch accepted")
	}
	if _, err := p.Exchange(context.Background(), redirect, "good", "verifier-x", "nonce-3"); err == nil {
		t.Error("wrong PKCE verifier accepted")
	}
	is.userinfo = map[string]any{"sub": "someone-else", "email": "eve@example.com", "email_verified": true}
	if _, err := p.Exchange(context.Background(), redirect, "good", "verifier-3", "nonce-3"); err == nil {
		t.Error("userinfo of another subject accepted")
	}
}

func TestDiscoveryFails(t *testing.T) {
	srv := httptest.NewServer(http.NotFoundHandler())
	defer srv.Close()
	if _, err := New(context.Background(), accounts.ProviderConfig{Issuer: srv.URL, ClientID: "mesa"}); err == nil {
		t.Fatal("a server without discovery is not a provider")
	}
}
