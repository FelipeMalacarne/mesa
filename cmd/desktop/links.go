//go:build desktop

package main

import (
	"context"
	"encoding/json"
	"sync"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// linkRouter navigates the window to mesa:// links. A link that arrives
// before the page is ready waits for it.
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

func (l *linkRouter) navigate(ctx context.Context, route string) {
	js, _ := json.Marshal(route)
	runtime.WindowExecJS(ctx, "window.location.assign("+string(js)+")")
}
