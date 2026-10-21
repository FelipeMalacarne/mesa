// Package designer serves the schema designer: the live model as DBML, the
// diff between an edited model and the database, and the saved layout.
package designer

import (
	"context"

	"github.com/google/uuid"

	"github.com/felipemalacarne/mesa/internal/app/changes"
	"github.com/felipemalacarne/mesa/internal/core/changeset"
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
	if err != nil {
		return nil, err
	}
	if layout == nil {
		layout = map[string]store.Position{}
	}
	return &Model{Schema: db, DBML: dbml.Print(db), Layout: layout, Problems: dbml.Validate(db)}, nil
}

// Parse turns DBML text into a model; parse errors come back as problems.
func (s *Service) Parse(src string) (*schema.Database, []dbml.Problem) {
	db, err := dbml.Parse(src)
	if err != nil {
		return nil, []dbml.Problem{{Level: "error", Message: err.Error()}}
	}
	return db, dbml.Validate(db)
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
	target.Normalize()
	cs := &changeset.ChangeSet{Database: ident.Name(database), Schema: schema.Diff(live, target)}
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
