// Package app wires every use case over the ports. Transports hold an *App
// and call its services; nothing here knows HTTP, the terminal or MCP.
package app

import (
	"time"

	"github.com/felipemalacarne/mesa/internal/app/changes"
	"github.com/felipemalacarne/mesa/internal/app/connections"
	"github.com/felipemalacarne/mesa/internal/app/designer"
	"github.com/felipemalacarne/mesa/internal/app/explore"
	"github.com/felipemalacarne/mesa/internal/app/monitor"
	"github.com/felipemalacarne/mesa/internal/app/query"
	"github.com/felipemalacarne/mesa/internal/app/roles"
	"github.com/felipemalacarne/mesa/internal/engine"
	"github.com/felipemalacarne/mesa/internal/secret"
	"github.com/felipemalacarne/mesa/internal/session"
	"github.com/felipemalacarne/mesa/internal/store"
)

// Mode names the surface Mesa runs as.
type Mode string

const (
	ModeServer  Mode = "server"
	ModeDesktop Mode = "desktop"
	ModeCLI     Mode = "cli"
)

// App holds every service.
type App struct {
	Version  string
	Mode     Mode
	Engines  []engine.Engine
	Sessions *session.Manager
	Store    store.Store

	Connections *connections.Service
	Explore     *explore.Service
	Changes     *changes.Service
	Query       *query.Service
	Monitor     *monitor.Service
	Roles       *roles.Service
	Designer    *designer.Service
}

// Options configure New.
type Options struct {
	Version string
	Mode    Mode
	Engines []engine.Engine
	Store   store.Store
	Vault   secret.Vault
	Session session.Options
}

// New builds the application.
func New(o Options) *App {
	sessions := session.New(o.Engines, o.Store.Connections(), o.Vault, o.Session)
	unlocks := changes.NewMemoryUnlocks()
	exp := explore.New(sessions, 30*time.Second)
	ch := changes.New(sessions, o.Store.Audit(), unlocks, exp)
	a := &App{
		Version:  o.Version,
		Mode:     o.Mode,
		Engines:  o.Engines,
		Sessions: sessions,
		Store:    o.Store,

		Connections: connections.New(o.Store.Connections(), o.Vault, sessions, unlocks),
		Explore:     exp,
		Changes:     ch,
		Query:       query.New(sessions, o.Store.History(), o.Store.Queries(), unlocks),
		Monitor:     monitor.New(sessions, unlocks),
		Roles:       roles.New(sessions, ch),
	}
	a.Designer = designer.New(sessions, ch, o.Store.Layouts(), exp.Schema)
	return a
}

// Close releases sessions and the store.
func (a *App) Close() error {
	a.Sessions.Close()
	return a.Store.Close()
}
