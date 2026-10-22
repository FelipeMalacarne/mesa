//go:build desktop

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// linkRouter navigates the window to mesa:// links. A link given at launch
// is written into the first page (serve), so the web app starts on it; a
// link that arrives later moves the running app without reloading it.
type linkRouter struct {
	mu      sync.Mutex
	ctx     context.Context
	isReady bool
	pending string
}

func (l *linkRouter) start(ctx context.Context) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.ctx = ctx
}

// ready is OnDomReady: it opens a link that was waiting.
func (l *linkRouter) ready(ctx context.Context) {
	l.mu.Lock()
	l.ctx, l.isReady = ctx, true
	route := l.pending
	l.pending = ""
	l.mu.Unlock()
	if route != "" {
		l.navigate(ctx, route)
	}
}

// open shows the window and navigates to route, a path desktop.Route
// produced (so it is a same-origin path with no script in it).
func (l *linkRouter) open(route string) {
	l.mu.Lock()
	ctx, ready := l.ctx, l.isReady
	if !ready {
		l.pending = route
	}
	l.mu.Unlock()
	if ctx == nil {
		return
	}
	l.focus()
	if ready {
		l.navigate(ctx, route)
	}
}

func (l *linkRouter) focus() {
	l.mu.Lock()
	ctx := l.ctx
	l.mu.Unlock()
	if ctx == nil {
		return
	}
	runtime.WindowUnminimise(ctx)
	runtime.WindowShow(ctx)
}

// navigate moves the web app to route in place: its router follows the
// history change on popstate, so open tabs and staged edits stay.
func (l *linkRouter) navigate(ctx context.Context, route string) {
	js, _ := json.Marshal(route)
	runtime.WindowExecJS(ctx, "history.pushState(null,'',"+string(js)+");dispatchEvent(new PopStateEvent('popstate'))")
}

// serve wraps the app's handler: the first page load, while a launch link
// is pending, gets a script that sets the address before the app starts.
func (l *linkRouter) serve(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/" {
			next.ServeHTTP(w, r)
			return
		}
		l.mu.Lock()
		route := l.pending
		l.pending = ""
		l.mu.Unlock()
		if route == "" {
			next.ServeHTTP(w, r)
			return
		}
		rec := httptest.NewRecorder()
		next.ServeHTTP(rec, r)
		body := rec.Body.Bytes()
		js, _ := json.Marshal(route)
		script := []byte("<head><script>history.replaceState(null,''," + string(js) + ")</script>")
		body = bytes.Replace(body, []byte("<head>"), script, 1)
		for k, v := range rec.Header() {
			w.Header()[k] = v
		}
		w.Header().Set("Content-Length", strconv.Itoa(len(body)))
		w.WriteHeader(rec.Code)
		_, _ = w.Write(body)
	})
}
