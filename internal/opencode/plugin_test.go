package opencode

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInstallPlugin(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".opencode", "plugin", "doupass.js")
	res, err := InstallPlugin(path, `C:\Program Files\doupass.exe`)
	if err != nil {
		t.Fatal(err)
	}
	if !res.Changed {
		t.Fatal("expected change")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	for _, want := range []string{
		pluginMarker,
		`C:\\Program Files\\doupass.exe`,
		"tool.execute.before",
		"DoupassPlugin",
		"canonicalTool",
		"normalizeArgs",
		"fail-closed",
		"--input",
		"node:fs",
		"execFileSync",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("plugin missing %q:\n%s", want, text)
		}
	}
	res, err = InstallPlugin(path, "doupass")
	if err != nil {
		t.Fatal(err)
	}
	if res.Changed {
		t.Fatal("second install should be a no-op")
	}
}

func TestInstallPluginRefusesForeignFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "doupass.js")
	if err := os.WriteFile(path, []byte("// someone else's plugin"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := InstallPlugin(path, "doupass"); err == nil {
		t.Fatal("expected refusal for foreign file")
	}
	data, _ := os.ReadFile(path)
	if string(data) != "// someone else's plugin" {
		t.Fatal("foreign file was modified")
	}
}

func TestUninstallPlugin(t *testing.T) {
	path := filepath.Join(t.TempDir(), "doupass.js")
	if _, err := InstallPlugin(path, "doupass"); err != nil {
		t.Fatal(err)
	}
	res, err := UninstallPlugin(path)
	if err != nil {
		t.Fatal(err)
	}
	if !res.Changed {
		t.Fatal("expected change")
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("plugin file still present")
	}
	res, err = UninstallPlugin(path)
	if err != nil {
		t.Fatal(err)
	}
	if res.Changed {
		t.Fatal("second uninstall should be a no-op")
	}
}

func TestUninstallPluginKeepsForeignFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "doupass.js")
	if err := os.WriteFile(path, []byte("// not ours"), 0o600); err != nil {
		t.Fatal(err)
	}
	res, err := UninstallPlugin(path)
	if err != nil {
		t.Fatal(err)
	}
	if res.Changed {
		t.Fatal("foreign file should not be removed")
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatal("foreign file was removed")
	}
}
