// Package desktop holds what the desktop shell needs beyond the shared
// application: mesa:// deep links and registering Mesa as their handler.
package desktop

import (
	"net/url"
	"path"
	"regexp"
	"strings"
)

// Scheme is the URL scheme the desktop app handles.
const Scheme = "mesa"

var (
	safePath  = regexp.MustCompile(`^/[A-Za-z0-9_.~%/-]*$`)
	safeQuery = regexp.MustCompile(`^[A-Za-z0-9_.~%=&-]*$`)
)

// Route turns a deep link into the web app route it opens:
//
//	mesa://c/<connection>/db/<database>/t/<schema>/<table>  → /c/<connection>/db/<database>/t/<schema>/<table>
//	mesa://c/<connection>/monitor?database=shop             → /c/<connection>/monitor?database=shop
//	mesa://                                                 → /
//
// Anything that is not a plain path with a simple query is refused, so a
// link can only navigate inside the app.
func Route(raw string) (string, bool) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || !strings.EqualFold(u.Scheme, Scheme) || u.User != nil || u.Fragment != "" {
		return "", false
	}
	p := "/" + u.Host + u.EscapedPath()
	if u.Opaque != "" {
		p = "/" + strings.TrimPrefix(u.Opaque, "//")
	}
	if strings.Contains(p, "..") {
		return "", false
	}
	p = path.Clean(p)
	if !safePath.MatchString(p) {
		return "", false
	}
	if u.RawQuery != "" {
		if !safeQuery.MatchString(u.RawQuery) {
			return "", false
		}
		p += "?" + u.RawQuery
	}
	return p, true
}

// FromArgs finds the first deep link among command-line arguments, which is
// how Linux and Windows hand a link to the app.
func FromArgs(args []string) (string, bool) {
	for _, a := range args {
		if strings.HasPrefix(strings.ToLower(a), Scheme+":") {
			if r, ok := Route(a); ok {
				return r, true
			}
		}
	}
	return "", false
}
