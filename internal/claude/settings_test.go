package claude

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInstallCreatesSettings(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".claude", "settings.json")
	res, err := InstallHook(path, "doupass hook claude")
	if err != nil {
		t.Fatal(err)
	}
	if !res.Changed {
		t.Fatal("expected change on fresh install")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "doupass hook claude") {
		t.Fatalf("settings missing hook: %s", data)
	}
	var settings map[string]any
	if err := json.Unmarshal(data, &settings); err != nil {
		t.Fatalf("settings not valid JSON: %v", err)
	}
}

func TestInstallPreservesExistingSettings(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	existing := `{
  "permissions": {"allow": ["Bash(npm test *)"]},
  "hooks": {
    "PreToolUse": [
      {"matcher": "Bash", "hooks": [{"type": "command", "command": "echo existing"}]}
    ]
  }
}`
	if err := os.WriteFile(path, []byte(existing), 0o600); err != nil {
		t.Fatal(err)
	}
	res, err := InstallHook(path, "doupass hook claude")
	if err != nil {
		t.Fatal(err)
	}
	if !res.Changed || res.Backup == "" {
		t.Fatalf("expected change with backup, got %+v", res)
	}
	if _, err := os.Stat(res.Backup); err != nil {
		t.Fatalf("backup missing: %v", err)
	}
	var settings map[string]any
	data, _ := os.ReadFile(path)
	if err := json.Unmarshal(data, &settings); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "echo existing") || !strings.Contains(string(data), "Bash(npm test *)") {
		t.Fatalf("existing entries lost: %s", data)
	}
	perms := settings["permissions"].(map[string]any)
	if len(perms["allow"].([]any)) != 1 {
		t.Fatal("permissions not preserved")
	}
}

func TestInstallIsIdempotent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	if _, err := InstallHook(path, "doupass hook claude"); err != nil {
		t.Fatal(err)
	}
	before, _ := os.ReadFile(path)
	res, err := InstallHook(path, "doupass hook claude")
	if err != nil {
		t.Fatal(err)
	}
	if res.Changed {
		t.Fatal("second install should be a no-op")
	}
	after, _ := os.ReadFile(path)
	if string(before) != string(after) {
		t.Fatal("file changed on idempotent install")
	}
}

func TestUninstallRemovesOnlyDoupass(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	existing := `{
  "hooks": {
    "PreToolUse": [
      {"matcher": "Bash", "hooks": [{"type": "command", "command": "echo existing"}]}
    ]
  }
}`
	if err := os.WriteFile(path, []byte(existing), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := InstallHook(path, "doupass hook claude"); err != nil {
		t.Fatal(err)
	}
	res, err := UninstallHook(path)
	if err != nil {
		t.Fatal(err)
	}
	if !res.Changed {
		t.Fatal("expected change on uninstall")
	}
	data, _ := os.ReadFile(path)
	if strings.Contains(string(data), "doupass hook") {
		t.Fatalf("doupass hook still present: %s", data)
	}
	if !strings.Contains(string(data), "echo existing") {
		t.Fatalf("existing hook removed: %s", data)
	}
}

func TestUninstallNoDoupassIsNoop(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	if err := os.WriteFile(path, []byte(`{"hooks": {}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	res, err := UninstallHook(path)
	if err != nil {
		t.Fatal(err)
	}
	if res.Changed {
		t.Fatal("uninstall without doupass hook should be a no-op")
	}
}

func TestUninstallCleansEmptyHooks(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	if _, err := InstallHook(path, "doupass hook claude"); err != nil {
		t.Fatal(err)
	}
	if _, err := UninstallHook(path); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(path)
	var settings map[string]any
	if err := json.Unmarshal(data, &settings); err != nil {
		t.Fatal(err)
	}
	if _, ok := settings["hooks"]; ok {
		t.Fatalf("empty hooks key should be removed: %s", data)
	}
}

func TestInvalidSettingsRejected(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	if err := os.WriteFile(path, []byte("{invalid"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := InstallHook(path, "doupass hook claude"); err == nil {
		t.Fatal("expected error for invalid JSON")
	}
	data, _ := os.ReadFile(path)
	if string(data) != "{invalid" {
		t.Fatal("invalid settings file was modified")
	}
}
