// Package declared reads the connections an operator declares for Mesa in a
// file (MESA_CONNECTIONS), so a deployment keeps them in Git with the rest of
// its configuration: a Kubernetes ConfigMap, a file mounted into a container.
// Like a project's, these connections are read-only through Mesa and their
// passwords are references (env:NAME), never the secret itself.
package declared

import (
	"context"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/google/uuid"

	"github.com/felipemalacarne/mesa/internal/core/connection"
	"github.com/felipemalacarne/mesa/internal/core/errs"
	"github.com/felipemalacarne/mesa/internal/secret"
	"github.com/felipemalacarne/mesa/internal/store"
)

// Conn is one [connections.<name>] table; the name is the connection's name.
type Conn struct {
	Engine      string `toml:"engine"`
	Environment string `toml:"environment"`
	Host        string `toml:"host"`
	Port        int    `toml:"port"`
	Database    string `toml:"database"`
	User        string `toml:"user"`
	Password    string `toml:"password"`
	TLS         string `toml:"tls"`
	ReadOnly    *bool  `toml:"read_only"`
}

type file struct {
	Connections map[string]Conn `toml:"connections"`
}

// Set is a loaded connections file.
type Set struct {
	// Path is the file the connections come from.
	Path  string
	conns []connection.Connection
	refs  map[uuid.UUID]string
}

var namespace = uuid.MustParse("0f6a3c1e-8d42-4b7a-9e15-3c2d7f4a9b61")

// ConnectionID is stable for a name, so history, layouts and audit follow a
// declared connection across restarts and replicas.
func ConnectionID(name string) uuid.UUID {
	return uuid.NewSHA1(namespace, []byte("declared:"+name))
}

// Load reads path. Unlike a project folder, a declared connection that does
// not validate stops Mesa from starting: the file is the deployment's.
func Load(path string) (*Set, error) {
	var f file
	md, err := toml.DecodeFile(path, &f)
	if err != nil {
		return nil, errs.Wrap(errs.Invalid, "cannot read "+path, err)
	}
	if extra := md.Undecoded(); len(extra) > 0 {
		return nil, errs.Newf(errs.Invalid, "%s: unknown key %s", path, extra[0])
	}
	stamp := time.Now().UTC()
	if fi, err := os.Stat(path); err == nil {
		stamp = fi.ModTime().UTC()
	}
	return build(path, f, stamp)
}

func build(path string, f file, now time.Time) (*Set, error) {
	s := &Set{Path: path, refs: map[uuid.UUID]string{}}
	names := make([]string, 0, len(f.Connections))
	for name := range f.Connections {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		c := f.Connections[name]
		if strings.TrimSpace(c.Password) != "" && !secret.IsReference(c.Password) {
			return nil, errs.Newf(errs.Invalid, "%s: [connections.%s] password must be env:NAME or keychain:service/account, never the secret itself", path, name)
		}
		engine := c.Engine
		if engine == "" {
			engine = string(connection.Postgres)
		}
		conn := connection.Connection{ID: ConnectionID(name), CreatedAt: now}
		if err := conn.Update(connection.Input{
			Name: name, Engine: engine, Environment: c.Environment, Host: c.Host, Port: c.Port,
			Database: c.Database, User: c.User, TLS: c.TLS, ReadOnly: c.ReadOnly,
		}, now); err != nil {
			return nil, errs.Wrap(errs.Invalid, path+": [connections."+name+"]", err)
		}
		s.conns = append(s.conns, conn)
		s.refs[conn.ID] = c.Password
	}
	return s, nil
}

// Connections returns the declared connections by name.
func (s *Set) Connections() []connection.Connection {
	return append([]connection.Connection(nil), s.conns...)
}

// Has reports whether id is a declared connection.
func (s *Set) Has(id uuid.UUID) bool {
	_, ok := s.refs[id]
	return ok
}

// Secret returns the password reference of a declared connection ("" when
// it needs none).
func (s *Set) Secret(id uuid.UUID) (string, bool) {
	ref, ok := s.refs[id]
	return ref, ok
}

func (s *Set) readOnly() error {
	return errs.Newf(errs.Forbidden, "this connection is declared in %s; change it there", s.Path)
}

// Overlay returns base with the declared connections listed next to the
// stored ones. Everything else (audit, history, layouts) stays in base.
func Overlay(base store.Store, s *Set) store.Store {
	if s == nil {
		return base
	}
	return &overlay{Store: base, s: s}
}

type overlay struct {
	store.Store
	s *Set
}

func (o *overlay) Connections() store.Connections { return &conns{base: o.Store.Connections(), s: o.s} }

type conns struct {
	base store.Connections
	s    *Set
}

func (c *conns) List(ctx context.Context) ([]connection.Connection, error) {
	own, err := c.base.List(ctx)
	if err != nil {
		return nil, err
	}
	return append(own, c.s.Connections()...), nil
}

func (c *conns) Get(ctx context.Context, id uuid.UUID) (*connection.Connection, error) {
	for _, conn := range c.s.conns {
		if conn.ID == id {
			conn := conn
			return &conn, nil
		}
	}
	return c.base.Get(ctx, id)
}

func (c *conns) Put(ctx context.Context, conn connection.Connection) error {
	if c.s.Has(conn.ID) {
		return c.s.readOnly()
	}
	return c.base.Put(ctx, conn)
}

func (c *conns) Delete(ctx context.Context, id uuid.UUID) error {
	if c.s.Has(id) {
		return c.s.readOnly()
	}
	return c.base.Delete(ctx, id)
}

func (c *conns) TouchOpened(ctx context.Context, id uuid.UUID, at time.Time) error {
	return c.base.TouchOpened(ctx, id, at)
}

func (c *conns) LastOpened(ctx context.Context) (map[uuid.UUID]time.Time, error) {
	return c.base.LastOpened(ctx)
}

// Vault returns base with declared connections resolved from their
// references. Their secrets are never stored.
func Vault(base secret.Vault, s *Set) secret.Vault {
	if s == nil {
		return base
	}
	return &vault{Vault: base, s: s}
}

type vault struct {
	secret.Vault
	s *Set
}

func (v *vault) Get(ctx context.Context, id uuid.UUID) (string, error) {
	if ref, ok := v.s.Secret(id); ok {
		return secret.Resolve(ref)
	}
	return v.Vault.Get(ctx, id)
}

func (v *vault) Set(ctx context.Context, id uuid.UUID, s string) error {
	if v.s.Has(id) {
		return v.s.readOnly()
	}
	return v.Vault.Set(ctx, id, s)
}

func (v *vault) Delete(ctx context.Context, id uuid.UUID) error {
	if v.s.Has(id) {
		return nil
	}
	return v.Vault.Delete(ctx, id)
}
