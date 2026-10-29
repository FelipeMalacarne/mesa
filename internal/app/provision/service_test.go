package provision_test

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"filippo.io/age"
	"github.com/jackc/pgx/v5"

	"github.com/felipemalacarne/mesa/internal/app"
	"github.com/felipemalacarne/mesa/internal/app/provision"
	"github.com/felipemalacarne/mesa/internal/core/account"
	"github.com/felipemalacarne/mesa/internal/core/errs"
	"github.com/felipemalacarne/mesa/internal/engine"
	"github.com/felipemalacarne/mesa/internal/engine/postgres"
	"github.com/felipemalacarne/mesa/internal/secret"
	"github.com/felipemalacarne/mesa/internal/session"
	"github.com/felipemalacarne/mesa/internal/store/declared"
	"github.com/felipemalacarne/mesa/internal/store/sqlite"
)

// repo is a repository in memory.
type repo struct {
	files   map[string]string
	commits []provision.Commit
}

func (r *repo) Read(_ context.Context, p string) ([]byte, error) {
	if b, ok := r.files[p]; ok {
		return []byte(b), nil
	}
	return nil, errs.New(errs.NotFound, "no "+p)
}

func (r *repo) List(_ context.Context, dir string) ([]string, error) {
	var out []string
	for p := range r.files {
		if rest, ok := strings.CutPrefix(p, dir+"/"); ok && !strings.Contains(rest, "/") {
			out = append(out, rest)
		}
	}
	sort.Strings(out)
	return out, nil
}

func (r *repo) Commit(_ context.Context, c provision.Commit) (*provision.Committed, error) {
	r.commits = append(r.commits, c)
	if c.PullRequest == nil {
		for p, b := range c.Files {
			r.files[p] = string(b)
		}
	}
	return &provision.Committed{SHA: fmt.Sprint(len(r.commits)), URL: "https://example.com/c", PullRequestURL: map[bool]string{true: "https://example.com/pr"}[c.PullRequest != nil]}, nil
}

const dir = "clusters/test/apps/postgres/mesa"

// setup builds Mesa with one declared connection to MESA_TEST_PG that
// provisions into an in-memory repository.
func setup(t *testing.T, env string, pullRequest bool) (*app.App, *repo, *age.X25519Identity, string) {
	t.Helper()
	dsn := os.Getenv("MESA_TEST_PG")
	if dsn == "" {
		t.Skip("MESA_TEST_PG is not set")
	}
	u, err := url.Parse(dsn)
	if err != nil {
		t.Fatal(err)
	}
	id, err := age.GenerateX25519Identity()
	if err != nil {
		t.Fatal(err)
	}
	r := &repo{files: map[string]string{
		".sops.yaml": "creation_rules:\n  - path_regex: .*secrets?(\\.dec)?\\.yaml$\n    age: " + id.Recipient().String() + "\n",
	}}
	pass, _ := u.User.Password()
	t.Setenv("MESA_TEST_PROVISION_PASSWORD", pass)
	conf := filepath.Join(t.TempDir(), "connections.toml")
	body := fmt.Sprintf(`[connections.test]
environment = %q
host = %q
port = %s
user = %q
password = "env:MESA_TEST_PROVISION_PASSWORD"
tls = "disable"

[connections.test.gitops]
repo = "felipe/gielinor"
path = %q
cluster = "postgres-main"
namespace = "databases"
pull_request = %v
`, env, u.Hostname(), u.Port(), u.User.Username(), dir, pullRequest)
	if err := os.WriteFile(conf, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	decl, err := declared.Load(conf)
	if err != nil {
		t.Fatal(err)
	}
	st, err := sqlite.Open(context.Background(), sqlite.Memory)
	if err != nil {
		t.Fatal(err)
	}
	vault, _ := secret.NewLocal("k", st.Secrets())
	a := app.New(app.Options{
		Mode: app.ModeServer, Store: st, Vault: vault, Declared: decl,
		Engines: []engine.Engine{postgres.New()},
		Session: session.Options{AppName: "mesa-test", IdleTimeout: time.Minute},
		Repos:   func(string, string) (provision.Repo, error) { return r, nil },
	})
	t.Cleanup(func() { _ = a.Close() })
	return a, r, id, dsn
}

func freshName(t *testing.T) string {
	t.Helper()
	return fmt.Sprintf("app_%d", time.Now().UnixNano()%1_000_000_000)
}

// drop removes what a test provisioned on the shared server.
func drop(t *testing.T, dsn, name string) {
	t.Cleanup(func() {
		c, err := pgx.Connect(context.Background(), dsn)
		if err != nil {
			return
		}
		defer func() { _ = c.Close(context.Background()) }()
		_, _ = c.Exec(context.Background(), "DROP DATABASE IF EXISTS "+name)
		_, _ = c.Exec(context.Background(), "DROP ROLE IF EXISTS "+name)
	})
}

func TestProvision(t *testing.T) {
	a, r, id, dsn := setup(t, "development", false)
	ctx := account.With(context.Background(), account.Local())
	conn := declared.ConnectionID("test")
	name := freshName(t)
	drop(t, dsn, name)

	if tg, err := a.Provision.Target(conn); err != nil || tg.Cluster != "postgres-main" {
		t.Fatalf("Target = %+v %v", tg, err)
	}
	in := provision.Input{ConnectionID: conn, Name: name, Namespace: "shop", Actor: "tester", Surface: "test"}

	// A dry run says what would happen and touches nothing.
	dry := in
	dry.DryRun = true
	plan, err := a.Provision.Provision(ctx, dry)
	if err != nil {
		t.Fatalf("dry run: %v", err)
	}
	if len(r.commits) != 0 || len(plan.Statements) != 2 || !strings.Contains(plan.Statements[0], "'********'") {
		t.Fatalf("dry run = %+v", plan)
	}
	if plan.AppSecret != strings.ReplaceAll(name, "_", "-")+"-db" || len(plan.Files) != 5 {
		t.Fatalf("dry run files = %v, secret %s", plan.Files, plan.AppSecret)
	}

	res, err := a.Provision.Provision(ctx, in)
	if err != nil {
		t.Fatalf("Provision: %v", err)
	}
	if !res.Applied || res.Commit == nil || len(r.commits) != 1 {
		t.Fatalf("result = %+v", res)
	}
	for _, want := range []string{
		dir + "/kustomization.yaml", dir + "/roles.yaml", dir + "/ksops.yaml",
		dir + "/databases/" + name + ".yaml", dir + "/secrets/" + name + "-secrets.yaml",
	} {
		if _, ok := r.files[want]; !ok {
			t.Errorf("no %s in the commit", want)
		}
	}
	if !strings.Contains(r.files[dir+"/roles.yaml"], "name: "+name) || !strings.Contains(r.files[dir+"/kustomization.yaml"], "databases/"+name+".yaml") {
		t.Errorf("index:\n%s\n%s", r.files[dir+"/kustomization.yaml"], r.files[dir+"/roles.yaml"])
	}
	enc := r.files[dir+"/secrets/"+name+"-secrets.yaml"]
	if !strings.Contains(enc, "ENC[AES256_GCM") || strings.Contains(enc, name) {
		t.Fatalf("the secrets are not encrypted:\n%s", enc)
	}

	// The committed password is the role's: sign in with it, not with
	// another (unless the test server trusts everyone, as CI's does).
	if bin := sopsBinary(); bin != "" {
		u, err := url.Parse(decryptURI(t, bin, id, enc))
		if err != nil {
			t.Fatal(err)
		}
		u.Host = hostPort(dsn)
		u.RawQuery = "sslmode=disable"
		c, err := pgx.Connect(ctx, u.String())
		if err != nil {
			t.Fatalf("connect as the new role: %v", err)
		}
		var db string
		_ = c.QueryRow(ctx, "SELECT current_database()").Scan(&db)
		_ = c.Close(ctx)
		if db != name {
			t.Fatalf("connected to %q", db)
		}
		u.User = url.UserPassword(name, "wrong")
		if c, err := pgx.Connect(ctx, u.String()); err == nil {
			_ = c.Close(ctx)
			t.Log("the test server accepts any password, so the password itself is unchecked here")
		}
	}

	// The audit log has the SQL and the commit.
	entries, err := a.Changes.Audit(ctx, conn, 10)
	if err != nil || len(entries) != 3 {
		t.Fatalf("audit = %d entries, %v", len(entries), err)
	}
	for _, e := range entries {
		if strings.Contains(e.SQL, "PASSWORD '") && !strings.Contains(e.SQL, "'********'") {
			t.Errorf("the audit log shows the password: %s", e.SQL)
		}
	}

	// Again: the name is taken.
	if _, err := a.Provision.Provision(ctx, in); !errs.Is(err, errs.Conflict) {
		t.Fatalf("second time: %v", err)
	}
	// A bad name or namespace never reaches the server.
	for _, bad := range []provision.Input{
		{ConnectionID: conn, Name: "Shop", Namespace: "shop"},
		{ConnectionID: conn, Name: "shop", Namespace: "Shop_NS"},
	} {
		if _, err := a.Provision.Provision(ctx, bad); !errs.Is(err, errs.Invalid) {
			t.Errorf("%+v: %v", bad, err)
		}
	}
}

func TestProvisionPullRequestAndPolicy(t *testing.T) {
	ctx := account.With(context.Background(), account.Local())
	conn := declared.ConnectionID("test")

	a, r, _, dsn := setup(t, "development", true)
	name := freshName(t)
	drop(t, dsn, name)
	res, err := a.Provision.Provision(ctx, provision.Input{ConnectionID: conn, Name: name, Namespace: "shop"})
	if err != nil {
		t.Fatal(err)
	}
	if res.Applied || res.Commit.PullRequestURL == "" || r.commits[0].PullRequest == nil {
		t.Fatalf("pull request mode = %+v", res)
	}
	c, _ := pgx.Connect(ctx, dsn)
	defer func() { _ = c.Close(ctx) }()
	var n int
	_ = c.QueryRow(ctx, "SELECT count(*) FROM pg_roles WHERE rolname = $1", name).Scan(&n)
	if n != 0 {
		t.Fatal("pull request mode created the role before the merge")
	}

	// Production is read-only until unlocked, and then needs the name typed.
	prod, _, _, _ := setup(t, "production", false)
	in := provision.Input{ConnectionID: conn, Name: freshName(t), Namespace: "shop"}
	if _, err := prod.Provision.Provision(ctx, in); !errs.Is(err, errs.Forbidden) {
		t.Fatalf("locked production: %v", err)
	}
	if _, err := prod.Connections.Unlock(ctx, conn, time.Minute); err != nil {
		t.Fatal(err)
	}
	if _, err := prod.Provision.Provision(ctx, in); !errs.Is(err, errs.NeedsConfirmation) {
		t.Fatalf("unconfirmed production: %v", err)
	}

	// Only admins provision.
	viewer, _ := account.New(account.Input{Subject: "s", Email: "v@example.com", Role: account.Editor}, time.Now())
	if _, err := prod.Provision.Provision(account.With(context.Background(), account.Of(viewer)), in); !errs.Is(err, errs.Forbidden) {
		t.Fatalf("an editor provisions: %v", err)
	}
}

func sopsBinary() string {
	if p := os.Getenv("MESA_TEST_SOPS"); p != "" {
		return p
	}
	if p, err := exec.LookPath("sops"); err == nil {
		return p
	}
	return ""
}

func decryptURI(t *testing.T, bin string, id *age.X25519Identity, enc string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "s-secrets.yaml")
	if err := os.WriteFile(p, []byte(enc), 0o600); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(bin, "decrypt", p)
	cmd.Env = append(os.Environ(), "SOPS_AGE_KEY="+id.String())
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("sops: %v\n%s", err, out)
	}
	for _, line := range strings.Split(string(out), "\n") {
		if v, ok := strings.CutPrefix(strings.TrimSpace(line), "uri: "); ok {
			return strings.Trim(v, `"`)
		}
	}
	t.Fatalf("no uri in\n%s", out)
	return ""
}

func hostPort(dsn string) string {
	u, _ := url.Parse(dsn)
	return u.Host
}
