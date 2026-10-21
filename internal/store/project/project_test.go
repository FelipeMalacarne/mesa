package project

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/felipemalacarne/mesa/internal/core/connection"
	"github.com/felipemalacarne/mesa/internal/core/errs"
	"github.com/felipemalacarne/mesa/internal/store"
	"github.com/felipemalacarne/mesa/internal/store/sqlite"
)

const sample = `[project]
name = "billing"
default = "staging"

[connections.staging]
driver = "postgres"
host = "10.0.4.12"
database = "billing"
user = "mesa_app"
password = "env:BILLING_STAGING_PASSWORD"

[connections.production]
host = "10.0.9.3"
password = "keychain:mesa/billing-prod"
read_only = true
`

func write(t *testing.T, root, body string) {
	t.Helper()
	dir := filepath.Join(root, DirName)
	if err := os.MkdirAll(filepath.Join(dir, "queries"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ConfigName), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestFindAndInherit(t *testing.T) {
	root := t.TempDir()
	write(t, root, sample)
	nested := filepath.Join(root, "src", "app")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatal(err)
	}
	p, err := Find(nested)
	if err != nil || p == nil {
		t.Fatalf("Find: %v %v", p, err)
	}
	if p.Name != "billing" || p.Default != "staging" {
		t.Fatalf("project = %+v", p)
	}
	if got := p.Envs(); len(got) != 2 || got[0] != "staging" || got[1] != "production" {
		t.Errorf("Envs = %v", got)
	}
	prod, err := p.Connection("production")
	if err != nil {
		t.Fatal(err)
	}
	if prod.Database != "billing" || prod.User != "mesa_app" || prod.Host != "10.0.9.3" || prod.Port != 5432 || !prod.ReadOnly || prod.Environment != connection.Production {
		t.Errorf("production inherits from staging: %+v", prod)
	}
	if ref, _ := p.Secret(prod.ID); ref != "keychain:mesa/billing-prod" {
		t.Errorf("secret ref = %q", ref)
	}
	if prod.ID != ConnectionID("billing", "production") {
		t.Error("IDs must be stable")
	}
}

func TestFindNone(t *testing.T) {
	p, err := Find(t.TempDir())
	if err != nil || p != nil {
		t.Fatalf("no project: %v %v", p, err)
	}
}

func TestRejectsLiteralPasswords(t *testing.T) {
	root := t.TempDir()
	write(t, root, "[project]\nname = \"x\"\n[connections.local]\nhost = \"h\"\nuser = \"u\"\npassword = \"hunter2\"\n")
	if _, err := Find(root); !errs.Is(err, errs.Invalid) {
		t.Fatalf("literal password: %v", err)
	}
}

func TestInitRoundTrip(t *testing.T) {
	root := t.TempDir()
	p, err := Init(root, "shop", "development", map[string]Conn{"development": {Host: "localhost", Database: "shop", User: "app", Password: "env:SHOP_PASSWORD"}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(p.MigrationsDir()); err != nil {
		t.Error(err)
	}
	if _, err := Init(root, "shop", "development", nil); !errs.Is(err, errs.Conflict) {
		t.Errorf("second Init: %v", err)
	}
	c, err := p.Connection("development")
	if err != nil || c.Environment != connection.Development || c.Name != "shop" {
		t.Fatalf("%+v %v", c, err)
	}
}

func TestOverlay(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	write(t, root, sample)
	if err := os.WriteFile(filepath.Join(root, DirName, "queries", "mrr.sql"), []byte("select 1"), 0o644); err != nil {
		t.Fatal(err)
	}
	p, err := Find(root)
	if err != nil {
		t.Fatal(err)
	}
	base, err := sqlite.Open(ctx, ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = base.Close() }()
	st := Overlay(base, p)

	list, err := st.Connections().List(ctx)
	if err != nil || len(list) != 2 {
		t.Fatalf("connections: %d %v", len(list), err)
	}
	id := ConnectionID("billing", "staging")
	if _, err := st.Connections().Get(ctx, id); err != nil {
		t.Errorf("Get project connection: %v", err)
	}
	if err := st.Connections().Delete(ctx, id); !errs.Is(err, errs.Forbidden) {
		t.Errorf("Delete project connection: %v", err)
	}

	if _, err := st.Layouts().Get(ctx, id, "billing"); !errs.Is(err, errs.NotFound) {
		t.Errorf("missing layout: %v", err)
	}
	if err := st.Layouts().Put(ctx, id, "billing", map[string]store.Position{"public.a": {X: 1, Y: 2}}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(p.LayoutPath("billing")); err != nil {
		t.Errorf("layout file: %v", err)
	}
	got, err := st.Layouts().Get(ctx, id, "billing")
	if err != nil || got["public.a"].Y != 2 {
		t.Errorf("layout round trip: %v %v", got, err)
	}

	qs, err := st.Queries().List(ctx)
	if err != nil || len(qs) != 1 || qs[0].Name != "mrr" {
		t.Fatalf("queries: %+v %v", qs, err)
	}
	qs[0].SQL = "select 2"
	if err := st.Queries().Put(ctx, qs[0]); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(filepath.Join(root, DirName, "queries", "mrr.sql")); string(b) != "select 2" {
		t.Errorf("query file = %q", b)
	}

	t.Setenv("BILLING_STAGING_PASSWORD", "s3cret")
	v := Vault(nil, p)
	if s, err := v.Get(ctx, id); err != nil || s != "s3cret" {
		t.Errorf("vault: %q %v", s, err)
	}
}
