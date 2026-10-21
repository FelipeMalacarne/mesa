// Package monitor reads runtime state and keeps a short series of samples per
// database so the UI can draw rates without its own history.
package monitor

import (
	"context"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/felipemalacarne/mesa/internal/core/errs"
	"github.com/felipemalacarne/mesa/internal/engine"
	"github.com/felipemalacarne/mesa/internal/session"
)

// Unlocks answers whether a connection is unlocked for writes.
type Unlocks interface {
	IsUnlocked(id uuid.UUID) bool
}

// Service samples metrics.
type Service struct {
	sessions *session.Manager
	unlocks  Unlocks
	keep     int           // samples kept per database
	minGap   time.Duration // do not sample more often than this
	now      func() time.Time

	mu     sync.Mutex
	series map[key]*ring
}

type key struct {
	conn uuid.UUID
	db   string
}

// Point is one sample reduced to what the charts need.
type Point struct {
	At        time.Time `json:"at"`
	TPS       float64   `json:"tps"`
	Active    int       `json:"active"`
	Idle      int       `json:"idle"`
	IdleTx    int       `json:"idleTx"`
	LatencyMs float64   `json:"latencyMs"`
}

type ring struct {
	points []Point
	last   *engine.Metrics
	size   int64
	first  time.Time
}

// Snapshot is the current state plus the recent series.
type Snapshot struct {
	Metrics     *engine.Metrics `json:"metrics"`
	TPS         float64         `json:"tps"`
	LatencyMs   float64         `json:"latencyMs"`
	SizeDelta7d int64           `json:"sizeDelta7d,omitempty"`
	Series      []Point         `json:"series"`
}

// New wires the service.
func New(sessions *session.Manager, unlocks Unlocks) *Service {
	return &Service{sessions: sessions, unlocks: unlocks, keep: 240, minGap: 5 * time.Second, now: time.Now, series: map[key]*ring{}}
}

// Overview returns the server header.
func (s *Service) Overview(ctx context.Context, connID uuid.UUID, database string) (*engine.Overview, error) {
	sess, _, err := s.sessions.Session(ctx, connID, database)
	if err != nil {
		return nil, err
	}
	return sess.Overview(ctx)
}

// Snapshot samples metrics (rate-limited) and returns them with the series.
func (s *Service) Snapshot(ctx context.Context, connID uuid.UUID, database string) (*Snapshot, error) {
	sess, conn, err := s.sessions.Session(ctx, connID, database)
	if err != nil {
		return nil, err
	}
	if database == "" {
		database = conn.Database
	}
	k := key{connID, database}

	s.mu.Lock()
	r := s.series[k]
	if r == nil {
		r = &ring{}
		s.series[k] = r
	}
	if r.last != nil && s.now().Sub(r.last.SampledAt) < s.minGap {
		snap := s.snapshot(r)
		s.mu.Unlock()
		return snap, nil
	}
	s.mu.Unlock()

	start := s.now()
	if err := sess.Ping(ctx); err != nil {
		return nil, err
	}
	latency := s.now().Sub(start)
	m, err := sess.Metrics(ctx)
	if err != nil {
		return nil, err
	}
	if m.SampledAt.IsZero() {
		m.SampledAt = s.now()
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	p := Point{At: m.SampledAt, Active: m.ConnectionsActive, Idle: m.ConnectionsIdle, IdleTx: m.ConnectionsIdleTx, LatencyMs: float64(latency) / float64(time.Millisecond)}
	if r.last != nil {
		dt := m.SampledAt.Sub(r.last.SampledAt).Seconds()
		if dt > 0 {
			p.TPS = float64((m.Commits+m.Rollbacks)-(r.last.Commits+r.last.Rollbacks)) / dt
			if p.TPS < 0 {
				p.TPS = 0 // stats reset
			}
		}
	} else {
		r.first = m.SampledAt
		r.size = m.DatabaseSizeBytes
	}
	r.points = append(r.points, p)
	if len(r.points) > s.keep {
		r.points = r.points[len(r.points)-s.keep:]
	}
	r.last = m
	return s.snapshot(r), nil
}

func (s *Service) snapshot(r *ring) *Snapshot {
	snap := &Snapshot{Metrics: r.last, Series: append([]Point(nil), r.points...)}
	if n := len(r.points); n > 0 {
		snap.TPS = r.points[n-1].TPS
		snap.LatencyMs = r.points[n-1].LatencyMs
	}
	if r.last != nil && r.size > 0 {
		snap.SizeDelta7d = r.last.DatabaseSizeBytes - r.size
	}
	return snap
}

// Sessions lists active backends.
func (s *Service) Sessions(ctx context.Context, connID uuid.UUID, database string) ([]engine.ActiveSession, error) {
	sess, _, err := s.sessions.Session(ctx, connID, database)
	if err != nil {
		return nil, err
	}
	return sess.Sessions(ctx)
}

// Kill cancels or terminates a backend. It is a write: read-only connections
// must be unlocked first.
func (s *Service) Kill(ctx context.Context, connID uuid.UUID, database string, pid int, terminate bool) error {
	sess, conn, err := s.sessions.Session(ctx, connID, database)
	if err != nil {
		return err
	}
	if conn.ReadOnly && !s.unlocks.IsUnlocked(conn.ID) {
		return errs.New(errs.Forbidden, "connection is read-only; unlock it for this session to kill sessions")
	}
	return sess.Kill(ctx, pid, terminate)
}

// Panel returns the engine-specific panel.
func (s *Service) Panel(ctx context.Context, connID uuid.UUID, database string) (*engine.Panel, error) {
	sess, _, err := s.sessions.Session(ctx, connID, database)
	if err != nil {
		return nil, err
	}
	return sess.EnginePanel(ctx)
}
