package policy

import (
	"testing"

	"github.com/felipemalacarne/mesa/internal/core/changeset"
	"github.com/felipemalacarne/mesa/internal/core/connection"
	"github.com/felipemalacarne/mesa/internal/core/errs"
)

var plan = &changeset.Plan{Statements: []changeset.Statement{{SQL: "UPDATE t SET a = 1"}}}

func TestStagingAllows(t *testing.T) {
	d := Evaluate(Request{Environment: connection.Staging, Database: "billing", Plan: plan})
	if d.Outcome != Allow {
		t.Fatalf("got %+v", d)
	}
}

func TestReadOnlyDenies(t *testing.T) {
	d := Evaluate(Request{Environment: connection.Staging, ReadOnly: true, Database: "billing", Plan: plan})
	if d.Outcome != Deny || !errs.Is(d.Err(), errs.Forbidden) {
		t.Fatalf("got %+v", d)
	}
}

func TestProductionNeedsTypedName(t *testing.T) {
	r := Request{Environment: connection.Production, ReadOnly: true, Unlocked: true, Database: "billing", Plan: plan}
	d := Evaluate(r)
	if d.Outcome != NeedsConfirmation || d.ConfirmWith != "billing" || !errs.Is(d.Err(), errs.NeedsConfirmation) {
		t.Fatalf("got %+v", d)
	}
	r.Confirmation = " billing "
	if d := Evaluate(r); d.Outcome != Allow {
		t.Fatalf("confirmed: got %+v", d)
	}
}

func TestDataLossNeedsConfirmationAnywhere(t *testing.T) {
	p := &changeset.Plan{Statements: plan.Statements, Risks: []changeset.Risk{{Level: changeset.Danger, Code: changeset.RiskDataLoss}}}
	d := Evaluate(Request{Environment: connection.Local, Database: "dev", Plan: p})
	if d.Outcome != NeedsConfirmation {
		t.Fatalf("got %+v", d)
	}
}

func TestEmptyPlanAllowed(t *testing.T) {
	if d := Evaluate(Request{Environment: connection.Production, ReadOnly: true, Plan: &changeset.Plan{}}); d.Outcome != Allow {
		t.Fatalf("got %+v", d)
	}
}
