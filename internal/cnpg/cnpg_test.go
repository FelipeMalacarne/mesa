package cnpg

import (
	"strings"
	"testing"

	"go.yaml.in/yaml/v3"

	"github.com/felipemalacarne/mesa/internal/core/errs"
)

var cluster = Cluster{Dir: "clusters/saradomin/apps/postgres/mesa", Name: "postgres-main", Namespace: "databases"}

func TestValidate(t *testing.T) {
	for _, ok := range []Database{{Name: "shop", AppNamespace: "shop"}, {Name: "shop_api2", AppNamespace: "my-apps"}} {
		if err := ok.Validate(); err != nil {
			t.Errorf("%+v: %v", ok, err)
		}
	}
	for _, bad := range []Database{
		{Name: "Shop", AppNamespace: "shop"}, {Name: "2shop", AppNamespace: "shop"}, {Name: "shop-api", AppNamespace: "shop"},
		{Name: "shop", AppNamespace: "Shop"}, {Name: "shop", AppNamespace: "-shop"}, {Name: "shop", AppNamespace: ""},
	} {
		if err := bad.Validate(); !errs.Is(err, errs.Invalid) {
			t.Errorf("%+v: %v", bad, err)
		}
	}
	if RoleSecret("shop_api") != "shop-api-db-secret" || AppSecret("shop_api") != "shop-api-db" {
		t.Error("Kubernetes names have no underscores")
	}
}

func TestNames(t *testing.T) {
	got := Names([]string{"shop.yaml", "README.md", "blog.yaml", "Bad.yaml", ".keep"})
	if strings.Join(got, ",") != "blog,shop" {
		t.Errorf("Names = %v", got)
	}
}

func TestManifests(t *testing.T) {
	d := Database{Name: "shop", AppNamespace: "shop", Password: "p4ss"}
	var db map[string]any
	if err := yaml.Unmarshal(cluster.DatabaseManifest(d), &db); err != nil {
		t.Fatal(err)
	}
	spec := db["spec"].(map[string]any)
	if db["kind"] != "Database" || spec["owner"] != "shop" || spec["name"] != "shop" || spec["databaseReclaimPolicy"] != "retain" {
		t.Errorf("Database = %v", db)
	}

	dec := yaml.NewDecoder(strings.NewReader(string(cluster.Secrets(d))))
	var role, conn struct {
		Metadata struct {
			Name      string            `yaml:"name"`
			Namespace string            `yaml:"namespace"`
			Labels    map[string]string `yaml:"labels"`
		} `yaml:"metadata"`
		Type       string            `yaml:"type"`
		StringData map[string]string `yaml:"stringData"`
	}
	if err := dec.Decode(&role); err != nil {
		t.Fatal(err)
	}
	if err := dec.Decode(&conn); err != nil {
		t.Fatal(err)
	}
	if role.Metadata.Namespace != "databases" || role.Type != "kubernetes.io/basic-auth" || role.StringData["password"] != "p4ss" || role.Metadata.Labels["cnpg.io/reload"] != "true" {
		t.Errorf("role secret = %+v", role)
	}
	want := "postgres://shop:p4ss@postgres-main-rw.databases.svc:5432/shop?sslmode=require"
	if conn.Metadata.Name != "shop-db" || conn.Metadata.Namespace != "shop" || conn.StringData["uri"] != want || conn.StringData["DATABASE_URL"] != want || conn.StringData["port"] != "5432" {
		t.Errorf("app secret = %+v", conn)
	}
}

func TestIndex(t *testing.T) {
	empty := cluster.Index(nil)
	if len(empty) != 1 || strings.Contains(string(empty[cluster.Dir+"/kustomization.yaml"]), "resources") {
		t.Errorf("an empty folder is a bare component: %v", empty)
	}
	files := cluster.Index([]string{"shop", "blog"})
	k := string(files[cluster.Dir+"/kustomization.yaml"])
	if !strings.Contains(k, "kind: Component") || strings.Index(k, "databases/blog.yaml") > strings.Index(k, "databases/shop.yaml") {
		t.Errorf("kustomization:\n%s", k)
	}
	var ops []struct {
		Op    string `yaml:"op"`
		Path  string `yaml:"path"`
		Value struct {
			Name           string `yaml:"name"`
			PasswordSecret struct {
				Name string `yaml:"name"`
			} `yaml:"passwordSecret"`
		} `yaml:"value"`
	}
	if err := yaml.Unmarshal(files[cluster.Dir+"/roles.yaml"], &ops); err != nil {
		t.Fatal(err)
	}
	if len(ops) != 2 || ops[0].Path != "/spec/managed/roles/-" || ops[1].Value.PasswordSecret.Name != "shop-db-secret" {
		t.Errorf("roles = %+v", ops)
	}
	if !strings.Contains(string(files[cluster.Dir+"/ksops.yaml"]), "secrets/shop-secrets.yaml") {
		t.Errorf("ksops:\n%s", files[cluster.Dir+"/ksops.yaml"])
	}
}
