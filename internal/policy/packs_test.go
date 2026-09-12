package policy

import (
	"path/filepath"
	"testing"
)

func TestPackPoliciesAreValid(t *testing.T) {
	matches, err := filepath.Glob(filepath.Join(specDir(t), "..", "rules", "packs", "*.yml"))
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) == 0 {
		t.Fatal("no rule packs found")
	}
	for _, path := range matches {
		p, err := LoadFile(path)
		if err != nil {
			t.Fatalf("%s: %v", path, err)
		}
		if _, err := New(p, Options{}); err != nil {
			t.Fatalf("%s: %v", path, err)
		}
		if issues := Lint(p); len(issues) > 0 {
			t.Fatalf("%s: lint issues: %+v", path, issues)
		}
	}
}
