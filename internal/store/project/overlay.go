package project

import (
	"context"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/felipemalacarne/mesa/internal/core/connection"
	"github.com/felipemalacarne/mesa/internal/core/errs"
	"github.com/felipemalacarne/mesa/internal/secret"
	"github.com/felipemalacarne/mesa/internal/store"
)

// Overlay returns base with the project layered on top: the project's
// connections and saved queries are listed with the person's own, layouts of
// project connections are files next to the models. Audit, history and
// everything else stay in base.
func Overlay(base store.Store, p *Project) store.Store {
	if p == nil {
		return base
	}
	return &overlay{Store: base, p: p}
}

type overlay struct {
	store.Store
	p *Project
}

func (o *overlay) Connections() store.Connections { return &conns{base: o.Store.Connections(), p: o.p} }
func (o *overlay) Layouts() store.Layouts         { return &layouts{base: o.Store.Layouts(), p: o.p} }
func (o *overlay) Queries() store.SavedQueries    { return &queries{base: o.Store.Queries(), p: o.p} }

func readOnlyDefinition(p *Project) error {
	return errs.Newf(errs.Forbidden, "this connection is defined in %s; edit that file instead", p.Rel(filepath.Join(p.Dir, ConfigName)))
}

type conns struct {
	base store.Connections
	p    *Project
}

func (c *conns) List(ctx context.Context) ([]connection.Connection, error) {
	own, err := c.base.List(ctx)
	if err != nil {
		return nil, err
	}
	return append(own, c.p.Connections()...), nil
}

func (c *conns) Get(ctx context.Context, id uuid.UUID) (*connection.Connection, error) {
	if env, ok := c.p.EnvOf(id); ok {
		return c.p.Connection(env)
	}
	return c.base.Get(ctx, id)
}

func (c *conns) Put(ctx context.Context, conn connection.Connection) error {
	if _, ok := c.p.EnvOf(conn.ID); ok {
		return readOnlyDefinition(c.p)
	}
	return c.base.Put(ctx, conn)
}

func (c *conns) Delete(ctx context.Context, id uuid.UUID) error {
	if _, ok := c.p.EnvOf(id); ok {
		return readOnlyDefinition(c.p)
	}
	return c.base.Delete(ctx, id)
}

func (c *conns) TouchOpened(ctx context.Context, id uuid.UUID, at time.Time) error {
	return c.base.TouchOpened(ctx, id, at)
}

func (c *conns) LastOpened(ctx context.Context) (map[uuid.UUID]time.Time, error) {
	return c.base.LastOpened(ctx)
}

type layouts struct {
	base store.Layouts
	p    *Project
}

func (l *layouts) Get(ctx context.Context, id uuid.UUID, database string) (map[string]store.Position, error) {
	if _, ok := l.p.EnvOf(id); !ok {
		return l.base.Get(ctx, id, database)
	}
	b, err := os.ReadFile(l.p.LayoutPath(database))
	if errors.Is(err, fs.ErrNotExist) {
		return nil, errs.New(errs.NotFound, "no layout yet")
	}
	if err != nil {
		return nil, errs.Wrap(errs.Unavailable, "cannot read the layout", err)
	}
	var out map[string]store.Position
	if err := json.Unmarshal(b, &out); err != nil {
		return nil, errs.Wrap(errs.Invalid, "cannot parse "+l.p.Rel(l.p.LayoutPath(database)), err)
	}
	return out, nil
}

func (l *layouts) Put(ctx context.Context, id uuid.UUID, database string, positions map[string]store.Position) error {
	if _, ok := l.p.EnvOf(id); !ok {
		return l.base.Put(ctx, id, database, positions)
	}
	b, err := json.MarshalIndent(positions, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(l.p.LayoutPath(database)), 0o755); err != nil {
		return err
	}
	return os.WriteFile(l.p.LayoutPath(database), append(b, '\n'), 0o644)
}

type queries struct {
	base store.SavedQueries
	p    *Project
}

func queryID(project, file string) uuid.UUID {
	return uuid.NewSHA1(namespace, []byte("query:"+project+":"+file))
}

// files lists the project's saved queries by their IDs.
func (q *queries) files() (map[uuid.UUID]string, error) {
	entries, err := os.ReadDir(q.p.QueriesDir())
	if errors.Is(err, fs.ErrNotExist) {
		return map[uuid.UUID]string{}, nil
	}
	if err != nil {
		return nil, err
	}
	out := map[uuid.UUID]string{}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".sql") {
			continue
		}
		out[queryID(q.p.Name, e.Name())] = e.Name()
	}
	return out, nil
}

func (q *queries) List(ctx context.Context) ([]store.SavedQuery, error) {
	own, err := q.base.List(ctx)
	if err != nil {
		return nil, err
	}
	files, err := q.files()
	if err != nil {
		return nil, errs.Wrap(errs.Unavailable, "cannot list project queries", err)
	}
	var mine []store.SavedQuery
	for id, name := range files {
		path := filepath.Join(q.p.QueriesDir(), name)
		b, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		fi, _ := os.Stat(path)
		sq := store.SavedQuery{ID: id, Name: strings.TrimSuffix(name, ".sql"), SQL: string(b)}
		if fi != nil {
			sq.UpdatedAt = fi.ModTime().UTC()
		}
		mine = append(mine, sq)
	}
	sort.Slice(mine, func(i, j int) bool { return mine[i].Name < mine[j].Name })
	return append(own, mine...), nil
}

func (q *queries) Put(ctx context.Context, sq store.SavedQuery) error {
	files, err := q.files()
	if err != nil {
		return err
	}
	if name, ok := files[sq.ID]; ok {
		return os.WriteFile(filepath.Join(q.p.QueriesDir(), name), []byte(sq.SQL), 0o644)
	}
	return q.base.Put(ctx, sq)
}

func (q *queries) Delete(ctx context.Context, id uuid.UUID) error {
	files, err := q.files()
	if err != nil {
		return err
	}
	if name, ok := files[id]; ok {
		return os.Remove(filepath.Join(q.p.QueriesDir(), name))
	}
	return q.base.Delete(ctx, id)
}

// Vault returns base with project connections resolved from their
// references in mesa.toml. Their secrets are never stored.
func Vault(base secret.Vault, p *Project) secret.Vault {
	if p == nil {
		return base
	}
	return &vault{Vault: base, p: p}
}

type vault struct {
	secret.Vault
	p *Project
}

func (v *vault) Get(ctx context.Context, id uuid.UUID) (string, error) {
	if ref, ok := v.p.Secret(id); ok {
		return secret.Resolve(ref)
	}
	return v.Vault.Get(ctx, id)
}

func (v *vault) Set(ctx context.Context, id uuid.UUID, s string) error {
	if _, ok := v.p.EnvOf(id); ok {
		return readOnlyDefinition(v.p)
	}
	return v.Vault.Set(ctx, id, s)
}

func (v *vault) Delete(ctx context.Context, id uuid.UUID) error {
	if _, ok := v.p.EnvOf(id); ok {
		return nil
	}
	return v.Vault.Delete(ctx, id)
}
