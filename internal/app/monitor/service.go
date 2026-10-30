// Package monitor reads runtime state and keeps a short series of samples per
// database so the UI can draw rates without its own history.
package monitor

import (
	"context"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/felipemalacarne/mesa/internal/core/account"
	"github.com/felipemalacarne/mesa/internal/core/errs"
	"github.com/felipemalacarne/mesa/internal/engine"
	"github.com/felipemalacarne/mesa/internal/session"
	"github.com/felipemalacarne/mesa/internal/store"
)

// Unlocks answers whether a connection is unlocked for writes.
type Unlocks interface {
	IsUnlocked(id uuid.UUID) bool
}

// Ranges the Monitor can show from stored samples; "live" is the in-memory
// series of the current page.
var Ranges = map[string]time.Duration{
	"15m": 15 * time.Minute,
	"1h":  time.Hour,
	"6h":  6 * time.Hour,
	"24h": 24 * time.Hour,
	"7d":  7 * 24 * time.Hour,
}

// Retention is how long samples are kept.
const Retention = 8 * 24 * time.Hour

// Service samples metrics. A database whose Monitor has been opened stays
// watched: Run samples it in the background so the ranges have history.
type Service struct {
	sessions *session.Manager
	unlocks  Unlocks
	history  store.Metrics // nil keeps only the in-memory series
	keep     int           // samples kept in memory per database
	minGap   time.Duration // do not sample more often than this
	every    time.Duration // background sampling interval
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
	points   []Point
	last     *engine.Metrics
	size     int64
	failures int
}

// Snapshot is the current state plus a series: the recent in-memory one, or
// stored samples for a range.
type Snapshot struct {
	Metrics     *engine.Metrics `json:"metrics"`
	TPS         float64         `json:"tps"`
	LatencyMs   float64         `json:"latencyMs"`
	SizeDelta7d int64           `json:"sizeDelta7d,omitempty"`
	Series      []Point         `json:"series"`
}

// New wires the service; history may be nil.
func New(sessions *session.Manager, unlocks Unlocks, history store.Metrics) *Service {
	return &Service{sessions: sessions, unlocks: unlocks, history: history, keep: 240, minGap: 5 * time.Second, every: 30 * time.Second, now: time.Now, series: map[key]*ring{}}
}

// Overview returns the server header.
func (s *Service) Overview(ctx context.Context, connID uuid.UUID, database string) (*engine.Overview, error) {
	sess, _, err := s.sessions.Session(ctx, connID, database)
	if err != nil {
		return nil, err
	}
	return sess.Overview(ctx)
}

// Snapshot samples metrics (rate-limited) and returns them with the series
// for rng: "" or "live" for the in-memory series, else a key of Ranges.
func (s *Service) Snapshot(ctx context.Context, connID uuid.UUID, database, rng string) (*Snapshot, error) {
	window, stored := Ranges[rng]
	if rng != "" && rng != "live" && !stored {
		return nil, errs.Newf(errs.Invalid, "unknown range %q (live, 15m, 1h, 6h, 24h, 7d)", rng)
	}
	_, conn, err := s.sessions.Session(ctx, connID, database)
	if err != nil {
		return nil, err
	}
	if database == "" {
		database = conn.Database
	}
	k := key{connID, database}
	if err := s.sample(ctx, k, s.minGap); err != nil {
		return nil, err
	}
	s.mu.Lock()
	snap := s.snapshot(s.series[k])
	s.mu.Unlock()
	if s.history != nil {
		now := s.now()
		if old, err := s.history.Range(ctx, connID, database, now.Add(-7*24*time.Hour), now, 1); err == nil && len(old) == 1 && snap.Metrics != nil && old[0].SizeBytes > 0 {
			snap.SizeDelta7d = snap.Metrics.DatabaseSizeBytes - old[0].SizeBytes
		}
		if stored {
			points, err := s.history.Range(ctx, connID, database, now.Add(-window), now, 240)
			if err != nil {
				return nil, err
			}
			snap.Series = make([]Point, 0, len(points))
			for _, p := range points {
				snap.Series = append(snap.Series, Point{At: p.At, TPS: p.TPS, Active: p.Active, Idle: p.Idle, IdleTx: p.IdleTx, LatencyMs: p.LatencyMs})
			}
		}
	}
	return snap, nil
}

// sample reads metrics unless the last sample is newer than gap, keeps the
// point in memory and in history. The database becomes watched.
func (s *Service) sample(ctx context.Context, k key, gap time.Duration) error {
	s.mu.Lock()
	r := s.series[k]
	if r == nil {
		r = &ring{}
		s.series[k] = r
	}
	fresh := r.last != nil && s.now().Sub(r.last.SampledAt) < gap
	s.mu.Unlock()
	if fresh {
		return nil
	}
	sess, _, err := s.sessions.Session(ctx, k.conn, k.db)
	if err != nil {
		return err
	}
	start := s.now()
	if err := sess.Ping(ctx); err != nil {
		return err
	}
	latency := s.now().Sub(start)
	m, err := sess.Metrics(ctx)
	if err != nil {
		return err
	}
	if m.SampledAt.IsZero() {
		m.SampledAt = s.now()
	}

	s.mu.Lock()
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
		r.size = m.DatabaseSizeBytes
	}
	r.points = append(r.points, p)
	if len(r.points) > s.keep {
		r.points = r.points[len(r.points)-s.keep:]
	}
	first := r.last == nil
	r.last = m
	r.failures = 0
	s.mu.Unlock()

	// The first sample has no rate yet; history starts with the second.
	if s.history != nil && !first {
		_ = s.history.Add(ctx, k.conn, k.db, store.MetricPoint{At: p.At, TPS: p.TPS, Active: p.Active, Idle: p.Idle, IdleTx: p.IdleTx, LatencyMs: p.LatencyMs, SizeBytes: m.DatabaseSizeBytes})
	}
	return nil
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

// Run samples every watched database until ctx ends, and prunes old
// samples. The service and desktop binaries start it; the CLI does not.
func (s *Service) Run(ctx context.Context) {
	tick := time.NewTicker(s.every)
	defer tick.Stop()
	lastPrune := time.Time{}
	for {
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
		}
		s.mu.Lock()
		keys := make([]key, 0, len(s.series))
		for k := range s.series {
			keys = append(keys, k)
		}
		s.mu.Unlock()
		for _, k := range keys {
			sctx, cancel := context.WithTimeout(ctx, 10*time.Second)
			err := s.sample(sctx, k, s.every/2)
			cancel()
			if err != nil {
				s.mu.Lock()
				if r := s.series[k]; r != nil {
					r.failures++
					// A database that keeps failing (deleted connection, dropped
					// database) stops being watched until opened again.
					if r.failures >= 10 {
						delete(s.series, k)
					}
				}
				s.mu.Unlock()
			}
		}
		if s.history != nil && s.now().Sub(lastPrune) > time.Hour {
			_ = s.history.Prune(ctx, s.now().Add(-Retention))
			lastPrune = s.now()
		}
	}
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
	if err := account.Require(ctx, account.Write); err != nil {
		return err
	}
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
