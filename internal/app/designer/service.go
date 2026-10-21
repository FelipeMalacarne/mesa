// Package designer serves the schema designer: the live model as DBML, the
// diff between an edited model and the database, and the saved layout.
package designer

import (
	"context"

	"github.com/google/uuid"

	"github.com/felipemalacarne/mesa/internal/app/changes"
	"github.com/felipemalacarne/mesa/internal/core/changeset"
	"github.com/felipemalacarne/mesa/internal/core/errs"
	"github.com/felipemalacarne/mesa/internal/core/ident"
	"github.com/felipemalacarne/mesa/internal/core/policy"
	"github.com/felipemalacarne/mesa/internal/core/schema"
	"github.com/felipemalacarne/mesa/internal/dbml"
	"github.com/felipemalacarne/mesa/internal/session"
	"github.com/felipemalacarne/mesa/internal/store"
)

// Service is the designer use case.
type Service struct {
	sessions *session.Manager
	changes  *changes.Service
	layouts  store.Layouts
	live     func(ctx context.Context, connID uuid.UUID, database string, refresh bool) (*schema.Database, error)
}

// New wires the service; live reads the current model (explore.Service.Schema).
func New(sessions *session.Manager, ch *changes.Service, layouts store.Layouts,
	live func(ctx context.Context, connID uuid.UUID, database string, refresh bool) (*schema.Database, error)) *Service {
	return &Service{sessions: sessions, changes: ch, layouts: layouts, live: live}
}

// Model is what the designer opens with.
type Model struct {
	Schema   *schema.Database
	DBML     string
	Layout   map[string]store.Position
	Problems []dbml.Problem
}

// Model returns the live schema, its DBML and the saved positions.
func (s *Service) Model(ctx context.Context, connID uuid.UUID, database string) (*Model, error) {
	db, err := s.live(ctx, connID, database, true)
	if err != nil {
		return nil, err
	}
	layout, err := s.layouts.Get(ctx, connID, database)
	if err != nil && !errs.Is(err, errs.NotFound) {
		return nil, err
	}
	if layout == nil {
		layout = map[string]store.Position{}
	}
	return &Model{Schema: db, DBML: dbml.Print(db), Layout: layout, Problems: dbml.Validate(db)}, nil
}

// Parse turns DBML text into a model; parse errors come back as problems.
func (s *Service) Parse(src string) (*schema.Database, []dbml.Problem) {
	problems := dbml.Check(src) // carries source lines, syntax errors included
	db, err := dbml.Parse(src)
	if err != nil {
		return nil, problems
	}
	return db, problems
}

// Print renders a model as DBML.
func (s *Service) Print(db *schema.Database) string { return dbml.Print(db) }

// Diff is the designer's review: changes, their plan and the policy verdict.
type Diff struct {
	Changes  []schema.Change
	Plan     *changeset.Plan
	Decision policy.Decision
}

// Diff compares an edited model with the live database and plans the SQL.
func (s *Service) Diff(ctx context.Context, connID uuid.UUID, database string, target *schema.Database) (*Diff, error) {
	live, err := s.live(ctx, connID, database, true)
	if err != nil {
		return nil, err
	}
	sess, _, err := s.sessions.Session(ctx, connID, database)
	if err != nil {
		return nil, err
	}
	target.Normalize()
	Preserve(live, target)
	opts := schema.DiffOptions{SameType: sess.SameType, SameDefault: sess.SameDefault}
	cs := &changeset.ChangeSet{Database: ident.Name(database), Schema: schema.DiffWith(live, target, opts)}
	if len(cs.Schema) == 0 {
		return &Diff{Changes: []schema.Change{}, Plan: &changeset.Plan{Database: cs.Database}, Decision: policy.Decision{Outcome: policy.Allow}}, nil
	}
	pr, err := s.changes.Plan(ctx, connID, database, cs)
	if err != nil {
		return nil, err
	}
	return &Diff{Changes: cs.Schema, Plan: pr.Plan, Decision: pr.Decision}, nil
}

// SaveLayout stores diagram positions.
func (s *Service) SaveLayout(ctx context.Context, connID uuid.UUID, database string, positions map[string]store.Position) error {
	return s.layouts.Put(ctx, connID, database, positions)
}

// Preserve copies into target what DBML cannot express, from the live
// model, so a model edited as DBML never drops it by omission: check
// constraints, partial-index predicates, index methods, ALWAYS identities,
// and views, functions and sequences. A table or index that is gone from
// target stays gone.
func Preserve(live, target *schema.Database) {
	for si := range target.Schemas {
		ts := &target.Schemas[si]
		ls := live.Schema(ts.Name)
		if ls == nil {
			continue
		}
		if len(ts.Views) == 0 {
			ts.Views = ls.Views
		}
		if len(ts.Functions) == 0 {
			ts.Functions = ls.Functions
		}
		if len(ts.Sequences) == 0 {
			ts.Sequences = ls.Sequences
		}
		for ti := range ts.Tables {
			tt := &ts.Tables[ti]
			lt := ls.Table(tt.Name)
			if lt == nil {
				continue
			}
			if len(tt.Checks) == 0 {
				tt.Checks = lt.Checks
			}
			for ci := range tt.Columns {
				tc := &tt.Columns[ci]
				if lc := lt.Column(tc.Name); lc != nil && lc.Identity == schema.IdentityAlways && tc.Identity == schema.IdentityByDefault {
					tc.Identity = schema.IdentityAlways
				}
			}
			for ii := range tt.Indexes {
				ti := &tt.Indexes[ii]
				for _, li := range lt.Indexes {
					if li.Name != ti.Name {
						continue
					}
					if ti.Predicate == "" {
						ti.Predicate = li.Predicate
					}
					if ti.Method == "" {
						ti.Method = li.Method
					}
				}
			}
		}
	}
}
