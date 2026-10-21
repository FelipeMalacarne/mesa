// Package policy decides whether a plan may run against a connection. It is
// the one place the safety rules live, so every surface enforces the same.
package policy

import (
	"strings"

	"github.com/felipemalacarne/mesa/internal/core/changeset"
	"github.com/felipemalacarne/mesa/internal/core/connection"
	"github.com/felipemalacarne/mesa/internal/core/errs"
)

// Request describes a write about to happen.
type Request struct {
	Environment connection.Environment
	ReadOnly    bool   // the connection's read-only flag
	Unlocked    bool   // the person unlocked this connection for the session
	Database    string // the database the plan runs on
	// Confirmation is what the person typed when asked to confirm, if anything.
	Confirmation string
	Plan         *changeset.Plan
}

// Outcome is the policy's verdict.
type Outcome string

const (
	Allow             Outcome = "allow"
	NeedsConfirmation Outcome = "needs_confirmation"
	Deny              Outcome = "deny"
)

// Decision is the result of Evaluate.
type Decision struct {
	Outcome Outcome `json:"outcome"`
	Reason  string  `json:"reason,omitempty"`
	// ConfirmWith is the text the person must type when Outcome is NeedsConfirmation.
	ConfirmWith string `json:"confirmWith,omitempty"`
}

// Err turns a non-allow decision into an error with the matching kind.
func (d Decision) Err() error {
	switch d.Outcome {
	case Deny:
		return errs.New(errs.Forbidden, d.Reason)
	case NeedsConfirmation:
		return errs.New(errs.NeedsConfirmation, d.Reason)
	}
	return nil
}

// Evaluate applies the rules:
//
//  1. A read-only connection denies every write until unlocked for the session.
//  2. Production always asks the person to type the database name.
//  3. A plan that can lose data asks for the same confirmation on any environment.
//  4. An empty plan is allowed (there is nothing to run).
func Evaluate(r Request) Decision {
	if r.Plan == nil || r.Plan.IsEmpty() {
		return Decision{Outcome: Allow}
	}
	if r.ReadOnly && !r.Unlocked {
		return Decision{Outcome: Deny, Reason: "connection is read-only; unlock it for this session to write"}
	}
	confirm := r.Environment == connection.Production || r.Plan.Has(changeset.RiskDataLoss)
	if !confirm {
		return Decision{Outcome: Allow}
	}
	if strings.TrimSpace(r.Confirmation) == r.Database && r.Database != "" {
		return Decision{Outcome: Allow}
	}
	reason := "writing to Production: type the database name to confirm"
	if r.Environment != connection.Production {
		reason = "this change can lose data: type the database name to confirm"
	}
	return Decision{Outcome: NeedsConfirmation, Reason: reason, ConfirmWith: r.Database}
}

// CanUnlock reports whether a connection may be unlocked at all. Mesa allows
// it everywhere; the rule exists so a deployment can tighten it later.
func CanUnlock(env connection.Environment) bool { return env != "" }
