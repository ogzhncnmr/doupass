package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/ogzhncnmr/doupass/internal/audit"
	"github.com/ogzhncnmr/doupass/internal/policy"
)

func TestMain(m *testing.M) {
	home, err := os.MkdirTemp("", "doupass-cli-test-home")
	if err != nil {
		panic(err)
	}
	_ = os.Setenv("HOME", home)
	_ = os.Setenv("USERPROFILE", home)
	code := m.Run()
	_ = os.RemoveAll(home)
	os.Exit(code)
}

func run(t *testing.T, args ...string) (string, string, error) {
	t.Helper()
	return runWithStdin(t, "", args...)
}

func runWithStdin(t *testing.T, stdin string, args ...string) (string, string, error) {
	t.Helper()
	var out, errOut bytes.Buffer
	err := Execute(args, strings.NewReader(stdin), &out, &errOut)
	return out.String(), errOut.String(), err
}

func examplePolicy() string {
	return filepath.Join("..", "..", "spec", "examples", "default-dev.yml")
}

func TestVersionCommand(t *testing.T) {
	out, _, err := run(t, "version")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "doupass") {
		t.Fatalf("out = %q", out)
	}
}

func TestResolvedVersion(t *testing.T) {
	old := Version
	defer func() { Version = old }()
	Version = "1.2.3"
	if got := ResolvedVersion(); got != "1.2.3" {
		t.Fatalf("got %q, want 1.2.3", got)
	}
	Version = "dev"
	if got := ResolvedVersion(); got != "dev" {
		t.Fatalf("test binary should report dev, got %q", got)
	}
}

func TestPolicyTestValidates(t *testing.T) {
	out, _, err := run(t, "policy", "test", examplePolicy())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "OK: default-dev") {
		t.Fatalf("out = %q", out)
	}
}

func TestPolicyTestDecides(t *testing.T) {
	out, _, err := run(t, "policy", "test", examplePolicy(),
		"--surface", "mcp", "--tool", "read_file", "--arg", "path=/home/x/.ssh/id_rsa")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, `"action": "deny"`) || !strings.Contains(out, "block-ssh-credentials") {
		t.Fatalf("out = %q", out)
	}
}

func TestPolicyTestBadArg(t *testing.T) {
	_, _, err := run(t, "policy", "test", examplePolicy(), "--tool", "Read", "--arg", "nope")
	if err == nil {
		t.Fatal("expected error for malformed --arg")
	}
}

func TestInitCommand(t *testing.T) {
	dir := t.TempDir()
	out, _, err := run(t, "init", "--dir", dir)
	if err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(dir, "doupass.yml")
	if _, err := os.Stat(target); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "wrote") {
		t.Fatalf("out = %q", out)
	}
	if _, _, err := run(t, "init", "--dir", dir); err == nil {
		t.Fatal("expected exists error")
	}
	if _, _, err := run(t, "init", "--dir", dir, "--force"); err != nil {
		t.Fatalf("force overwrite: %v", err)
	}
}

func TestLogVerifyAndTail(t *testing.T) {
	path := filepath.Join(t.TempDir(), "audit.jsonl")
	l := &audit.Logger{Path: path}
	if err := l.Append(policy.Call{Surface: "hook", Tool: "Bash"}, policy.Decision{Action: policy.ActionDeny, RuleID: "r1"}); err != nil {
		t.Fatal(err)
	}
	out, _, err := run(t, "log", "verify", "--path", path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "OK: 1 entries") {
		t.Fatalf("verify out = %q", out)
	}
	out, _, err = run(t, "log", "tail", "--path", path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "Bash") || !strings.Contains(out, "deny") {
		t.Fatalf("tail out = %q", out)
	}
}

func TestLogVerifyTamperFails(t *testing.T) {
	path := filepath.Join(t.TempDir(), "audit.jsonl")
	l := &audit.Logger{Path: path}
	if err := l.Append(policy.Call{Tool: "Bash"}, policy.Decision{Action: policy.ActionAllow}); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	tampered := bytes.Replace(data, []byte(`"allow"`), []byte(`"deny"`), 1)
	if err := os.WriteFile(path, tampered, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, _, err := run(t, "log", "verify", "--path", path); err == nil {
		t.Fatal("expected verify error on tampered log")
	}
}

func TestProxyRequiresServerAndCommand(t *testing.T) {
	if _, _, err := run(t, "proxy", "--policy", examplePolicy()); err == nil {
		t.Fatal("expected --server error")
	}
	if _, _, err := run(t, "proxy", "--server", "fs", "--policy", examplePolicy()); err == nil {
		t.Fatal("expected missing command error")
	}
}

func TestHookClaudeDenies(t *testing.T) {
	input := `{"hook_event_name":"PreToolUse","tool_name":"Read","tool_input":{"file_path":"/home/x/.ssh/id_rsa"}}`
	out, _, err := runWithStdin(t, input, "hook", "claude", "--policy", examplePolicy())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, `"permissionDecision":"deny"`) {
		t.Fatalf("out = %q", out)
	}
}

func TestHookClaudeAllowsSilently(t *testing.T) {
	input := `{"hook_event_name":"PreToolUse","tool_name":"Read","tool_input":{"file_path":"/etc/hosts"}}`
	out, _, err := runWithStdin(t, input, "hook", "claude", "--policy", examplePolicy())
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(out) != "" {
		t.Fatalf("expected no output for allow, got %q", out)
	}
}

func TestDecideCommand(t *testing.T) {
	input := `{"surface":"mcp","server":"fs","tool":"read_file","args":{"path":"/home/x/.ssh/id_rsa"}}`
	out, _, err := runWithStdin(t, input, "decide", "--policy", examplePolicy())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, `"action":"deny"`) || !strings.Contains(out, "block-ssh-credentials") {
		t.Fatalf("out = %q", out)
	}
}

func TestDecideCommandDefaultsToHookSurface(t *testing.T) {
	input := `{"tool":"Bash","args":{"command":"npm install x"}}`
	out, _, err := runWithStdin(t, input, "decide", "--policy", examplePolicy())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, `"action":"ask"`) || !strings.Contains(out, "ask-npm-install") {
		t.Fatalf("out = %q", out)
	}
}

func TestDecideCommandRejectsInvalidJSON(t *testing.T) {
	if _, _, err := runWithStdin(t, "not json", "decide", "--policy", examplePolicy()); err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestDecideCommandFromInputFile(t *testing.T) {
	inputPath := filepath.Join(t.TempDir(), "call.json")
	if err := os.WriteFile(inputPath, []byte(`{"tool":"Read","args":{"file_path":"/home/x/.ssh/id_rsa"}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	out, _, err := run(t, "decide", "--policy", examplePolicy(), "--input", inputPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, `"action":"deny"`) {
		t.Fatalf("out = %q", out)
	}
}

func TestSetupCommand(t *testing.T) {
	home := os.Getenv("USERPROFILE")
	if home == "" {
		home = os.Getenv("HOME")
	}
	t.Setenv("AppData", filepath.Join(t.TempDir(), "appdata"))
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(t.TempDir(), "xdg"))

	cursorDir := filepath.Join(home, ".cursor")
	if err := os.MkdirAll(cursorDir, 0o755); err != nil {
		t.Fatal(err)
	}
	mcpPath := filepath.Join(cursorDir, "mcp.json")
	if err := os.WriteFile(mcpPath, []byte(`{"mcpServers":{"fs":{"command":"npx","args":["-y","server-fs","."]}}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	claudeDir := filepath.Join(home, ".claude")
	if err := os.MkdirAll(claudeDir, 0o755); err != nil {
		t.Fatal(err)
	}
	settingsPath := filepath.Join(claudeDir, "settings.json")
	if err := os.WriteFile(settingsPath, []byte("{}"), 0o600); err != nil {
		t.Fatal(err)
	}

	out, _, err := run(t, "setup", "--dry-run")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "cursor") || !strings.Contains(out, "would wrap") {
		t.Fatalf("dry-run out = %q", out)
	}
	data, _ := os.ReadFile(mcpPath)
	if strings.Contains(string(data), "doupass") {
		t.Fatal("dry-run modified the cursor config")
	}

	out, _, err = run(t, "setup")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "wrapped 1 MCP server") {
		t.Fatalf("setup out = %q", out)
	}
	data, _ = os.ReadFile(mcpPath)
	if !strings.Contains(string(data), "doupass") {
		t.Fatal("cursor config was not wrapped")
	}
	settings, _ := os.ReadFile(settingsPath)
	if !strings.Contains(string(settings), "doupass hook") {
		t.Fatal("claude hook was not installed")
	}
}

func TestInstallAndUninstallGeneric(t *testing.T) {
	cfg := filepath.Join(t.TempDir(), "mcp.json")
	if err := os.WriteFile(cfg, []byte(`{"mcpServers":{"fs":{"command":"npx","args":["-y","x"]}}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	out, _, err := run(t, "install", "generic", "--config", cfg)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "wrapped 1 MCP server") {
		t.Fatalf("out = %q", out)
	}
	out, _, err = run(t, "uninstall", "generic", "--config", cfg)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "unwrapped 1 MCP server") {
		t.Fatalf("out = %q", out)
	}
	data, _ := os.ReadFile(cfg)
	if strings.Contains(string(data), "doupass") {
		t.Fatalf("config not restored: %s", data)
	}
}

func TestDoctorCommand(t *testing.T) {
	home := os.Getenv("USERPROFILE")
	if home == "" {
		home = os.Getenv("HOME")
	}
	dir := filepath.Join(home, ".doupass")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	src, err := os.ReadFile(examplePolicy())
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "doupass.yml"), src, 0o600); err != nil {
		t.Fatal(err)
	}
	out, _, err := run(t, "doctor")
	if err != nil {
		t.Fatalf("doctor: %v (out=%s)", err, out)
	}
	for _, want := range []string{"policy", "audit", "integrations"} {
		if !strings.Contains(out, want) {
			t.Fatalf("doctor output missing %q:\n%s", want, out)
		}
	}
}

func TestDoctorReportsMissingPolicy(t *testing.T) {
	home := t.TempDir()
	t.Setenv("USERPROFILE", home)
	t.Setenv("HOME", home)
	_, _, err := run(t, "doctor")
	if err == nil || !strings.Contains(err.Error(), "problem") {
		t.Fatalf("expected problems error, got %v", err)
	}
}

func TestLandingStaticWithoutTerminal(t *testing.T) {
	out, _, err := run(t)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"Getting started", "doupass doctor", "interactive menu"} {
		if !strings.Contains(out, want) {
			t.Fatalf("missing %q in:\n%s", want, out)
		}
	}
}

func TestLandingMenuModel(t *testing.T) {
	m := newLandingModel("test status")
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m = updated.(landingModel)
	if m.cursor != 1 {
		t.Fatalf("cursor = %d, want 1", m.cursor)
	}
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(landingModel)
	if len(m.chosen) != 2 || m.chosen[0] != "setup" || m.chosen[1] != "--dry-run" {
		t.Fatalf("chosen = %v, want [setup --dry-run]", m.chosen)
	}

	wrapped := newLandingModel("")
	updated, _ = wrapped.Update(tea.KeyMsg{Type: tea.KeyUp})
	wrapped = updated.(landingModel)
	if wrapped.cursor != len(landingChoices)-1 {
		t.Fatalf("wrap-around cursor = %d, want %d", wrapped.cursor, len(landingChoices)-1)
	}
	updated, _ = wrapped.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
	wrapped = updated.(landingModel)
	if !wrapped.quit {
		t.Fatal("q should quit")
	}
	if wrapped.chosen != nil {
		t.Fatal("quitting should not choose a command")
	}

	view := newLandingModel("policy starter (18 rules)").View()
	if !strings.Contains(view, "Show status") || !strings.Contains(view, "policy starter") {
		t.Fatalf("view missing content:\n%s", view)
	}
}

func TestPolicyCommandsExpandHome(t *testing.T) {
	home := os.Getenv("USERPROFILE")
	if home == "" {
		home = os.Getenv("HOME")
	}
	src, err := os.ReadFile(examplePolicy())
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(home, "tilde-policy.yml"), src, 0o600); err != nil {
		t.Fatal(err)
	}
	out, _, err := run(t, "policy", "lint", "~/tilde-policy.yml")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "default-dev") {
		t.Fatalf("lint out = %q", out)
	}
	out, _, err = run(t, "policy", "test", "~/tilde-policy.yml", "--tool", "Read", "--arg", "file_path=/home/x/.ssh/id_rsa")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, `"action": "deny"`) {
		t.Fatalf("test out = %q", out)
	}
	if _, _, err := run(t, "policy", "test", "~/missing-policy.yml"); err == nil || !strings.Contains(err.Error(), "policy file not found") {
		t.Fatalf("expected friendly not-found error, got %v", err)
	}
}

func TestInstallAndUninstallClaude(t *testing.T) {
	settings := filepath.Join(t.TempDir(), "settings.json")
	out, _, err := run(t, "install", "claude", "--settings", settings)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "installed hook") {
		t.Fatalf("out = %q", out)
	}
	data, err := os.ReadFile(settings)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "doupass hook") {
		t.Fatalf("settings = %s", data)
	}
	out, _, err = run(t, "install", "claude", "--settings", settings)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "already installed") {
		t.Fatalf("out = %q", out)
	}
	out, _, err = run(t, "uninstall", "claude", "--settings", settings)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "removed doupass hook") {
		t.Fatalf("out = %q", out)
	}
	data, err = os.ReadFile(settings)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "doupass hook") {
		t.Fatalf("hook still present: %s", data)
	}
}

func TestInstallAndUninstallOpenCode(t *testing.T) {
	config := filepath.Join(t.TempDir(), "opencode.json")
	body := `{"mcp": {"fs": {"type": "local", "command": ["npx", "-y", "server-fs", "."]}}}`
	if err := os.WriteFile(config, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	out, _, err := run(t, "install", "opencode", "--config", config)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "wrapped 1 MCP server") {
		t.Fatalf("out = %q", out)
	}
	data, err := os.ReadFile(config)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `"doupass"`) {
		t.Fatalf("config = %s", data)
	}
	out, _, err = run(t, "uninstall", "opencode", "--config", config)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "unwrapped 1 MCP server") {
		t.Fatalf("out = %q", out)
	}
	data, err = os.ReadFile(config)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "doupass") {
		t.Fatalf("config = %s", data)
	}
}

func TestInstallAndUninstallOpenCodePlugin(t *testing.T) {
	dir := t.TempDir()
	out, _, err := run(t, "install", "opencode", "--plugin", "--plugin-dir", dir)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "installed native-tool plugin") {
		t.Fatalf("out = %q", out)
	}
	pluginPath := filepath.Join(dir, "doupass.js")
	data, err := os.ReadFile(pluginPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "tool.execute.before") {
		t.Fatalf("plugin = %s", data)
	}
	out, _, err = run(t, "install", "opencode", "--plugin", "--plugin-dir", dir)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "already installed") {
		t.Fatalf("out = %q", out)
	}
	out, _, err = run(t, "uninstall", "opencode", "--plugin", "--plugin-dir", dir)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "removed plugin") {
		t.Fatalf("out = %q", out)
	}
	if _, err := os.Stat(pluginPath); !os.IsNotExist(err) {
		t.Fatal("plugin still present")
	}
}
