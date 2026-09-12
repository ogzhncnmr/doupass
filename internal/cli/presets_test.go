package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ogzhncnmr/doupass/internal/policy"
)

func TestPresetsAreValid(t *testing.T) {
	for _, name := range presetNames {
		data, err := presetsFS.ReadFile("presets/" + name + ".yml")
		if err != nil {
			t.Fatalf("preset %s: %v", name, err)
		}
		if _, err := policy.Parse(data); err != nil {
			t.Fatalf("preset %s: %v", name, err)
		}
	}
}

func TestPresetCopiesStayInSync(t *testing.T) {
	pairs := map[string]string{
		"starter":     filepath.Join("..", "..", "rules", "starter.yml"),
		"locked-down": filepath.Join("..", "..", "spec", "examples", "locked-down.yml"),
		"red-team":    filepath.Join("..", "..", "spec", "examples", "red-team.yml"),
	}
	normalize := func(b []byte) string {
		return strings.ReplaceAll(string(b), "\r\n", "\n")
	}
	for name, source := range pairs {
		want, err := os.ReadFile(source)
		if err != nil {
			t.Fatal(err)
		}
		got, err := presetsFS.ReadFile("presets/" + name + ".yml")
		if err != nil {
			t.Fatal(err)
		}
		if normalize(got) != normalize(want) {
			t.Fatalf("preset %s diverged from %s; update the embedded copy", name, source)
		}
	}
}

func TestInitRejectsUnknownPreset(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := run(t, "init", "--dir", dir, "--preset", "nope"); err == nil {
		t.Fatal("expected error for unknown preset")
	}
}
