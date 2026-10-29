// Package provision creates an app's database on a CloudNativePG cluster in
// one step, and keeps the GitOps repository the cluster is reconciled from
// in step with it: the role and database are created now, through the same
// reviewed and audited path as every write, and their declaration (the CNPG
// Database, the managed role, the role's password and the app's connection
// Secret, encrypted with sops) is committed to the repository, so Argo CD
// or Flux keeps them and the app's namespace gets its Secret.
package provision

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/felipemalacarne/mesa/internal/app/changes"
	"github.com/felipemalacarne/mesa/internal/cnpg"
	"github.com/felipemalacarne/mesa/internal/core/account"
	"github.com/felipemalacarne/mesa/internal/core/changeset"
	"github.com/felipemalacarne/mesa/internal/core/errs"
	"github.com/felipemalacarne/mesa/internal/core/ident"
	"github.com/felipemalacarne/mesa/internal/core/policy"
	"github.com/felipemalacarne/mesa/internal/engine"
	"github.com/felipemalacarne/mesa/internal/session"
	"github.com/felipemalacarne/mesa/internal/sops"
	"github.com/felipemalacarne/mesa/internal/store"
)

// Target is where a connection's cluster is declared: the repository, the
// folder Mesa owns in it, and the CNPG Cluster.
type Target struct {
	Repo      string `json:"repo"`
	Branch    string `json:"branch"`
	Path      string `json:"path"`
	Cluster   string `json:"cluster"`
	Namespace string `json:"namespace"`
	// PullRequest opens a pull request instead of committing to Branch;
	// nothing is created until it is merged and synced.
	PullRequest bool `json:"pullRequest"`
}

// Service provisions databases.
type Service struct {
	sessions *session.Manager
	changes  *changes.Service
	audit    store.Audit
	targets  func(uuid.UUID) (Target, bool)
	repos    RepoFactory
	now      func() time.Time
}

// New wires the service. targets says which connections can provision and
// where; repos opens their repositories (nil when no Git access is
// configured).
func New(sessions *session.Manager, ch *changes.Service, audit store.Audit, targets func(uuid.UUID) (Target, bool), repos RepoFactory) *Service {
	return &Service{sessions: sessions, changes: ch, audit: audit, targets: targets, repos: repos, now: time.Now}
}

// Target returns where a connection provisions, if it can.
func (s *Service) Target(id uuid.UUID) (Target, error) {
	if s.targets != nil {
		if t, ok := s.targets(id); ok {
			return t, nil
		}
	}
	return Target{}, errs.New(errs.NotFound, "this connection has no GitOps repository to provision into")
}

// Input asks for an app's database.
type Input struct {
	ConnectionID uuid.UUID
	// Name is the database and its role.
	Name string
	// Namespace is where the app runs: it gets the connection Secret.
	Namespace    string
	Confirmation string
	DryRun       bool
	Actor        string
	Surface      string
}

// Result is what provisioning did, or (dry run) would do.
type Result struct {
	Target Target
	// Statements are the SQL Mesa runs now, the password hidden.
	Statements []string
	// Files are the paths written to the repository.
	Files    []string
	Decision policy.Decision
	// RoleSecret and AppSecret name the Secrets the repository declares.
	RoleSecret string
	AppSecret  string
	Commit     *Committed
	// Applied reports that the role and database exist now; otherwise CNPG
	// creates them once the commit is synced.
	Applied bool
	// Note says what happens next, or what went wrong after the commit.
	Note string
}

// Provision creates an app's database and its role, and commits their
// declaration. Without DryRun the commit comes first: once it lands, the
// cluster converges on its own even if running the SQL now fails.
func (s *Service) Provision(ctx context.Context, in Input) (*Result, error) {
	if err := account.Require(ctx, account.Manage); err != nil {
		return nil, err
	}
	t, err := s.Target(in.ConnectionID)
	if err != nil {
		return nil, err
	}
	if s.repos == nil {
		return nil, errs.New(errs.Unsupported, "Mesa has no access to Git (MESA_GITHUB_TOKEN)")
	}
	db := cnpg.Database{Name: strings.TrimSpace(in.Name), AppNamespace: strings.TrimSpace(in.Namespace)}
	if err := db.Validate(); err != nil {
		return nil, err
	}
	cluster := cnpg.Cluster{Dir: strings.Trim(t.Path, "/"), Name: t.Cluster, Namespace: t.Namespace}
	res := &Result{Target: t, RoleSecret: cnpg.RoleSecret(db.Name), AppSecret: cnpg.AppSecret(db.Name)}

	sess, _, err := s.sessions.Session(ctx, in.ConnectionID, "")
	if err != nil {
		return nil, err
	}
	if err := s.checkFree(ctx, sess, db.Name); err != nil {
		return nil, err
	}
	repo, err := s.repos(t.Repo, t.Branch)
	if err != nil {
		return nil, err
	}
	sopsConfig, err := repo.Read(ctx, ".sops.yaml")
	if err != nil {
		return nil, errs.Wrap(errs.KindOf(err), "cannot read .sops.yaml in "+t.Repo, err)
	}
	recipients, err := sops.Recipients(sopsConfig, cluster.SecretsPath(db.Name))
	if err != nil {
		return nil, errs.Wrap(errs.Invalid, "cannot tell whom to encrypt for", err)
	}
	if db.Password, err = newPassword(); err != nil {
		return nil, err
	}

	role := sess.CreateRolePlan(engine.RoleInput{Name: ident.Name(db.Name), Password: db.Password, CanLogin: true})
	database := sess.CreateDatabasePlan(ident.Name(db.Name), ident.Name(db.Name))
	for _, p := range []*changeset.Plan{role, database} {
		for _, st := range p.Statements {
			res.Statements = append(res.Statements, st.Display())
		}
	}
	// The policy judges what Mesa is about to do on the server: a
	// production connection must be unlocked and named.
	judged, err := s.changes.Apply(ctx, changes.ApplyInput{
		ConnectionID: in.ConnectionID, Plan: role, Confirmation: in.Confirmation,
		Transaction: true, DryRun: true, Actor: in.Actor, Surface: in.Surface,
	})
	if err != nil {
		return nil, err
	}
	res.Decision = judged.Decision

	names, err := s.names(ctx, repo, cluster, db.Name)
	if err != nil {
		return nil, err
	}
	files := cluster.Index(append(names, db.Name))
	files[cluster.DatabasePath(db.Name)] = cluster.DatabaseManifest(db)
	for p := range files {
		res.Files = append(res.Files, p)
	}
	res.Files = append(res.Files, cluster.SecretsPath(db.Name))
	sort.Strings(res.Files)
	if in.DryRun {
		return res, nil
	}
	if err := res.Decision.Err(); err != nil {
		return res, err
	}

	secrets, err := sops.Encrypt(cluster.Secrets(db), recipients, s.now())
	if err != nil {
		return nil, err
	}
	files[cluster.SecretsPath(db.Name)] = secrets
	commit := Commit{
		Message: fmt.Sprintf("%s: database %s for %s, from Mesa\n\nRole and database %s on %s/%s; Secret %s in %s.\nProvisioned by %s.",
			t.Cluster, db.Name, db.AppNamespace, db.Name, t.Namespace, t.Cluster, res.AppSecret, db.AppNamespace, actorOr(in.Actor)),
		Files: files,
	}
	if t.PullRequest {
		commit.PullRequest = &PullRequest{
			Branch: "mesa/" + strings.ReplaceAll(db.Name, "_", "-") + "-" + s.now().UTC().Format("20060102150405"),
			Title:  fmt.Sprintf("Database %s for %s on %s", db.Name, db.AppNamespace, t.Cluster),
			Body: fmt.Sprintf("Provisioned from Mesa by %s.\n\nOnce merged and synced, CNPG creates the role and database `%s` on `%s`, and namespace `%s` gets the Secret `%s` (`DATABASE_URL`, `uri`, `username`, `password`, `host`, `port`, `dbname`).",
				actorOr(in.Actor), db.Name, t.Cluster, db.AppNamespace, res.AppSecret),
		}
	}
	res.Commit, err = s.commit(ctx, repo, cluster, db.Name, commit)
	if err != nil {
		return nil, err
	}
	s.record(ctx, in, t, db.Name, res.Commit)

	if t.PullRequest {
		res.Note = "Merge the pull request; once it syncs, CNPG creates the role and database and the Secret appears."
		return res, nil
	}
	if _, err := s.changes.Apply(ctx, changes.ApplyInput{
		ConnectionID: in.ConnectionID, Plan: role, Confirmation: in.Confirmation,
		Transaction: true, Actor: in.Actor, Surface: in.Surface,
	}); err != nil {
		res.Note = "Committed, but creating the role now failed (" + errs.Message(err) + "); CNPG creates it when the commit syncs."
		return res, nil
	}
	if _, err := s.changes.Apply(ctx, changes.ApplyInput{
		ConnectionID: in.ConnectionID, Plan: database, Confirmation: in.Confirmation,
		Transaction: false, Actor: in.Actor, Surface: in.Surface,
	}); err != nil {
		res.Note = "Committed and the role exists, but creating the database now failed (" + errs.Message(err) + "); CNPG creates it when the commit syncs."
		return res, nil
	}
	res.Applied = true
	res.Note = "The database is ready. The Secret appears in the app's namespace once the commit syncs."
	return res, nil
}

// checkFree refuses a name the server already has, as a role or a database.
func (s *Service) checkFree(ctx context.Context, sess engine.Session, name string) error {
	roles, err := sess.Roles(ctx)
	if err != nil {
		return err
	}
	for _, r := range roles {
		if string(r.Name) == name {
			return errs.Newf(errs.Conflict, "the server already has a role %s", name)
		}
	}
	dbs, err := sess.Databases(ctx)
	if err != nil {
		return err
	}
	for _, d := range dbs {
		if string(d.Name) == name {
			return errs.Newf(errs.Conflict, "the server already has a database %s", name)
		}
	}
	return nil
}

// names lists the apps Mesa's folder declares, refusing name if it is one.
func (s *Service) names(ctx context.Context, repo Repo, cluster cnpg.Cluster, name string) ([]string, error) {
	entries, err := repo.List(ctx, cluster.DatabasesDir())
	if err != nil {
		return nil, err
	}
	names := cnpg.Names(entries)
	if slices.Contains(names, name) {
		return nil, errs.Newf(errs.Conflict, "%s already declares a database %s", cluster.Dir, name)
	}
	return names, nil
}

// commit writes the files, reading the folder again when the branch moved
// meanwhile (someone else's commit may have added an app).
func (s *Service) commit(ctx context.Context, repo Repo, cluster cnpg.Cluster, name string, c Commit) (*Committed, error) {
	for attempt := 0; ; attempt++ {
		done, err := repo.Commit(ctx, c)
		if err == nil || !errs.Is(err, errs.Conflict) || attempt == 2 {
			return done, err
		}
		names, err := s.names(ctx, repo, cluster, name)
		if err != nil {
			return nil, err
		}
		for p, b := range cluster.Index(append(names, name)) {
			c.Files[p] = b
		}
	}
}

// record puts the commit in the audit log next to the SQL.
func (s *Service) record(ctx context.Context, in Input, t Target, name string, c *Committed) {
	_, conn, err := s.sessions.Session(ctx, in.ConnectionID, "")
	if err != nil {
		return
	}
	where := c.URL
	if c.PullRequestURL != "" {
		where = c.PullRequestURL
	}
	now := s.now()
	e := store.AuditEntry{
		ID: uuid.New(), ConnectionID: in.ConnectionID, Database: conn.Database, Environment: conn.Environment,
		Actor: in.Actor, Surface: in.Surface,
		Summary:   fmt.Sprintf("Declare database %s in %s", name, t.Repo),
		SQL:       fmt.Sprintf("-- %s\n-- %s@%s, %s", where, t.Repo, t.Branch, t.Path),
		Status:    store.AuditApplied,
		StartedAt: now, FinishedAt: &now,
	}
	_ = s.audit.Begin(ctx, e)
}

func newPassword() (string, error) {
	b := make([]byte, 24)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func actorOr(a string) string {
	if a == "" {
		return "Mesa"
	}
	return a
}
