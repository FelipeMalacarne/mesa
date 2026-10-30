package sops

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/sha512"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"testing"
	"time"

	"filippo.io/age"
	"filippo.io/age/armor"
	"go.yaml.in/yaml/v3"
)

const plain = `apiVersion: v1
kind: Secret
metadata:
  name: shop-db-secret
  namespace: databases
  labels:
    cnpg.io/reload: "true"
type: kubernetes.io/basic-auth
stringData:
  username: shop
  password: s3cr3t-p4ss
  note_unencrypted: readable
  empty: ""
---
apiVersion: v1
kind: Secret
metadata:
  name: shop-db
  namespace: shop # a comment that must not leak
stringData:
  uri: postgres://shop:s3cr3t-p4ss@postgres-main-rw.databases.svc:5432/shop?sslmode=require
  port: 5432
  ready: true
  tags: [a, b]
`

var now = time.Date(2026, time.October, 29, 18, 0, 0, 0, time.UTC)

func TestEncryptThenDecrypt(t *testing.T) {
	id, err := age.GenerateX25519Identity()
	if err != nil {
		t.Fatal(err)
	}
	other, _ := age.GenerateX25519Identity()
	out, err := Encrypt([]byte(plain), []string{id.Recipient().String(), other.Recipient().String()}, now)
	if err != nil {
		t.Fatal(err)
	}
	for _, leak := range []string{"s3cr3t", "shop-db", "a comment", "databases"} {
		if bytes.Contains(out, []byte(leak)) {
			t.Errorf("%q is readable in the output:\n%s", leak, out)
		}
	}
	if !bytes.Contains(out, []byte("readable")) {
		t.Error("a key ending with _unencrypted must stay readable")
	}

	got := decrypt(t, out, id)
	want := documents(t, []byte(plain))
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("round trip:\n got %#v\nwant %#v", got, want)
	}
}

// TestSopsDecrypts hands the output to the real sops, when it is installed.
func TestSopsDecrypts(t *testing.T) {
	bin := os.Getenv("MESA_TEST_SOPS")
	if bin == "" {
		if p, err := exec.LookPath("sops"); err == nil {
			bin = p
		}
	}
	if bin == "" {
		t.Skip("sops is not installed (set MESA_TEST_SOPS to its path)")
	}
	id, err := age.GenerateX25519Identity()
	if err != nil {
		t.Fatal(err)
	}
	out, err := Encrypt([]byte(plain), []string{id.Recipient().String()}, now)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "secrets.yaml")
	if err := os.WriteFile(path, out, 0o600); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(bin, "decrypt", path)
	cmd.Env = append(os.Environ(), "SOPS_AGE_KEY="+id.String())
	dec, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("sops decrypt: %v\n%s\n--- file ---\n%s", err, dec, out)
	}
	if got, want := documents(t, dec), documents(t, []byte(plain)); !reflect.DeepEqual(got, want) {
		t.Fatalf("sops decrypted:\n%s", dec)
	}
}

func TestEncryptRejects(t *testing.T) {
	id, _ := age.GenerateX25519Identity()
	r := []string{id.Recipient().String()}
	if _, err := Encrypt([]byte(plain), nil, now); err == nil {
		t.Error("no recipients")
	}
	if _, err := Encrypt([]byte(plain), []string{"age1nope"}, now); err == nil {
		t.Error("bad recipient")
	}
	if _, err := Encrypt([]byte("- a\n- b\n"), r, now); err == nil {
		t.Error("a sequence document")
	}
	if _, err := Encrypt([]byte("a: 1\nsops: {}\n"), r, now); err == nil {
		t.Error("already encrypted")
	}
	if _, err := Encrypt([]byte("a: 2026-10-29T18:00:00Z\n"), r, now); err == nil {
		t.Error("a timestamp, which sops and YAML disagree on")
	}
}

// decrypt is sops' decryption written from its format, independent of
// Encrypt: the data key from age, every value with its key path, then the
// MAC over the plaintext in document order.
func decrypt(t *testing.T, file []byte, id *age.X25519Identity) []any {
	t.Helper()
	nodes := yamlNodes(t, file)
	var meta struct {
		Age []struct {
			Enc string `yaml:"enc"`
		} `yaml:"age"`
		LastModified string `yaml:"lastmodified"`
		MAC          string `yaml:"mac"`
	}
	for i := 0; i+1 < len(nodes[0].Content); i += 2 {
		if nodes[0].Content[i].Value == "sops" {
			if err := nodes[0].Content[i+1].Decode(&meta); err != nil {
				t.Fatal(err)
			}
		}
	}
	r, err := age.Decrypt(armor.NewReader(strings.NewReader(meta.Age[0].Enc)), id)
	if err != nil {
		t.Fatalf("age: %v", err)
	}
	key, _ := io.ReadAll(r)
	block, _ := aes.NewCipher(key)
	gcm, _ := cipher.NewGCMWithNonceSize(block, 32)
	re := regexp.MustCompile(`^ENC\[AES256_GCM,data:(.*),iv:(.+),tag:(.+),type:(.+)\]$`)
	open := func(v, aad string) (string, string) {
		m := re.FindStringSubmatch(v)
		if m == nil {
			t.Fatalf("not a sops value: %q", v)
		}
		b64 := base64.StdEncoding
		data, _ := b64.DecodeString(m[1])
		iv, _ := b64.DecodeString(m[2])
		tag, _ := b64.DecodeString(m[3])
		p, err := gcm.Open(nil, iv, append(data, tag...), []byte(aad))
		if err != nil {
			t.Fatalf("open %q with %q: %v", v, aad, err)
		}
		return string(p), m[4]
	}
	mac := sha512.New()
	var walk func(n *yaml.Node, path []string) any
	walk = func(n *yaml.Node, path []string) any {
		switch n.Kind {
		case yaml.MappingNode:
			out := map[string]any{}
			for i := 0; i+1 < len(n.Content); i += 2 {
				k := n.Content[i].Value
				if k == "sops" && len(path) == 0 {
					continue
				}
				out[k] = walk(n.Content[i+1], append(path[:len(path):len(path)], k))
			}
			return out
		case yaml.SequenceNode:
			out := make([]any, len(n.Content))
			for i, e := range n.Content {
				out[i] = walk(e, path)
			}
			return out
		}
		if n.Value == "" || strings.HasSuffix(path[len(path)-1], unencryptedSuffix) {
			_, _ = mac.Write([]byte(n.Value))
			return n.Value
		}
		p, typ := open(n.Value, strings.Join(path, ":")+":")
		_, _ = mac.Write([]byte(p))
		switch typ {
		case "int":
			var i int
			_, _ = fmt.Sscan(p, &i)
			return i
		case "bool":
			return p == "True"
		}
		return p
	}
	var out []any
	for _, d := range nodes {
		out = append(out, walk(d, nil))
	}
	want, _ := open(meta.MAC, meta.LastModified)
	if got := fmt.Sprintf("%X", mac.Sum(nil)); got != want {
		t.Fatalf("MAC = %s, file says %s", got, want)
	}
	return out
}

func yamlNodes(t *testing.T, b []byte) []*yaml.Node {
	t.Helper()
	var out []*yaml.Node
	dec := yaml.NewDecoder(bytes.NewReader(b))
	for {
		var n yaml.Node
		if err := dec.Decode(&n); errors.Is(err, io.EOF) {
			break
		} else if err != nil {
			t.Fatal(err)
		}
		out = append(out, n.Content[0])
	}
	return out
}

func documents(t *testing.T, b []byte) []any {
	t.Helper()
	var out []any
	dec := yaml.NewDecoder(bytes.NewReader(b))
	for {
		var d map[string]any
		if err := dec.Decode(&d); errors.Is(err, io.EOF) {
			break
		} else if err != nil {
			t.Fatal(err)
		}
		delete(d, "sops")
		out = append(out, normalize(d))
	}
	return out
}

// normalize makes decoded YAML comparable: every map as map[string]any.
func normalize(v any) any {
	switch x := v.(type) {
	case map[string]any:
		out := map[string]any{}
		for k, e := range x {
			out[k] = normalize(e)
		}
		return out
	case []any:
		out := make([]any, len(x))
		for i, e := range x {
			out[i] = normalize(e)
		}
		return out
	}
	return v
}
