// Package query runs ad-hoc SQL and keeps history and saved queries.
package query

import (
	"context"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/felipemalacarne/mesa/internal/core/errs"
	"github.com/felipemalacarne/mesa/internal/engine"
	"github.com/felipemalacarne/mesa/internal/session"
	"github.com/felipemalacarne/mesa/internal/store"
)

// Unlocks answers whether a connection is unlocked for writes.
type Unlocks interface {
	IsUnlocked(id uuid.UUID) bool
}

// Service runs queries.
type Service struct {
	sessions *session.Manager
	history  store.History
	saved    store.SavedQueries
	unlocks  Unlocks
	now      func() time.Time
}

// New wires the service.
func New(sessions *session.Manager, history store.History, saved store.SavedQueries, unlocks Unlocks) *Service {
	return &Service{sessions: sessions, history: history, saved: saved, unlocks: unlocks, now: time.Now}
}

// RunInput is one query.
type RunInput struct {
	ConnectionID uuid.UUID
	Database     string
	SQL          string
	MaxRows      int
	ReadOnly     bool
	Timeout      time.Duration
}

// Run executes SQL. A read-only connection that is not unlocked runs inside a
// read-only transaction, so the database itself refuses writes.
func (s *Service) Run(ctx context.Context, in RunInput) (*engine.Result, error) {
	if strings.TrimSpace(in.SQL) == "" {
		return nil, errs.New(errs.Invalid, "query is empty")
	}
	sess, conn, err := s.sessions.Session(ctx, in.ConnectionID, in.Database)
	if err != nil {
		return nil, err
	}
	opts := engine.QueryOptions{MaxRows: in.MaxRows, Timeout: in.Timeout}
	if opts.MaxRows <= 0 {
		opts.MaxRows = 500
	}
	if opts.MaxRows > 10000 {
		opts.MaxRows = 10000
	}
	opts.ReadOnly = in.ReadOnly || (conn.ReadOnly && !s.unlocks.IsUnlocked(conn.ID))

	res, runErr := sess.Query(ctx, in.SQL, nil, opts)
	rec := store.QueryRun{
		ID:           uuid.New(),
		ConnectionID: conn.ID,
		Database:     databaseOf(in.Database, conn.Database),
		SQL:          in.SQL,
		RanAt:        s.now(),
	}
	if runErr != nil {
		rec.Error = errs.Message(runErr)
	} else {
		rec.RowCount = int64(len(res.Rows))
		if res.Command != "" && res.RowsAffected > 0 && len(res.Rows) == 0 {
			rec.RowCount = res.RowsAffected
		}
		rec.Duration = res.Duration
	}
	_ = s.history.Add(ctx, rec)
	return res, runErr
}

func databaseOf(given, fallback string) string {
	if given == "" {
		return fallback
	}
	return given
}

// Explain returns the plan text for a query.
func (s *Service) Explain(ctx context.Context, connID uuid.UUID, database, sql string, analyze bool) ([]string, error) {
	if strings.TrimSpace(sql) == "" {
		return nil, errs.New(errs.Invalid, "query is empty")
	}
	sess, conn, err := s.sessions.Session(ctx, connID, database)
	if err != nil {
		return nil, err
	}
	if analyze && conn.ReadOnly && !s.unlocks.IsUnlocked(conn.ID) {
		return nil, errs.New(errs.Forbidden, "EXPLAIN ANALYZE runs the query; unlock the connection first")
	}
	return sess.Explain(ctx, sql, nil, analyze)
}

// History lists recent runs on a connection.
func (s *Service) History(ctx context.Context, connID uuid.UUID, limit int) ([]store.QueryRun, error) {
	if limit <= 0 || limit > 500 {
		limit = 50
	}
	return s.history.List(ctx, connID, limit)
}

// Saved lists saved queries.
func (s *Service) Saved(ctx context.Context) ([]store.SavedQuery, error) {
	return s.saved.List(ctx)
}

// Save creates or updates a saved query.
func (s *Service) Save(ctx context.Context, q store.SavedQuery) (*store.SavedQuery, error) {
	if strings.TrimSpace(q.Name) == "" {
		return nil, errs.New(errs.Invalid, "name is required")
	}
	if strings.TrimSpace(q.SQL) == "" {
		return nil, errs.New(errs.Invalid, "query is empty")
	}
	if q.ID == uuid.Nil {
		q.ID = uuid.New()
	}
	q.UpdatedAt = s.now()
	if err := s.saved.Put(ctx, q); err != nil {
		return nil, err
	}
	return &q, nil
}

// DeleteSaved removes a saved query.
func (s *Service) DeleteSaved(ctx context.Context, id uuid.UUID) error {
	return s.saved.Delete(ctx, id)
}
