// Package explore reads structure and rows: the tree, the structure view and
// the data grid all come from here.
package explore

import (
	"context"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/felipemalacarne/mesa/internal/core/errs"
	"github.com/felipemalacarne/mesa/internal/core/ident"
	"github.com/felipemalacarne/mesa/internal/core/schema"
	"github.com/felipemalacarne/mesa/internal/engine"
	"github.com/felipemalacarne/mesa/internal/session"
)

// Service reads catalogs and rows through pooled sessions.
type Service struct {
	sessions *session.Manager
	ttl      time.Duration
	now      func() time.Time

	mu    sync.Mutex
	cache map[cacheKey]cacheEntry
}

type cacheKey struct {
	conn uuid.UUID
	db   string
}

type cacheEntry struct {
	db *schema.Database
	at time.Time
}

// New wires the service with a short schema cache (default 30s).
func New(sessions *session.Manager, ttl time.Duration) *Service {
	if ttl == 0 {
		ttl = 30 * time.Second
	}
	return &Service{sessions: sessions, ttl: ttl, now: time.Now, cache: map[cacheKey]cacheEntry{}}
}

// Databases lists the databases the connection's role can see.
func (s *Service) Databases(ctx context.Context, connID uuid.UUID) ([]engine.DatabaseInfo, error) {
	sess, _, err := s.sessions.Session(ctx, connID, "")
	if err != nil {
		return nil, err
	}
	return sess.Databases(ctx)
}

// Schema returns the model of a database, from a short cache unless refresh.
func (s *Service) Schema(ctx context.Context, connID uuid.UUID, database string, refresh bool) (*schema.Database, error) {
	sess, conn, err := s.sessions.Session(ctx, connID, database)
	if err != nil {
		return nil, err
	}
	if database == "" {
		database = conn.Database
	}
	k := cacheKey{connID, database}
	if !refresh {
		s.mu.Lock()
		e, ok := s.cache[k]
		s.mu.Unlock()
		if ok && s.now().Sub(e.at) < s.ttl {
			return e.db, nil
		}
	}
	db, err := sess.Database(ctx)
	if err != nil {
		return nil, err
	}
	s.mu.Lock()
	s.cache[k] = cacheEntry{db: db, at: s.now()}
	s.mu.Unlock()
	return db, nil
}

// Invalidate drops the cached model of a database.
func (s *Service) Invalidate(connID uuid.UUID, database string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if database == "" {
		for k := range s.cache {
			if k.conn == connID {
				delete(s.cache, k)
			}
		}
		return
	}
	delete(s.cache, cacheKey{connID, database})
}

// Table returns one table with fresh stats.
func (s *Service) Table(ctx context.Context, connID uuid.UUID, database string, q ident.Qualified) (*schema.Table, error) {
	sess, _, err := s.sessions.Session(ctx, connID, database)
	if err != nil {
		return nil, err
	}
	t, err := sess.Table(ctx, q)
	if err != nil {
		return nil, err
	}
	if t == nil {
		return nil, errs.Newf(errs.NotFound, "table %s not found", q)
	}
	return t, nil
}

// Rows pages through a table.
func (s *Service) Rows(ctx context.Context, connID uuid.UUID, database string, q engine.RowsQuery) (*engine.RowsPage, error) {
	if q.Limit <= 0 {
		q.Limit = 100
	}
	if q.Limit > 1000 {
		q.Limit = 1000
	}
	if q.Offset < 0 {
		q.Offset = 0
	}
	sess, _, err := s.sessions.Session(ctx, connID, database)
	if err != nil {
		return nil, err
	}
	return sess.TableRows(ctx, q)
}

// Roles lists server roles.
func (s *Service) Roles(ctx context.Context, connID uuid.UUID) ([]engine.Role, error) {
	sess, _, err := s.sessions.Session(ctx, connID, "")
	if err != nil {
		return nil, err
	}
	return sess.Roles(ctx)
}

// Capabilities reports what the connection's engine supports.
func (s *Service) Capabilities(ctx context.Context, connID uuid.UUID) (engine.Capabilities, error) {
	_, conn, err := s.sessions.Session(ctx, connID, "")
	if err != nil {
		return engine.Capabilities{}, err
	}
	eng, err := s.sessions.Engine(conn.Engine)
	if err != nil {
		return engine.Capabilities{}, err
	}
	return eng.Capabilities(), nil
}
