package postgres

import (
	"strings"
	"testing"
)

func TestColumnTypes(t *testing.T) {
	seen := map[string]bool{}
	for _, ct := range New().Capabilities().Types {
		if seen[ct.Name] {
			t.Errorf("%s is listed twice", ct.Name)
		}
		seen[ct.Name] = true
		if ct.Group == "" || !strings.HasPrefix(ct.Spelling, strings.TrimSuffix(ct.Name, "[]")) {
			t.Errorf("%s: group %q, spelling %q", ct.Name, ct.Group, ct.Spelling)
		}
		if ct.Args != "" && !strings.Contains(ct.Spelling, "(") {
			t.Errorf("%s: args %q but spelling %q", ct.Name, ct.Args, ct.Spelling)
		}
	}
}
