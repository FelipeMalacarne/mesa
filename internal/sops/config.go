package sops

import (
	"fmt"
	"regexp"
	"strings"

	"go.yaml.in/yaml/v3"
)

// Recipients returns the age recipients a repository's .sops.yaml gives a
// file: those of the first creation rule whose path_regex matches path
// (relative to the repository root), as sops itself picks them.
func Recipients(config []byte, path string) ([]string, error) {
	var c struct {
		CreationRules []struct {
			PathRegex string `yaml:"path_regex"`
			Age       any    `yaml:"age"`
			KeyGroups []struct {
				Age []string `yaml:"age"`
			} `yaml:"key_groups"`
		} `yaml:"creation_rules"`
	}
	if err := yaml.Unmarshal(config, &c); err != nil {
		return nil, fmt.Errorf("sops: read .sops.yaml: %w", err)
	}
	for _, r := range c.CreationRules {
		if r.PathRegex != "" {
			re, err := regexp.Compile(r.PathRegex)
			if err != nil {
				return nil, fmt.Errorf("sops: path_regex %q: %w", r.PathRegex, err)
			}
			if !re.MatchString(path) {
				continue
			}
		}
		var out []string
		switch a := r.Age.(type) {
		case string:
			for _, k := range strings.Split(a, ",") {
				if k = strings.TrimSpace(k); k != "" {
					out = append(out, k)
				}
			}
		case []any:
			for _, k := range a {
				if s, ok := k.(string); ok && strings.TrimSpace(s) != "" {
					out = append(out, strings.TrimSpace(s))
				}
			}
		}
		for _, g := range r.KeyGroups {
			out = append(out, g.Age...)
		}
		if len(out) == 0 {
			return nil, fmt.Errorf("sops: the rule for %s has no age recipients", path)
		}
		return out, nil
	}
	return nil, fmt.Errorf("sops: no creation rule in .sops.yaml matches %s", path)
}
