// Package cnpg writes the manifests Mesa keeps in a GitOps repository for
// the databases it provisions on a CloudNativePG cluster: a Kustomize
// component, owned by Mesa, inside the folder that declares the cluster.
//
//	<dir>/
//	  kustomization.yaml         the component (Mesa rewrites it)
//	  roles.yaml                 the managed roles, patched into the Cluster (Mesa rewrites it)
//	  ksops.yaml                 the KSOPS generator for secrets/ (Mesa rewrites it)
//	  databases/<name>.yaml      one CNPG Database per app (written once)
//	  secrets/<name>-secrets.yaml  the role's password and the app's connection, SOPS-encrypted (written once)
//
// The parent kustomization includes the component once (components: [mesa])
// and must leave explicit namespaces alone, because each app's connection
// Secret lives in the app's namespace.
package cnpg

import (
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/felipemalacarne/mesa/internal/core/errs"
)

// Cluster is a CloudNativePG cluster and Mesa's folder for it.
type Cluster struct {
	// Dir is Mesa's folder in the repository.
	Dir string
	// Name and Namespace name the CNPG Cluster.
	Name      string
	Namespace string
}

// Database is an app's database, the role that owns it, and where the app
// runs.
type Database struct {
	// Name is the database and the role.
	Name string
	// AppNamespace receives the connection Secret <name>-db.
	AppNamespace string
	Password     string
}

var (
	namePattern      = regexp.MustCompile(`^[a-z][a-z0-9_]{0,39}$`)
	namespacePattern = regexp.MustCompile(`^[a-z0-9]([-a-z0-9]{0,61}[a-z0-9])?$`)
)

// Validate checks names Postgres and Kubernetes both accept.
func (d Database) Validate() error {
	if !namePattern.MatchString(d.Name) {
		return errs.Newf(errs.Invalid, "%q: use lowercase letters, digits and _, starting with a letter (up to 40)", d.Name)
	}
	if !namespacePattern.MatchString(d.AppNamespace) {
		return errs.Newf(errs.Invalid, "%q is not a Kubernetes namespace name", d.AppNamespace)
	}
	return nil
}

// kube is the name as a Kubernetes object name (no underscores there).
func kube(name string) string { return strings.ReplaceAll(name, "_", "-") }

// RoleSecret is the Secret, in the cluster's namespace, that holds the
// role's password for CNPG.
func RoleSecret(name string) string { return kube(name) + "-db-secret" }

// AppSecret is the Secret, in the app's namespace, with the connection.
func AppSecret(name string) string { return kube(name) + "-db" }

// Host is the cluster's read-write service, as apps in the cluster reach it.
func (c Cluster) Host() string { return c.Name + "-rw." + c.Namespace + ".svc" }

// DatabasePath and SecretsPath are where an app's files go.
func (c Cluster) DatabasePath(name string) string { return c.Dir + "/databases/" + name + ".yaml" }
func (c Cluster) SecretsPath(name string) string {
	return c.Dir + "/secrets/" + name + "-secrets.yaml"
}

// DatabasesDir is listed to know which apps the folder already has.
func (c Cluster) DatabasesDir() string { return c.Dir + "/databases" }

// Names reads app names from the entries of DatabasesDir.
func Names(entries []string) []string {
	var out []string
	for _, e := range entries {
		if n, ok := strings.CutSuffix(e, ".yaml"); ok && namePattern.MatchString(n) {
			out = append(out, n)
		}
	}
	sort.Strings(out)
	return out
}

// DatabaseManifest is the CNPG Database of an app.
func (c Cluster) DatabaseManifest(d Database) []byte {
	return []byte(fmt.Sprintf(`# Provisioned by Mesa. The role %[1]s is in ../roles.yaml; its password, and the
# app's connection (Secret %[3]s in namespace %[4]s), in ../secrets/%[1]s-secrets.yaml.
apiVersion: postgresql.cnpg.io/v1
kind: Database
metadata:
  name: %[5]s-%[2]s
  namespace: %[6]s
spec:
  cluster:
    name: %[5]s
  name: %[1]s
  owner: %[1]s
  # Deleting this file stops managing the database; it keeps its data.
  databaseReclaimPolicy: retain
`, d.Name, kube(d.Name), AppSecret(d.Name), d.AppNamespace, c.Name, c.Namespace))
}

// Secrets is the plaintext of an app's secrets, for sops to encrypt: the
// role's password where CNPG reads it, and the app's connection where the
// app runs.
func (c Cluster) Secrets(d Database) []byte {
	uri := fmt.Sprintf("postgres://%s:%s@%s:5432/%s?sslmode=require", d.Name, d.Password, c.Host(), d.Name)
	q := strconv.Quote
	return []byte(fmt.Sprintf(`apiVersion: v1
kind: Secret
metadata:
  name: %s
  namespace: %s
  labels:
    cnpg.io/reload: "true"
type: kubernetes.io/basic-auth
stringData:
  username: %s
  password: %s
---
apiVersion: v1
kind: Secret
metadata:
  name: %s
  namespace: %s
type: Opaque
stringData:
  username: %s
  password: %s
  host: %s
  port: "5432"
  dbname: %s
  uri: %s
  DATABASE_URL: %s
`,
		q(RoleSecret(d.Name)), q(c.Namespace), q(d.Name), q(d.Password),
		q(AppSecret(d.Name)), q(d.AppNamespace), q(d.Name), q(d.Password), q(c.Host()), q(d.Name), q(uri), q(uri)))
}

// Index renders the files Mesa rewrites from the list of apps: the
// component, the roles patch and the KSOPS generator.
func (c Cluster) Index(names []string) map[string][]byte {
	names = append([]string(nil), names...)
	sort.Strings(names)
	var k strings.Builder
	// kustomize refuses a kustomization with nothing but apiVersion and
	// kind, so the name keeps the folder valid before its first database.
	k.WriteString(`# Databases Mesa provisioned on ` + c.Name + `. Mesa rewrites this file, roles.yaml and ksops.yaml
# from databases/; the files in databases/ and secrets/ are written once.
apiVersion: kustomize.config.k8s.io/v1alpha1
kind: Component
metadata:
  name: mesa
`)
	files := map[string][]byte{}
	if len(names) == 0 {
		files[c.Dir+"/kustomization.yaml"] = []byte(k.String())
		return files
	}
	k.WriteString("\nresources:\n")
	for _, n := range names {
		k.WriteString("  - databases/" + n + ".yaml\n")
	}
	k.WriteString("\ngenerators:\n  - ksops.yaml\n")
	k.WriteString(`
patches:
  - path: roles.yaml
    target:
      group: postgresql.cnpg.io
      kind: Cluster
      name: ` + c.Name + "\n")
	files[c.Dir+"/kustomization.yaml"] = []byte(k.String())

	var r strings.Builder
	r.WriteString("# The roles of the databases Mesa provisioned, added to " + c.Name + "'s managed roles.\n")
	for _, n := range names {
		fmt.Fprintf(&r, `- op: add
  path: /spec/managed/roles/-
  value:
    name: %s
    ensure: present
    login: true
    passwordSecret:
      name: %s
`, n, RoleSecret(n))
	}
	files[c.Dir+"/roles.yaml"] = []byte(r.String())

	var s strings.Builder
	s.WriteString(`apiVersion: viaduct.ai/v1
kind: ksops
metadata:
  name: mesa-secrets
  annotations:
    config.kubernetes.io/function: |
      exec:
        path: ksops
files:
`)
	for _, n := range names {
		s.WriteString("  - secrets/" + n + "-secrets.yaml\n")
	}
	files[c.Dir+"/ksops.yaml"] = []byte(s.String())
	return files
}
