package http

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/felipemalacarne/mesa/internal/app"
	"github.com/felipemalacarne/mesa/internal/app/accounts"
	"github.com/felipemalacarne/mesa/internal/core/account"
	"github.com/felipemalacarne/mesa/internal/core/errs"
)

const (
	sessionCookie = "mesa_session"
	flowCookie    = "mesa_signin"
	// loginPath sends the browser to the identity provider; callbackPath is
	// where it comes back (the redirect URI to register with the provider).
	loginPath    = "/auth/login"
	callbackPath = "/auth/callback"
)

// requestInfo is what a handler needs to know about the request it serves
// beyond its body: where the server is reached, and the credential used.
type requestInfo struct {
	secure     bool
	baseURL    string
	credential string
}

type requestInfoKey struct{}

func info(ctx context.Context) requestInfo {
	i, _ := ctx.Value(requestInfoKey{}).(requestInfo)
	return i
}

// describe reads how the request reached Mesa: the public URL when one is
// configured, else the request (and the proxy headers in front of it).
func describe(r *http.Request, publicURL string) requestInfo {
	if publicURL != "" {
		return requestInfo{secure: strings.HasPrefix(publicURL, "https://"), baseURL: publicURL}
	}
	scheme := "http"
	if r.TLS != nil || strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https") {
		scheme = "https"
	}
	host := r.Host
	if h := r.Header.Get("X-Forwarded-Host"); h != "" {
		host = h
	}
	return requestInfo{secure: scheme == "https", baseURL: scheme + "://" + host}
}

// publicPaths answer without a signed-in user: the sign-in page asks them.
var publicPaths = map[string]bool{
	"/api/auth/state":    true,
	"/api/auth/sign-out": true,
}

// authenticate puts the principal of each request in its context. Without
// an identity provider (desktop, or a server without one) everyone is the
// local principal; with one, /api and /mcp need a session cookie or an API
// token, except what the sign-in page asks.
func authenticate(a *app.App, o Options) func(http.Handler) http.Handler {
	trusted := map[string]bool{}
	for _, origin := range o.CORSOrigins {
		trusted[origin] = true
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			in := describe(r, o.PublicURL)
			ctx := r.Context()
			if a.Accounts == nil {
				ctx = context.WithValue(account.With(ctx, account.Local()), requestInfoKey{}, in)
				next.ServeHTTP(w, r.WithContext(ctx))
				return
			}
			cred, bearer := credential(r)
			// A browser sends the cookie on its own: anything that changes
			// state must come from Mesa's own pages. A bearer token is never
			// sent on its own, so it needs no such check.
			if !bearer && crossSite(r, trusted) {
				writeError(w, errs.New(errs.Forbidden, "Mesa refuses changes sent from another site"))
				return
			}
			in.credential = cred
			ctx = context.WithValue(ctx, requestInfoKey{}, in)
			if cred != "" {
				u, _, err := a.Accounts.Authenticate(ctx, cred)
				switch {
				case err == nil:
					ctx = account.With(ctx, account.Of(u))
				case publicPaths[r.URL.Path]:
					// A stale cookie on the sign-in page is no error.
				default:
					unauthorized(w, err, bearer)
					return
				}
			} else if !publicPaths[r.URL.Path] {
				unauthorized(w, errs.New(errs.Unauthenticated, "sign in to Mesa first"), bearer)
				return
			}
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// credential returns the bearer token, else the session cookie, and whether
// it was a bearer token.
func credential(r *http.Request) (string, bool) {
	if h := r.Header.Get("Authorization"); len(h) > 7 && strings.EqualFold(h[:7], "bearer ") {
		return strings.TrimSpace(h[7:]), true
	}
	if c, err := r.Cookie(sessionCookie); err == nil {
		return c.Value, false
	}
	return "", false
}

func unauthorized(w http.ResponseWriter, err error, bearer bool) {
	if bearer || errs.KindOf(err) == errs.Unauthenticated {
		w.Header().Set("WWW-Authenticate", `Bearer realm="mesa"`)
	}
	writeError(w, err)
}

// crossSite reports whether a state-changing request comes from a page that
// is not Mesa's. Browsers say so with Sec-Fetch-Site and Origin; a client
// that sends neither is not a browser.
func crossSite(r *http.Request, trusted map[string]bool) bool {
	switch r.Method {
	case http.MethodGet, http.MethodHead, http.MethodOptions:
		return false
	}
	origin := r.Header.Get("Origin")
	if origin != "" && trusted[origin] {
		return false
	}
	if site := r.Header.Get("Sec-Fetch-Site"); site != "" && site != "same-origin" && site != "none" {
		return true
	}
	if origin == "" {
		return false
	}
	u, err := url.Parse(origin)
	if err != nil {
		return true
	}
	return u.Host != r.Host && u.Host != r.Header.Get("X-Forwarded-Host")
}

func sessionCookieHeader(in requestInfo, value string, expires time.Time) string {
	c := http.Cookie{
		Name: sessionCookie, Value: value, Path: "/",
		HttpOnly: true, Secure: in.secure, SameSite: http.SameSiteLaxMode,
	}
	if value == "" {
		c.MaxAge = -1
	} else {
		c.Expires = expires
	}
	return c.String()
}

// --- signing in through the identity provider ---------------------------------

// signInRoutes are plain redirects, not JSON: the browser goes to the
// provider and comes back. Errors land on the sign-in page as ?signin_error=.
func signInRoutes(a *app.App, o Options) (login, callback http.HandlerFunc) {
	fail := func(w http.ResponseWriter, r *http.Request, err error) {
		http.Redirect(w, r, "/?signin_error="+url.QueryEscape(errs.Message(err)), http.StatusSeeOther)
	}
	login = func(w http.ResponseWriter, r *http.Request) {
		if a.Accounts == nil {
			http.Redirect(w, r, "/", http.StatusSeeOther)
			return
		}
		in := describe(r, o.PublicURL)
		to, flow, err := a.Accounts.Begin(r.Context(), in.baseURL+callbackPath)
		if err != nil {
			fail(w, r, err)
			return
		}
		b, _ := json.Marshal(flow)
		http.SetCookie(w, &http.Cookie{
			Name: flowCookie, Value: base64.RawURLEncoding.EncodeToString(b), Path: "/auth",
			MaxAge: 600, HttpOnly: true, Secure: in.secure, SameSite: http.SameSiteLaxMode,
		})
		http.Redirect(w, r, to, http.StatusFound)
	}
	callback = func(w http.ResponseWriter, r *http.Request) {
		if a.Accounts == nil {
			http.Redirect(w, r, "/", http.StatusSeeOther)
			return
		}
		in := describe(r, o.PublicURL)
		http.SetCookie(w, &http.Cookie{Name: flowCookie, Path: "/auth", MaxAge: -1, HttpOnly: true, Secure: in.secure, SameSite: http.SameSiteLaxMode})
		q := r.URL.Query()
		if e := q.Get("error"); e != "" {
			msg := q.Get("error_description")
			if msg == "" {
				msg = e
			}
			fail(w, r, errs.New(errs.Unauthenticated, "the identity provider said: "+msg))
			return
		}
		var flow *accounts.Flow
		if c, err := r.Cookie(flowCookie); err == nil {
			if b, err := base64.RawURLEncoding.DecodeString(c.Value); err == nil {
				_ = json.Unmarshal(b, &flow)
			}
		}
		signed, err := a.Accounts.Finish(r.Context(), in.baseURL+callbackPath, q.Get("code"), q.Get("state"), flow)
		if err != nil {
			fail(w, r, err)
			return
		}
		w.Header().Add("Set-Cookie", sessionCookieHeader(in, signed.Session, signed.Expires))
		http.Redirect(w, r, "/", http.StatusSeeOther)
	}
	return login, callback
}
