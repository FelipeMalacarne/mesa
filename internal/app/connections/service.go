// Package connections manages the connections Mesa knows and their secrets.
package connections

import (
	"context"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/felipemalacarne/mesa/internal/core/connection"
	"github.com/felipemalacarne/mesa/internal/engine"
	"github.com/felipemalacarne/mesa/internal/secret"
	"github.com/felipemalacarne/mesa/internal/session"
	"github.com/felipemalacarne/mesa/internal/store"
)

// Unlocks tracks which read-only connections a person unlocked for a while.
// It is in-memory on purpose: an unlock never outlives the process.
type Unlocks interface {
	Until(id uuid.UUID) (time.Time, bool)
	Unlock(id uuid.UUID, ttl time.Duration) time.Time
	Lock(id uuid.UUID)
}

// Service is the connections use case.
type Service struct {
	conns    store.Connections
	vault    secret.Vault
	sessions *session.Manager
	unlocks  Unlocks
	now      func() time.Time
	// definedIn reports the project file of project connections.
	definedIn func(uuid.UUID) string
}

// SetDefinedIn tells the service which connections come from a project file.
func (s *Service) SetDefinedIn(fn func(uuid.UUID) string) { s.definedIn = fn }

// New wires the service.
func New(conns store.Connections, vault secret.Vault, sessions *session.Manager, unlocks Unlocks) *Service {
	return &Service{conns: conns, vault: vault, sessions: sessions, unlocks: unlocks, now: time.Now}
}

// Input is what a person fills in, including the secret.
type Input struct {
	connection.Input
	Secret string // empty on update keeps the stored one
}

// Item is a connection with its runtime state.
type Item struct {
	connection.Connection
	LastOpenedAt  *time.Time `json:"lastOpenedAt,omitempty"`
	UnlockedUntil *time.Time `json:"unlockedUntil,omitempty"`
	// DefinedIn names the project file that defines the connection
	// (".mesa/mesa.toml"); empty for the person's own connections.
	DefinedIn string `json:"definedIn,omitempty"`
}

// TestResult is the outcome of reaching a server.
type TestResult struct {
	Latency  time.Duration
	Overview *engine.Overview
}

func (s *Service) item(c connection.Connection, opened map[uuid.UUID]time.Time) Item {
	it := Item{Connection: c}
	if s.definedIn != nil {
		it.DefinedIn = s.definedIn(c.ID)
	}
	if t, ok := opened[c.ID]; ok && !t.IsZero() {
		t := t
		it.LastOpenedAt = &t
	}
	if until, ok := s.unlocks.Until(c.ID); ok && until.After(s.now()) {
		it.UnlockedUntil = &until
	}
	return it
}

// List returns every connection, newest last-opened first.
func (s *Service) List(ctx context.Context) ([]Item, error) {
	list, err := s.conns.List(ctx)
	if err != nil {
		return nil, err
	}
	opened, err := s.conns.LastOpened(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]Item, 0, len(list))
	for _, c := range list {
		out = append(out, s.item(c, opened))
	}
	return out, nil
}

// Get returns one connection.
func (s *Service) Get(ctx context.Context, id uuid.UUID) (*Item, error) {
	c, err := s.conns.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	opened, err := s.conns.LastOpened(ctx)
	if err != nil {
		return nil, err
	}
	it := s.item(*c, opened)
	return &it, nil
}

// Create stores a new connection and its secret.
func (s *Service) Create(ctx context.Context, in Input) (*Item, error) {
	c, err := connection.New(in.Input, s.now())
	if err != nil {
		return nil, err
	}
	// An empty secret is allowed: trust and peer authentication need none.
	if err := s.conns.Put(ctx, c); err != nil {
		return nil, err
	}
	if err := s.vault.Set(ctx, c.ID, in.Secret); err != nil {
		_ = s.conns.Delete(ctx, c.ID)
		return nil, err
	}
	it := s.item(c, nil)
	return &it, nil
}

// Update changes settings; the secret changes only when given.
func (s *Service) Update(ctx context.Context, id uuid.UUID, in Input) (*Item, error) {
	c, err := s.conns.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	if err := c.Update(in.Input, s.now()); err != nil {
		return nil, err
	}
	if err := s.conns.Put(ctx, *c); err != nil {
		return nil, err
	}
	if strings.TrimSpace(in.Secret) != "" {
		if err := s.vault.Set(ctx, id, in.Secret); err != nil {
			return nil, err
		}
	}
	s.sessions.Invalidate(id)
	s.unlocks.Lock(id)
	return s.Get(ctx, id)
}

// Delete removes the connection, its secret and its sessions.
func (s *Service) Delete(ctx context.Context, id uuid.UUID) error {
	if _, err := s.conns.Get(ctx, id); err != nil {
		return err
	}
	s.sessions.Invalidate(id)
	s.unlocks.Lock(id)
	_ = s.vault.Delete(ctx, id)
	return s.conns.Delete(ctx, id)
}

// Test reaches a server with explicit settings and secret; nothing is stored.
func (s *Service) Test(ctx context.Context, in Input) (*TestResult, error) {
	c, err := connection.New(in.Input, s.now())
	if err != nil {
		return nil, err
	}
	ov, latency, err := s.sessions.Test(ctx, c, in.Secret)
	if err != nil {
		return nil, err
	}
	return &TestResult{Latency: latency, Overview: ov}, nil
}

// Ping reaches a stored connection and records that it was opened.
func (s *Service) Ping(ctx context.Context, id uuid.UUID) (*TestResult, error) {
	c, err := s.conns.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	sec, err := s.vault.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	ov, latency, err := s.sessions.Test(ctx, *c, sec)
	if err != nil {
		return nil, err
	}
	return &TestResult{Latency: latency, Overview: ov}, nil
}

// TouchOpened records that a workspace was opened.
func (s *Service) TouchOpened(ctx context.Context, id uuid.UUID) error {
	return s.conns.TouchOpened(ctx, id, s.now())
}

// Unlock allows writes on a read-only connection for ttl.
func (s *Service) Unlock(ctx context.Context, id uuid.UUID, ttl time.Duration) (*Item, error) {
	if _, err := s.conns.Get(ctx, id); err != nil {
		return nil, err
	}
	if ttl <= 0 {
		ttl = time.Hour
	}
	if ttl > 24*time.Hour {
		ttl = 24 * time.Hour
	}
	s.unlocks.Unlock(id, ttl)
	return s.Get(ctx, id)
}

// Lock ends an unlock early.
func (s *Service) Lock(ctx context.Context, id uuid.UUID) (*Item, error) {
	s.unlocks.Lock(id)
	return s.Get(ctx, id)
}
