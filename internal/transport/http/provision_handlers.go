package http

import (
	"context"

	"github.com/felipemalacarne/mesa/internal/app/provision"
	"github.com/felipemalacarne/mesa/internal/transport/http/gen"
)

func toTarget(t provision.Target) gen.ProvisioningTarget {
	return gen.ProvisioningTarget{Repo: t.Repo, Branch: t.Branch, Path: t.Path, Cluster: t.Cluster, Namespace: t.Namespace, PullRequest: t.PullRequest}
}

func (h *handlers) GetProvisioningTarget(_ context.Context, r gen.GetProvisioningTargetRequestObject) (gen.GetProvisioningTargetResponseObject, error) {
	t, err := h.app.Provision.Target(r.ConnectionId)
	if err != nil {
		return nil, err
	}
	return gen.GetProvisioningTarget200JSONResponse(toTarget(t)), nil
}

func (h *handlers) ProvisionDatabase(ctx context.Context, r gen.ProvisionDatabaseRequestObject) (gen.ProvisionDatabaseResponseObject, error) {
	res, err := h.app.Provision.Provision(ctx, provision.Input{
		ConnectionID: r.ConnectionId, Name: r.Body.Name, Namespace: r.Body.Namespace,
		Confirmation: deref(r.Body.Confirmation, ""), DryRun: deref(r.Body.DryRun, false),
		Actor: actor(ctx), Surface: surface,
	})
	if err != nil {
		return nil, err
	}
	out := gen.ProvisionResult{
		Target: toTarget(res.Target), Statements: res.Statements, Files: res.Files, Decision: res.Decision,
		RoleSecret: res.RoleSecret, AppSecret: res.AppSecret, Applied: res.Applied,
	}
	if res.Note != "" {
		out.Note = ptr(res.Note)
	}
	if c := res.Commit; c != nil {
		out.Commit = &struct {
			PullRequestUrl *string `json:"pullRequestUrl,omitempty"`
			Sha            string  `json:"sha"`
			Url            string  `json:"url"`
		}{Sha: c.SHA, Url: c.URL}
		if c.PullRequestURL != "" {
			out.Commit.PullRequestUrl = ptr(c.PullRequestURL)
		}
	}
	return gen.ProvisionDatabase200JSONResponse(out), nil
}
