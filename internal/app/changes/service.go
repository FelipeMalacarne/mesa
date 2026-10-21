// Package changes is the single write path: a change set becomes a plan, the
// policy judges it, it runs in a transaction, and the audit log records it.
package changes

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/felipemalacarne/mesa/internal/core/changeset"
	"github.com/felipemalacarne/mesa/internal/core/connection"
	"github.com/felipemalacarne/mesa/internal/core/errs"
	"github.com/felipemalacarne/mesa/internal/core/ident"
	"github.com/felipemalacarne/mesa/internal/core/policy"
	"github.com/felipemalacarne/mesa/internal/core/schema"
	"github.com/felipemalacarne/mesa/internal/engine"
	"github.com/felipemalacarne/mesa/internal/session"
	"github.com/felipemalacarne/mesa/internal/store"
)

// Unlocks answers whether a connection is unlocked for writes.
type Unlocks interface {
	IsUnlocked(id uuid.UUID) bool
}

// Invalidator is told when a database's structure may have changed.
type Invalidator interface {
	Invalidate(connID uuid.UUID, database string)
}

// Service plans and applies writes.
type Service struct {
	sessions   *session.Manager
	audit      store.Audit
	unlocks    Unlocks
	invalidate Invalidator
	now        func() time.Time
}

// New wires the service. invalidate may be nil.
func New(sessions *session.Manager, audit store.Audit, unlocks Unlocks, invalidate Invalidator) *Service {
	return &Service{sessions: sessions, audit: audit, unlocks: unlocks, invalidate: invalidate, now: time.Now}
}

// PlanResult is a plan with the policy's verdict, before anything runs.
type PlanResult struct {
	Plan     *changeset.Plan
	Decision policy.Decision
}

// ApplyInput runs a change set or a prepared plan.
type ApplyInput struct {
	ConnectionID uuid.UUID
	Database     string
	// Either ChangeSet or Plan is given; a ChangeSet is compiled first.
	ChangeSet *changeset.ChangeSet
	Plan      *changeset.Plan

	Confirmation string
	Transaction  bool // run every statement in one transaction (default true for change sets)
	DryRun       bool // plan and judge, run nothing
	Actor        string
	Surface      string
}

// ApplyResult reports what happened.
type ApplyResult struct {
	Plan     *changeset.Plan
	Decision policy.Decision
	Applied  bool
	AuditID  uuid.UUID
	Duration time.Duration
}

// Plan compiles a change set against the live database and judges it.
func (s *Service) Plan(ctx context.Context, connID uuid.UUID, database string, cs *changeset.ChangeSet) (*PlanResult, error) {
	if err := cs.Validate(); err != nil {
		return nil, err
	}
	sess, conn, err := s.sessions.Session(ctx, connID, database)
	if err != nil {
		return nil, err
	}
	plan, err := s.compile(ctx, sess, cs)
	if err != nil {
		return nil, err
	}
	return &PlanResult{Plan: plan, Decision: s.judge(conn, database, plan, "")}, nil
}

func (s *Service) compile(ctx context.Context, sess engine.Session, cs *changeset.ChangeSet) (*changeset.Plan, error) {
	var current *schema.Database
	if len(cs.Schema) > 0 || len(cs.Rows) > 0 {
		db, err := sess.Database(ctx)
		if err == nil {
			current = db
		}
	}
	return sess.Compile(ctx, current, cs)
}

func (s *Service) judge(conn *connection.Connection, database string, plan *changeset.Plan, confirmation string) policy.Decision {
	if database == "" {
		database = conn.Database
	}
	return policy.Evaluate(policy.Request{
		Environment:  conn.Environment,
		ReadOnly:     conn.ReadOnly,
		Unlocked:     s.unlocks.IsUnlocked(conn.ID),
		Database:     database,
		Confirmation: confirmation,
		Plan:         plan,
	})
}

// Apply runs the change set (or plan) after the policy allows it.
func (s *Service) Apply(ctx context.Context, in ApplyInput) (*ApplyResult, error) {
	sess, conn, err := s.sessions.Session(ctx, in.ConnectionID, in.Database)
	if err != nil {
		return nil, err
	}
	database := in.Database
	if database == "" {
		database = conn.Database
	}
	plan := in.Plan
	if plan == nil {
		if in.ChangeSet == nil {
			return nil, errs.New(errs.Invalid, "nothing to apply")
		}
		if err := in.ChangeSet.Validate(); err != nil {
			return nil, err
		}
		if in.ChangeSet.Database == "" {
			in.ChangeSet.Database = ident.Name(database)
		}
		plan, err = s.compile(ctx, sess, in.ChangeSet)
		if err != nil {
			return nil, err
		}
	}
	res := &ApplyResult{Plan: plan, Decision: s.judge(conn, database, plan, in.Confirmation)}
	if in.DryRun || plan.IsEmpty() {
		return res, nil
	}
	if err := res.Decision.Err(); err != nil {
		return res, err
	}

	entry := store.AuditEntry{
		ID:           uuid.New(),
		ConnectionID: conn.ID,
		Database:     database,
		Environment:  conn.Environment,
		Actor:        in.Actor,
		Surface:      in.Surface,
		Summary:      plan.Title(),
		SQL:          plan.SQL(),
		DownSQL:      plan.DownSQL(),
		Status:       store.AuditStarted,
		StartedAt:    s.now(),
	}
	if err := s.audit.Begin(ctx, entry); err != nil {
		return res, err
	}
	res.AuditID = entry.ID

	start := s.now()
	runErr := s.run(ctx, sess, plan, in.Transaction)
	res.Duration = s.now().Sub(start)

	status, msg := store.AuditApplied, ""
	if runErr != nil {
		status, msg = store.AuditFailed, runErr.Error()
	}
	_ = s.audit.Finish(ctx, entry.ID, status, msg, s.now())
	if runErr != nil {
		return res, errs.Wrap(errs.Conflict, "the database rejected the change", runErr)
	}
	res.Applied = true
	if s.invalidate != nil && plan.Summary.SchemaChanges > 0 {
		s.invalidate.Invalidate(conn.ID, database)
	}
	return res, nil
}

func (s *Service) run(ctx context.Context, sess engine.Session, plan *changeset.Plan, tx bool) error {
	if tx {
		return sess.Tx(ctx, func(t engine.Tx) error {
			for _, st := range plan.Statements {
				if _, err := t.Exec(ctx, st.SQL, st.Args); err != nil {
					return err
				}
			}
			return nil
		})
	}
	for _, st := range plan.Statements {
		if _, err := sess.Query(ctx, st.SQL, st.Args, engine.QueryOptions{}); err != nil {
			return err
		}
	}
	return nil
}

// Audit lists what Mesa ran on a connection, newest first.
func (s *Service) Audit(ctx context.Context, connID uuid.UUID, limit int) ([]store.AuditEntry, error) {
	if limit <= 0 || limit > 500 {
		limit = 50
	}
	return s.audit.List(ctx, &connID, limit)
}
