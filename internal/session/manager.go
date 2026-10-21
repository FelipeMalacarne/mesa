// Package session turns a connection plus its secret into a pooled engine
// session, caches sessions per (connection, database) and evicts idle ones.
// Callers get a ready Session; the secret never leaves this package.
package session

import (
	"context"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/felipemalacarne/mesa/internal/core/connection"
	"github.com/felipemalacarne/mesa/internal/core/errs"
	"github.com/felipemalacarne/mesa/internal/engine"
	"github.com/felipemalacarne/mesa/internal/secret"
	"github.com/felipemalacarne/mesa/internal/store"
)

// Options tune the manager.
type Options struct {
	IdleTimeout    time.Duration // evict sessions unused for this long (default 10m)
	ConnectTimeout time.Duration // default 5s
	MaxConns       int           // per session pool (default 4)
	AppName        string        // reported to the server (default "mesa")
}

// Manager owns every open session.
type Manager struct {
	engines map[connection.Engine]engine.Engine
	conns   store.Connections
	vault   secret.Vault
	opts    Options

	mu       sync.Mutex
	sessions map[key]*entry
	stop     chan struct{}
	now      func() time.Time
}

type key struct {
	conn uuid.UUID
	db   string
}

type entry struct {
	session  engine.Session
	lastUsed time.Time
}

// New builds a manager over the given engines, connection store and vault.
func New(engines []engine.Engine, conns store.Connections, vault secret.Vault, opts Options) *Manager {
	if opts.IdleTimeout == 0 {
		opts.IdleTimeout = 10 * time.Minute
	}
	if opts.ConnectTimeout == 0 {
		opts.ConnectTimeout = 5 * time.Second
	}
	if opts.MaxConns == 0 {
		opts.MaxConns = 4
	}
	if opts.AppName == "" {
		opts.AppName = "mesa"
	}
	m := &Manager{
		engines:  map[connection.Engine]engine.Engine{},
		conns:    conns,
		vault:    vault,
		opts:     opts,
		sessions: map[key]*entry{},
		stop:     make(chan struct{}),
		now:      time.Now,
	}
	for _, e := range engines {
		m.engines[e.Name()] = e
	}
	go m.evictLoop()
	return m
}

// Engine returns the driver for an engine name.
func (m *Manager) Engine(name connection.Engine) (engine.Engine, error) {
	e, ok := m.engines[name]
	if !ok {
		return nil, errs.Newf(errs.Unsupported, "engine %q is not available in this build", name)
	}
	return e, nil
}

// Session returns a pooled session for the connection and database. An empty
// database means the connection's default database.
func (m *Manager) Session(ctx context.Context, connID uuid.UUID, database string) (engine.Session, *connection.Connection, error) {
	conn, err := m.conns.Get(ctx, connID)
	if err != nil {
		return nil, nil, err
	}
	if database == "" {
		database = conn.Database
	}
	k := key{conn: connID, db: database}

	m.mu.Lock()
	if e, ok := m.sessions[k]; ok {
		e.lastUsed = m.now()
		m.mu.Unlock()
		return e.session, conn, nil
	}
	m.mu.Unlock()

	sess, err := m.open(ctx, *conn, database)
	if err != nil {
		return nil, nil, err
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	if e, ok := m.sessions[k]; ok { // lost a race; keep the first
		sess.Close()
		e.lastUsed = m.now()
		return e.session, conn, nil
	}
	m.sessions[k] = &entry{session: sess, lastUsed: m.now()}
	return sess, conn, nil
}

func (m *Manager) open(ctx context.Context, conn connection.Connection, database string) (engine.Session, error) {
	eng, err := m.Engine(conn.Engine)
	if err != nil {
		return nil, err
	}
	sec, err := m.vault.Get(ctx, conn.ID)
	if err != nil {
		return nil, errs.Wrap(errs.Unavailable, "no secret for connection "+conn.Name, err)
	}
	return eng.Open(ctx, m.target(conn, database, sec))
}

func (m *Manager) target(conn connection.Connection, database, sec string) engine.Target {
	return engine.Target{
		Engine:         conn.Engine,
		Host:           conn.Host,
		Port:           conn.Port,
		Database:       database,
		User:           conn.User,
		Secret:         sec,
		TLS:            conn.TLS,
		AppName:        m.opts.AppName,
		ConnectTimeout: m.opts.ConnectTimeout,
		MaxConns:       m.opts.MaxConns,
	}
}

// Test opens a short-lived session with an explicit secret, pings it and
// returns the server overview and the round-trip latency. Nothing is cached.
func (m *Manager) Test(ctx context.Context, conn connection.Connection, sec string) (*engine.Overview, time.Duration, error) {
	eng, err := m.Engine(conn.Engine)
	if err != nil {
		return nil, 0, err
	}
	resolved, err := secret.Resolve(sec)
	if err != nil {
		return nil, 0, err
	}
	t := m.target(conn, conn.Database, resolved)
	t.MaxConns = 1
	start := m.now()
	sess, err := eng.Open(ctx, t)
	if err != nil {
		return nil, 0, err
	}
	defer sess.Close()
	if err := sess.Ping(ctx); err != nil {
		return nil, 0, err
	}
	latency := m.now().Sub(start)
	ov, err := sess.Overview(ctx)
	if err != nil {
		return nil, latency, err
	}
	return ov, latency, nil
}

// Invalidate closes every session of a connection (after it changed or was deleted).
func (m *Manager) Invalidate(connID uuid.UUID) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for k, e := range m.sessions {
		if k.conn == connID {
			e.session.Close()
			delete(m.sessions, k)
		}
	}
}

// Close shuts every session down.
func (m *Manager) Close() {
	close(m.stop)
	m.mu.Lock()
	defer m.mu.Unlock()
	for k, e := range m.sessions {
		e.session.Close()
		delete(m.sessions, k)
	}
}

func (m *Manager) evictLoop() {
	t := time.NewTicker(m.opts.IdleTimeout / 2)
	defer t.Stop()
	for {
		select {
		case <-m.stop:
			return
		case <-t.C:
			m.evict()
		}
	}
}

func (m *Manager) evict() {
	m.mu.Lock()
	defer m.mu.Unlock()
	cutoff := m.now().Add(-m.opts.IdleTimeout)
	for k, e := range m.sessions {
		if e.lastUsed.Before(cutoff) {
			e.session.Close()
			delete(m.sessions, k)
		}
	}
}
