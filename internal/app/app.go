// Package app wires every use case over the ports. Transports hold an *App
// and call its services; nothing here knows HTTP, the terminal or MCP.
package app

import (
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/felipemalacarne/mesa/internal/app/accounts"
	"github.com/felipemalacarne/mesa/internal/app/changes"
	"github.com/felipemalacarne/mesa/internal/app/connections"
	"github.com/felipemalacarne/mesa/internal/app/designer"
	"github.com/felipemalacarne/mesa/internal/app/explore"
	"github.com/felipemalacarne/mesa/internal/app/monitor"
	"github.com/felipemalacarne/mesa/internal/app/query"
	"github.com/felipemalacarne/mesa/internal/app/roles"
	"github.com/felipemalacarne/mesa/internal/core/connection"
	"github.com/felipemalacarne/mesa/internal/engine"
	"github.com/felipemalacarne/mesa/internal/secret"
	"github.com/felipemalacarne/mesa/internal/session"
	"github.com/felipemalacarne/mesa/internal/store"
	"github.com/felipemalacarne/mesa/internal/store/declared"
	"github.com/felipemalacarne/mesa/internal/store/project"
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
	// Project is the .mesa/ folder in use, or nil: it is always optional.
	Project *project.Project

	// Accounts is sign-in through the identity provider; nil when every
	// request runs as the local principal (desktop, CLI, MCP on stdio, or a
	// server without a provider).
	Accounts *accounts.Service

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
	// Project, when set, layers a .mesa/ folder over Store and Vault.
	Project *project.Project
	// Declared, when set, adds the connections a deployment declares in a
	// file (MESA_CONNECTIONS).
	Declared *declared.Set
	// Accounts, when set, turns on sign-in through an identity provider
	// (server mode).
	Accounts *accounts.Options
}

// New builds the application.
func New(o Options) *App {
	o.Store = declared.Overlay(project.Overlay(o.Store, o.Project), o.Declared)
	o.Vault = declared.Vault(project.Vault(o.Vault, o.Project), o.Declared)
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
		Project:  o.Project,

		Connections: connections.New(o.Store.Connections(), o.Vault, sessions, unlocks),
		Explore:     exp,
		Changes:     ch,
		Query:       query.New(sessions, o.Store.History(), o.Store.Queries(), unlocks),
		Monitor:     monitor.New(sessions, unlocks, o.Store.Metrics()),
		Roles:       roles.New(sessions, ch),
	}
	a.Designer = designer.New(sessions, ch, o.Store.Layouts(), exp.Schema)
	if o.Accounts != nil {
		a.Accounts = accounts.New(o.Store, *o.Accounts)
	}
	if o.Project != nil || o.Declared != nil {
		p, d := o.Project, o.Declared
		a.Connections.SetDefinedIn(func(id uuid.UUID) string {
			if d != nil && d.Has(id) {
				return d.Path
			}
			if p != nil {
				if _, ok := p.EnvOf(id); ok {
					return p.Rel(filepath.Join(p.Dir, project.ConfigName))
				}
			}
			return ""
		})
	}
	return a
}

// Close releases sessions and the store.
func (a *App) Close() error {
	a.Sessions.Close()
	return a.Store.Close()
}

// InitProject creates a .mesa/ folder in root. With from, the project gets one
// connection with its settings under its environment; the password becomes an
// env: reference, never the secret.
func InitProject(root, name string, from *connection.Connection) (*project.Project, error) {
	if name == "" {
		name = filepath.Base(root)
	}
	conns := map[string]project.Conn{}
	def := ""
	if from != nil {
		def = string(from.Environment)
		ro := from.ReadOnly
		conns[def] = project.Conn{
			Driver: string(from.Engine), Host: from.Host, Port: from.Port, Database: from.Database,
			User: from.User, TLS: string(from.TLS), ReadOnly: &ro,
			Password: "env:" + envName(name) + "_" + envName(def) + "_PASSWORD",
		}
	}
	return project.Init(root, name, def, conns)
}

func envName(s string) string {
	out := make([]rune, 0, len(s))
	for _, r := range strings.ToUpper(s) {
		if (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			out = append(out, r)
		} else {
			out = append(out, '_')
		}
	}
	return string(out)
}
