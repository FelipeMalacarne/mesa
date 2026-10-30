package http_test

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/felipemalacarne/mesa/internal/app"
	"github.com/felipemalacarne/mesa/internal/app/accounts"
	"github.com/felipemalacarne/mesa/internal/platform"
	"github.com/felipemalacarne/mesa/internal/platform/bootstrap"
	"github.com/felipemalacarne/mesa/internal/secret"
	"github.com/felipemalacarne/mesa/internal/session"
	httptransport "github.com/felipemalacarne/mesa/internal/transport/http"
	mcptransport "github.com/felipemalacarne/mesa/internal/transport/mcp"
)

// provider stands in for Zitadel: whoever signs in next gets these roles.
type provider struct {
	sub      string
	roles    []string
	verifier string
}

func (p *provider) AuthCodeURL(redirectURL, state, _, verifier string) string {
	p.verifier = verifier
	return "https://auth.example.com/authorize?" + url.Values{"redirect_uri": {redirectURL}, "state": {state}}.Encode()
}

func (p *provider) Exchange(_ context.Context, _, code, verifier, _ string) (*accounts.Identity, error) {
	if code != "good" || verifier != p.verifier {
		return nil, errors.New("invalid_grant")
	}
	roles := map[string]any{}
	for _, r := range p.roles {
		roles[r] = map[string]any{}
	}
	return &accounts.Identity{
		Issuer: "https://auth.example.com", Subject: p.sub, Email: p.sub + "@example.com",
		Claims: map[string]any{"urn:zitadel:iam:org:project:roles": roles},
	}, nil
}

func newServer(t *testing.T, idp *provider) *httptest.Server {
	t.Helper()
	ctx := context.Background()
	st, err := bootstrap.OpenStore(ctx, platform.Config{StoreDriver: "sqlite", StoreDSN: ":memory:"})
	if err != nil {
		t.Fatal(err)
	}
	vault, err := secret.NewLocal("test-key", st.Secrets())
	if err != nil {
		t.Fatal(err)
	}
	o := app.Options{Mode: app.ModeServer, Store: st, Vault: vault, Session: session.Options{AppName: "mesa-test"}}
	if idp != nil {
		o.Accounts = &accounts.Options{
			Label:     "Zitadel",
			Provider:  accounts.ProviderConfig{Issuer: "https://auth.example.com", ClientID: "mesa"},
			Providers: func(context.Context, accounts.ProviderConfig) (accounts.Provider, error) { return idp, nil },
		}
	}
	a := app.New(o)
	srv := httptest.NewServer(httptransport.New(a, httptransport.Options{
		MCP: mcptransport.Handler(a, mcptransport.Options{}),
	}))
	t.Cleanup(func() {
		srv.Close()
		_ = a.Close()
	})
	return srv
}

type call struct {
	method, path, body string
	cookie, bearer     string
	origin             string
}

func do(t *testing.T, srv *httptest.Server, c call) (*http.Response, string) {
	t.Helper()
	req, err := http.NewRequest(c.method, srv.URL+c.path, strings.NewReader(c.body))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	if c.cookie != "" {
		req.Header.Set("Cookie", c.cookie)
	}
	if c.bearer != "" {
		req.Header.Set("Authorization", "Bearer "+c.bearer)
	}
	if c.origin != "" {
		req.Header.Set("Origin", c.origin)
	}
	client := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	res, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	b, _ := io.ReadAll(res.Body)
	_ = res.Body.Close()
	return res, string(b)
}

func cookie(t *testing.T, res *http.Response, name string) *http.Cookie {
	t.Helper()
	for _, c := range res.Cookies() {
		if c.Name == name {
			return c
		}
	}
	t.Fatalf("no %s cookie in %v", name, res.Header)
	return nil
}

// signIn plays the browser: /auth/login, the provider, /auth/callback.
func signIn(t *testing.T, srv *httptest.Server) (*http.Response, string) {
	t.Helper()
	res, _ := do(t, srv, call{method: "GET", path: "/auth/login"})
	if res.StatusCode != http.StatusFound {
		t.Fatalf("/auth/login = %d", res.StatusCode)
	}
	to, err := url.Parse(res.Header.Get("Location"))
	if err != nil || to.Host != "auth.example.com" || to.Query().Get("redirect_uri") != srv.URL+"/auth/callback" {
		t.Fatalf("login redirects to %q", res.Header.Get("Location"))
	}
	flow := cookie(t, res, "mesa_signin")
	if !flow.HttpOnly {
		t.Error("the sign-in flow cookie must be HttpOnly")
	}
	q := url.Values{"code": {"good"}, "state": {to.Query().Get("state")}}
	return do(t, srv, call{method: "GET", path: "/auth/callback?" + q.Encode(), cookie: "mesa_signin=" + flow.Value})
}

func TestLocalMode(t *testing.T) {
	srv := newServer(t, nil)
	res, body := do(t, srv, call{method: "GET", path: "/api/auth/state"})
	if res.StatusCode != 200 || !strings.Contains(body, `"mode":"local"`) {
		t.Fatalf("state = %d %s", res.StatusCode, body)
	}
	res, body = do(t, srv, call{method: "POST", path: "/api/connections", body: `{"name":"x","engine":"postgres","host":"h","user":"u"}`})
	if res.StatusCode != 201 {
		t.Fatalf("create without sign-in = %d %s", res.StatusCode, body)
	}
}

func TestSignInThroughTheProvider(t *testing.T) {
	idp := &provider{sub: "felipe", roles: []string{"admin"}}
	srv := newServer(t, idp)

	res, body := do(t, srv, call{method: "GET", path: "/api/auth/state"})
	if res.StatusCode != 200 || !strings.Contains(body, `"mode":"oidc"`) || !strings.Contains(body, `"provider":"Zitadel"`) || strings.Contains(body, `"user"`) {
		t.Fatalf("state = %d %s", res.StatusCode, body)
	}
	if res, _ := do(t, srv, call{method: "GET", path: "/api/connections"}); res.StatusCode != 401 || res.Header.Get("WWW-Authenticate") == "" {
		t.Fatalf("API without a session = %d", res.StatusCode)
	}

	// A callback without the flow cookie, or with another state, fails back
	// to the sign-in page.
	if res, _ := do(t, srv, call{method: "GET", path: "/auth/callback?code=good&state=x"}); res.StatusCode != http.StatusSeeOther || !strings.Contains(res.Header.Get("Location"), "signin_error=") {
		t.Fatalf("forged callback = %d %s", res.StatusCode, res.Header.Get("Location"))
	}

	res, _ = signIn(t, srv)
	if res.StatusCode != http.StatusSeeOther || res.Header.Get("Location") != "/" {
		t.Fatalf("callback = %d %s", res.StatusCode, res.Header.Get("Location"))
	}
	session := cookie(t, res, "mesa_session")
	if !session.HttpOnly || session.SameSite != http.SameSiteLaxMode {
		t.Errorf("session cookie = %+v", session)
	}
	admin := "mesa_session=" + session.Value

	_, body = do(t, srv, call{method: "GET", path: "/api/auth/state", cookie: admin})
	if !strings.Contains(body, `"email":"felipe@example.com"`) || !strings.Contains(body, `"role":"admin"`) {
		t.Fatalf("state when signed in = %s", body)
	}
	if res, _ := do(t, srv, call{method: "POST", path: "/api/connections", cookie: admin, origin: "https://evil.example", body: `{"name":"x","engine":"postgres","host":"h","user":"u"}`}); res.StatusCode != 403 {
		t.Fatalf("cross-site write with the cookie = %d", res.StatusCode)
	}
	if res, body := do(t, srv, call{method: "POST", path: "/api/connections", cookie: admin, origin: srv.URL, body: `{"name":"x","engine":"postgres","host":"h","user":"u"}`}); res.StatusCode != 201 {
		t.Fatalf("admin creates a connection = %d %s", res.StatusCode, body)
	}

	// A viewer reads but does not manage; nobody without a role gets in.
	idp.sub, idp.roles = "bo", []string{"viewer"}
	res, _ = signIn(t, srv)
	viewer := "mesa_session=" + cookie(t, res, "mesa_session").Value
	if res, _ := do(t, srv, call{method: "GET", path: "/api/connections", cookie: viewer}); res.StatusCode != 200 {
		t.Fatalf("viewer lists connections = %d", res.StatusCode)
	}
	if res, _ := do(t, srv, call{method: "POST", path: "/api/connections", cookie: viewer, body: `{"name":"y","engine":"postgres","host":"h","user":"u"}`}); res.StatusCode != 403 {
		t.Fatalf("viewer creates a connection = %d", res.StatusCode)
	}
	if res, _ := do(t, srv, call{method: "GET", path: "/api/people", cookie: viewer}); res.StatusCode != 403 {
		t.Fatalf("viewer lists people = %d", res.StatusCode)
	}
	idp.sub, idp.roles = "eve", []string{"billing"}
	if res, _ := signIn(t, srv); !strings.Contains(res.Header.Get("Location"), "signin_error=") {
		t.Fatalf("no role: %s", res.Header.Get("Location"))
	}
	if _, body := do(t, srv, call{method: "GET", path: "/api/people", cookie: admin}); strings.Count(body, `"email"`) != 2 {
		t.Fatalf("people = %s", body)
	}

	// An API token works as a bearer, on the API and on /mcp, and nowhere
	// once revoked.
	res, body = do(t, srv, call{method: "POST", path: "/api/me/tokens", cookie: admin, body: `{"name":"claude"}`})
	if res.StatusCode != 201 {
		t.Fatalf("create token = %d %s", res.StatusCode, body)
	}
	var created struct {
		Secret string `json:"secret"`
		Token  struct {
			ID string `json:"id"`
		} `json:"token"`
	}
	if err := json.Unmarshal([]byte(body), &created); err != nil || !strings.HasPrefix(created.Secret, "mesa_") {
		t.Fatalf("created token = %s", body)
	}
	if res, _ := do(t, srv, call{method: "GET", path: "/api/connections", bearer: created.Secret, origin: "https://anywhere.example"}); res.StatusCode != 200 {
		t.Fatalf("bearer token = %d", res.StatusCode)
	}
	mcpCall := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"list_connections","arguments":{}}}`
	if res := mcpPost(t, srv, mcpCall, ""); res.StatusCode != 401 {
		t.Fatalf("/mcp without a token = %d", res.StatusCode)
	}
	if res := mcpPost(t, srv, mcpCall, created.Secret); res.StatusCode != 200 {
		t.Fatalf("/mcp with a token = %d", res.StatusCode)
	}
	if res, _ := do(t, srv, call{method: "DELETE", path: "/api/me/tokens/" + created.Token.ID, cookie: admin}); res.StatusCode != 204 {
		t.Fatalf("revoke = %d", res.StatusCode)
	}
	if res := mcpPost(t, srv, mcpCall, created.Secret); res.StatusCode != 401 {
		t.Fatalf("/mcp with a revoked token = %d", res.StatusCode)
	}

	// Signing out ends the session and clears the cookie.
	res, _ = do(t, srv, call{method: "POST", path: "/api/auth/sign-out", cookie: viewer})
	if res.StatusCode != 204 || !strings.Contains(res.Header.Get("Set-Cookie"), "Max-Age=0") {
		t.Fatalf("sign-out = %d %s", res.StatusCode, res.Header.Get("Set-Cookie"))
	}
	if res, _ := do(t, srv, call{method: "GET", path: "/api/connections", cookie: viewer}); res.StatusCode != 401 {
		t.Fatalf("after sign-out = %d", res.StatusCode)
	}
}

func mcpPost(t *testing.T, srv *httptest.Server, body, bearer string) *http.Response {
	t.Helper()
	req, err := http.NewRequest("POST", srv.URL+"/mcp", strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	req.Header.Set("MCP-Protocol-Version", "2025-06-18")
	if bearer != "" {
		req.Header.Set("Authorization", "Bearer "+bearer)
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = io.Copy(io.Discard, res.Body)
	_ = res.Body.Close()
	return res
}
