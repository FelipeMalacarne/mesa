package sops

import (
	"strings"
	"testing"
)

func TestRecipients(t *testing.T) {
	config := []byte(`creation_rules:
  - path_regex: ^clusters/zamorak/.*secrets?\.yaml$
    key_groups:
      - age: [age1zamorak]
  - path_regex: .*secrets?(\.dec)?\.yaml$
    age: "age1one, age1two"
`)
	for path, want := range map[string]string{
		"clusters/saradomin/apps/postgres/mesa/secrets/shop-secrets.yaml": "age1one age1two",
		"clusters/zamorak/apps/postgres/mesa/secrets/shop-secrets.yaml":   "age1zamorak",
	} {
		got, err := Recipients(config, path)
		if err != nil {
			t.Fatalf("%s: %v", path, err)
		}
		if s := strings.Join(got, " "); s != want {
			t.Errorf("%s: %q, want %q", path, s, want)
		}
	}
	if _, err := Recipients(config, "clusters/saradomin/apps/postgres/cluster.yaml"); err == nil {
		t.Error("a path no rule matches")
	}
	if _, err := Recipients([]byte("creation_rules: [{path_regex: '('}]"), "x"); err == nil {
		t.Error("a bad regex")
	}
}
