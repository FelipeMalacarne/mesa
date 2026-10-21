// Package roles lists and changes server roles through reviewed plans.
package roles

import (
	"context"
	"strings"

	"github.com/google/uuid"

	"github.com/felipemalacarne/mesa/internal/app/changes"
	"github.com/felipemalacarne/mesa/internal/core/errs"
	"github.com/felipemalacarne/mesa/internal/core/ident"
	"github.com/felipemalacarne/mesa/internal/engine"
	"github.com/felipemalacarne/mesa/internal/session"
)

// Service manages roles.
type Service struct {
	sessions *session.Manager
	changes  *changes.Service
}

// New wires the service.
func New(sessions *session.Manager, ch *changes.Service) *Service {
	return &Service{sessions: sessions, changes: ch}
}

// List returns every role.
func (s *Service) List(ctx context.Context, connID uuid.UUID) ([]engine.Role, error) {
	sess, _, err := s.sessions.Session(ctx, connID, "")
	if err != nil {
		return nil, err
	}
	return sess.Roles(ctx)
}

// CreateInput creates a role.
type CreateInput struct {
	ConnectionID uuid.UUID
	Role         engine.RoleInput
	Confirmation string
	DryRun       bool
	Actor        string
	Surface      string
}

// Create plans and (unless DryRun) applies a CREATE ROLE.
func (s *Service) Create(ctx context.Context, in CreateInput) (*changes.ApplyResult, error) {
	if strings.TrimSpace(in.Role.Name.String()) == "" {
		return nil, errs.New(errs.Invalid, "role name is required")
	}
	sess, _, err := s.sessions.Session(ctx, in.ConnectionID, "")
	if err != nil {
		return nil, err
	}
	plan := sess.CreateRolePlan(in.Role)
	return s.changes.Apply(ctx, changes.ApplyInput{
		ConnectionID: in.ConnectionID, Plan: plan, Confirmation: in.Confirmation,
		Transaction: true, DryRun: in.DryRun, Actor: in.Actor, Surface: in.Surface,
	})
}

// Drop plans and (unless dryRun) applies a DROP ROLE.
func (s *Service) Drop(ctx context.Context, connID uuid.UUID, name ident.Name, confirmation string, dryRun bool, actor, surface string) (*changes.ApplyResult, error) {
	sess, _, err := s.sessions.Session(ctx, connID, "")
	if err != nil {
		return nil, err
	}
	plan := sess.DropRolePlan(name)
	return s.changes.Apply(ctx, changes.ApplyInput{
		ConnectionID: connID, Plan: plan, Confirmation: confirmation,
		Transaction: true, DryRun: dryRun, Actor: actor, Surface: surface,
	})
}

// CreateDatabase plans and (unless dryRun) applies a CREATE DATABASE. It
// cannot run inside a transaction.
func (s *Service) CreateDatabase(ctx context.Context, connID uuid.UUID, name, owner ident.Name, confirmation string, dryRun bool, actor, surface string) (*changes.ApplyResult, error) {
	sess, _, err := s.sessions.Session(ctx, connID, "")
	if err != nil {
		return nil, err
	}
	plan := sess.CreateDatabasePlan(name, owner)
	return s.changes.Apply(ctx, changes.ApplyInput{
		ConnectionID: connID, Plan: plan, Confirmation: confirmation,
		Transaction: false, DryRun: dryRun, Actor: actor, Surface: surface,
	})
}
