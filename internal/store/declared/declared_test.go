package declared

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/felipemalacarne/mesa/internal/core/connection"
	"github.com/felipemalacarne/mesa/internal/core/errs"
	"github.com/felipemalacarne/mesa/internal/secret"
	"github.com/felipemalacarne/mesa/internal/store/sqlite"
)

const sample = `[connections.saradomin]
environment = "production"
host = "postgres-main-rw.databases.svc"
user = "mesa"
password = "env:MESA_TEST_SARADOMIN_PASSWORD"
tls = "require"

[connections.reports]
engine = "mysql"
environment = "staging"
host = "mysql.reports.svc"
user = "reader"
read_only = true
`

func write(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "connections.toml")
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestLoad(t *testing.T) {
	s, err := Load(write(t, sample))
	if err != nil {
		t.Fatal(err)
	}
	list := s.Connections()
	if len(list) != 2 || list[0].Name != "reports" || list[1].Name != "saradomin" {
		t.Fatalf("connections = %+v", list)
	}
	reports, sara := list[0], list[1]
	if reports.Engine != connection.MySQL || reports.Port != 3306 || reports.Database != "mysql" || !reports.ReadOnly {
		t.Errorf("reports takes the engine's defaults: %+v", reports)
	}
	if sara.Engine != connection.Postgres || sara.Port != 5432 || sara.Database != "postgres" || sara.TLS != connection.TLSRequire || !sara.ReadOnly {
		t.Errorf("saradomin: %+v", sara)
	}
	if sara.ID != ConnectionID("saradomin") {
		t.Error("IDs must be stable")
	}
	if ref, ok := s.Secret(sara.ID); !ok || ref != "env:MESA_TEST_SARADOMIN_PASSWORD" {
		t.Errorf("secret ref = %q %v", ref, ok)
	}
	if ref, ok := s.Secret(reports.ID); !ok || ref != "" {
		t.Errorf("no password: %q %v", ref, ok)
	}
}

func TestLoadRejects(t *testing.T) {
	for name, body := range map[string]string{
		"literal password": "[connections.x]\nhost = \"h\"\nuser = \"u\"\npassword = \"hunter2\"\n",
		"missing host":     "[connections.x]\nuser = \"u\"\n",
		"unknown engine":   "[connections.x]\nengine = \"oracle\"\nhost = \"h\"\nuser = \"u\"\n",
		"unknown key":      "[connections.x]\nhost = \"h\"\nuser = \"u\"\ndriver = \"postgres\"\n",
		"not toml":         "connections = [",
	} {
		if _, err := Load(write(t, body)); !errs.Is(err, errs.Invalid) {
			t.Errorf("%s: %v, want Invalid", name, err)
		}
	}
	if _, err := Load(filepath.Join(t.TempDir(), "missing.toml")); !errs.Is(err, errs.Invalid) {
		t.Errorf("missing file: %v", err)
	}
}

func TestOverlayAndVault(t *testing.T) {
	ctx := context.Background()
	s, err := Load(write(t, sample))
	if err != nil {
		t.Fatal(err)
	}
	base, err := sqlite.Open(ctx, ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = base.Close() }()
	st := Overlay(base, s)

	own, err := connection.New(connection.Input{Name: "mine", Engine: "postgres", Host: "localhost", User: "me"}, s.conns[0].CreatedAt)
	if err != nil {
		t.Fatal(err)
	}
	if err := st.Connections().Put(ctx, own); err != nil {
		t.Fatalf("own connections are still stored: %v", err)
	}
	list, err := st.Connections().List(ctx)
	if err != nil || len(list) != 3 {
		t.Fatalf("List: %d %v", len(list), err)
	}

	id := ConnectionID("saradomin")
	got, err := st.Connections().Get(ctx, id)
	if err != nil || got.Host != "postgres-main-rw.databases.svc" {
		t.Fatalf("Get declared: %+v %v", got, err)
	}
	if err := st.Connections().Put(ctx, *got); !errs.Is(err, errs.Forbidden) {
		t.Errorf("Put declared: %v", err)
	}
	if err := st.Connections().Delete(ctx, id); !errs.Is(err, errs.Forbidden) {
		t.Errorf("Delete declared: %v", err)
	}

	local, err := secret.NewLocal("test-key", base.Secrets())
	if err != nil {
		t.Fatal(err)
	}
	v := Vault(local, s)
	t.Setenv("MESA_TEST_SARADOMIN_PASSWORD", "s3cret")
	if pw, err := v.Get(ctx, id); err != nil || pw != "s3cret" {
		t.Errorf("vault resolves the reference: %q %v", pw, err)
	}
	if err := v.Set(ctx, id, "other"); !errs.Is(err, errs.Forbidden) {
		t.Errorf("vault Set on declared: %v", err)
	}
	if err := v.Set(ctx, own.ID, "mine"); err != nil {
		t.Fatal(err)
	}
	if pw, err := v.Get(ctx, own.ID); err != nil || pw != "mine" {
		t.Errorf("own secret: %q %v", pw, err)
	}
}

func TestNilIsBase(t *testing.T) {
	ctx := context.Background()
	base, err := sqlite.Open(ctx, ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = base.Close() }()
	if Overlay(base, nil) != base {
		t.Error("Overlay(nil) must return base")
	}
}
