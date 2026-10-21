// Package project reads and writes the optional .mesa/ folder (ADR 0005): a
// mesa.toml with the project's connections (never secrets, only env: or
// keychain: references), schema models, diagram layouts, migrations and saved
// queries as files. Overlay layers a project over Mesa's own store, so its
// connections and queries appear alongside the person's own.
package project

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/google/uuid"

	"github.com/felipemalacarne/mesa/internal/core/connection"
	"github.com/felipemalacarne/mesa/internal/core/errs"
)

const (
	// DirName is the project folder inside a repository or any directory.
	DirName = ".mesa"
	// ConfigName is the project file inside DirName.
	ConfigName = "mesa.toml"
)

// Conn is one [connections.<env>] table. Fields left out inherit from the
// project's default connection, so production can say only what differs.
type Conn struct {
	Driver      string `toml:"driver,omitempty"`
	Environment string `toml:"environment,omitempty"`
	Host        string `toml:"host,omitempty"`
	Port        int    `toml:"port,omitempty"`
	Database    string `toml:"database,omitempty"`
	User        string `toml:"user,omitempty"`
	Password    string `toml:"password,omitempty"`
	TLS         string `toml:"tls,omitempty"`
	ReadOnly    *bool  `toml:"read_only,omitempty"`
}

type file struct {
	Project struct {
		Name    string `toml:"name"`
		Default string `toml:"default,omitempty"`
	} `toml:"project"`
	Connections map[string]Conn `toml:"connections"`
}

// Project is a loaded .mesa/ folder.
type Project struct {
	// Root is the directory that holds .mesa; Dir is .mesa itself.
	Root    string
	Dir     string
	Name    string
	Default string
	conns   map[string]Conn // resolved, defaults applied
	byID    map[uuid.UUID]string
}

var namespace = uuid.MustParse("6c2b2f53-5f1e-4f35-9a4b-6d1b7e9a2c10")

// ConnectionID is stable for a project and environment, wherever the folder
// is checked out, so history, layouts and audit follow the project.
func ConnectionID(project, env string) uuid.UUID {
	return uuid.NewSHA1(namespace, []byte("project:"+project+":"+env))
}

// Find looks for .mesa/mesa.toml in start and its parents. It returns nil
// and no error when there is none: a project folder is never required.
func Find(start string) (*Project, error) {
	dir, err := filepath.Abs(start)
	if err != nil {
		return nil, err
	}
	for {
		cfg := filepath.Join(dir, DirName, ConfigName)
		if _, err := os.Stat(cfg); err == nil {
			return Load(filepath.Join(dir, DirName))
		} else if !errors.Is(err, fs.ErrNotExist) {
			return nil, err
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return nil, nil
		}
		dir = parent
	}
}

// Load reads a .mesa directory.
func Load(dir string) (*Project, error) {
	var f file
	if _, err := toml.DecodeFile(filepath.Join(dir, ConfigName), &f); err != nil {
		return nil, errs.Wrap(errs.Invalid, "cannot read "+filepath.Join(dir, ConfigName), err)
	}
	return build(dir, f)
}

func build(dir string, f file) (*Project, error) {
	p := &Project{Dir: dir, Root: filepath.Dir(dir), Name: strings.TrimSpace(f.Project.Name), Default: f.Project.Default, conns: map[string]Conn{}, byID: map[uuid.UUID]string{}}
	if p.Name == "" {
		p.Name = filepath.Base(p.Root)
	}
	if p.Default == "" && len(f.Connections) > 0 {
		p.Default = sortedEnvs(f.Connections)[0]
	}
	base, hasBase := f.Connections[p.Default]
	if p.Default != "" && !hasBase && len(f.Connections) > 0 {
		return nil, errs.Newf(errs.Invalid, "%s: default %q has no [connections.%s]", ConfigName, p.Default, p.Default)
	}
	for env, c := range f.Connections {
		if env != p.Default {
			c = inherit(c, base)
		}
		if strings.TrimSpace(c.Password) != "" && !isReference(c.Password) {
			return nil, errs.Newf(errs.Invalid, "%s: [connections.%s] password must be env:NAME or keychain:service/account, never the secret itself", ConfigName, env)
		}
		p.conns[env] = c
		p.byID[ConnectionID(p.Name, env)] = env
	}
	return p, nil
}

func isReference(s string) bool {
	return strings.HasPrefix(s, "env:") || strings.HasPrefix(s, "keychain:")
}

func inherit(c, base Conn) Conn {
	pick := func(v, d string) string {
		if v == "" {
			return d
		}
		return v
	}
	c.Driver = pick(c.Driver, base.Driver)
	c.Host = pick(c.Host, base.Host)
	c.Database = pick(c.Database, base.Database)
	c.User = pick(c.User, base.User)
	c.TLS = pick(c.TLS, base.TLS)
	if c.Port == 0 {
		c.Port = base.Port
	}
	// Environment, password and read_only are per environment on purpose.
	return c
}

var envOrder = map[string]int{"local": 0, "development": 1, "staging": 2, "production": 3}

func sortedEnvs(m map[string]Conn) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Slice(out, func(i, j int) bool {
		oi, iok := envOrder[out[i]]
		oj, jok := envOrder[out[j]]
		switch {
		case iok && jok:
			return oi < oj
		case iok != jok:
			return iok
		}
		return out[i] < out[j]
	})
	return out
}

// Envs lists the project's environments: local, development, staging,
// production, then any others by name.
func (p *Project) Envs() []string { return sortedEnvs(p.conns) }

// Connection returns the connection for env.
func (p *Project) Connection(env string) (*connection.Connection, error) {
	c, ok := p.conns[env]
	if !ok {
		return nil, errs.Newf(errs.NotFound, "project %s has no %q connection", p.Name, env)
	}
	envName := c.Environment
	if envName == "" {
		if _, known := envOrder[env]; known {
			envName = env
		}
	}
	driver := c.Driver
	if driver == "" {
		driver = "postgres"
	}
	stamp := p.modTime()
	conn := connection.Connection{ID: ConnectionID(p.Name, env), CreatedAt: stamp}
	if err := conn.Update(connection.Input{
		Name: p.Name, Engine: driver, Environment: envName, Host: c.Host, Port: c.Port,
		Database: c.Database, User: c.User, TLS: c.TLS, ReadOnly: c.ReadOnly,
	}, stamp); err != nil {
		return nil, errs.Wrap(errs.Invalid, fmt.Sprintf("%s: [connections.%s]", ConfigName, env), err)
	}
	return &conn, nil
}

// Connections returns every project connection that is valid.
func (p *Project) Connections() []connection.Connection {
	var out []connection.Connection
	for _, env := range p.Envs() {
		if c, err := p.Connection(env); err == nil {
			out = append(out, *c)
		}
	}
	return out
}

// EnvOf returns the environment key of a project connection ID.
func (p *Project) EnvOf(id uuid.UUID) (string, bool) {
	env, ok := p.byID[id]
	return env, ok
}

// Secret returns the secret reference of a project connection ("" when the
// connection needs none).
func (p *Project) Secret(id uuid.UUID) (string, bool) {
	env, ok := p.byID[id]
	if !ok {
		return "", false
	}
	return p.conns[env].Password, true
}

func (p *Project) modTime() time.Time {
	if fi, err := os.Stat(filepath.Join(p.Dir, ConfigName)); err == nil {
		return fi.ModTime().UTC()
	}
	return time.Time{}
}

// ModelPath is where the DBML model of a database lives.
func (p *Project) ModelPath(database string) string {
	return filepath.Join(p.Dir, "models", fileName(database)+".dbml")
}

// LayoutPath is where the diagram positions of a database live.
func (p *Project) LayoutPath(database string) string {
	return filepath.Join(p.Dir, "models", fileName(database)+".layout.json")
}

// MigrationsDir holds <timestamp>_<name>.up.sql and .down.sql files.
func (p *Project) MigrationsDir() string { return filepath.Join(p.Dir, "migrations") }

// QueriesDir holds saved queries as .sql files.
func (p *Project) QueriesDir() string { return filepath.Join(p.Dir, "queries") }

// Rel shows a path relative to the project root, for messages.
func (p *Project) Rel(path string) string {
	if r, err := filepath.Rel(p.Root, path); err == nil {
		return r
	}
	return path
}

func fileName(s string) string {
	return strings.Map(func(r rune) rune {
		if r == '/' || r == '\\' || r == ':' || r == 0 {
			return '_'
		}
		return r
	}, s)
}

// Init creates .mesa/ in root with the given connections. It refuses to
// overwrite an existing project.
func Init(root, name, defaultEnv string, conns map[string]Conn) (*Project, error) {
	dir := filepath.Join(root, DirName)
	if _, err := os.Stat(filepath.Join(dir, ConfigName)); err == nil {
		return nil, errs.Newf(errs.Conflict, "%s already exists", filepath.Join(dir, ConfigName))
	}
	for _, sub := range []string{"models", "migrations", "queries"} {
		if err := os.MkdirAll(filepath.Join(dir, sub), 0o755); err != nil {
			return nil, err
		}
	}
	var f file
	f.Project.Name = name
	f.Project.Default = defaultEnv
	f.Connections = conns
	var b strings.Builder
	b.WriteString("# Mesa project. Commit this folder; it never holds secrets.\n")
	b.WriteString("# Passwords are references: env:NAME reads the environment, keychain:service/account the OS keychain.\n")
	b.WriteString("# Connections inherit unset fields from the default one.\n\n")
	enc := toml.NewEncoder(&b)
	enc.Indent = ""
	if err := enc.Encode(f); err != nil {
		return nil, err
	}
	if err := os.WriteFile(filepath.Join(dir, ConfigName), []byte(b.String()), 0o644); err != nil {
		return nil, err
	}
	for _, sub := range []string{"migrations", "queries"} {
		_ = os.WriteFile(filepath.Join(dir, sub, ".gitkeep"), nil, 0o644)
	}
	return Load(dir)
}
