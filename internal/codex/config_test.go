package codex

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/BurntSushi/toml"
)

const sampleConfig = `model = "gpt-6"
[projects.'C:\proj']
trust_level = "trusted"

[mcp_servers.node_repl]
args = []
command = 'C:\tools\node_repl.exe'
startup_timeout_sec = 120

[mcp_servers.node_repl.env]
CACHE = 'C:\temp'

[mcp_servers.remote_api]
url = "https://mcp.example.com/sse"

[mcp_servers.fs]
command = "npx"
args = ["-y", "server-fs", "."]
`

func writeConfig(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestWrapAndUnwrapRoundTrip(t *testing.T) {
	path := writeConfig(t, sampleConfig)

	res, err := WrapServers(path)
	if err != nil {
		t.Fatal(err)
	}
	if !res.Changed || res.Servers != 2 {
		t.Fatalf("wrap = %+v, want 2 servers changed", res)
	}
	if res.Backup == "" {
		t.Fatal("backup missing")
	}

	wrapped, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	text := string(wrapped)
	for _, want := range []string{
		`command = "doupass"`,
		`"proxy"`,
		`"node_repl"`,
		`C:\\tools\\node_repl.exe`,
		"CACHE",
		`trust_level = "trusted"`,
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("wrapped config missing %q:\n%s", want, text)
		}
	}
	if strings.Contains(text, "url") && strings.Count(text, "command = \"doupass\"") != 2 {
		t.Fatalf("expected exactly 2 wrapped commands (remote url server untouched):\n%s", text)
	}

	if res2, err := WrapServers(path); err != nil || res2.Changed {
		t.Fatalf("second wrap should be a no-op, got %+v err=%v", res2, err)
	}
	if !HasWrappers(path) {
		t.Fatal("HasWrappers = false after wrap")
	}

	res, err = UnwrapServers(path)
	if err != nil {
		t.Fatal(err)
	}
	if !res.Changed || res.Servers != 2 {
		t.Fatalf("unwrap = %+v, want 2 servers", res)
	}
	if !sameTOML(t, path, sampleConfig) {
		t.Fatal("unwrapped config is not semantically equal to the original")
	}
	if HasWrappers(path) {
		t.Fatal("HasWrappers = true after unwrap")
	}
}

func sameTOML(t *testing.T, path, want string) bool {
	t.Helper()
	var gotDoc, wantDoc map[string]any
	if err := toml.Unmarshal([]byte(want), &wantDoc); err != nil {
		t.Fatalf("parse original: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := toml.Unmarshal(data, &gotDoc); err != nil {
		t.Fatalf("parse unwrapped: %v", err)
	}
	return reflect.DeepEqual(gotDoc, wantDoc)
}

func TestUnwrapWithoutWrappers(t *testing.T) {
	path := writeConfig(t, sampleConfig)
	res, err := UnwrapServers(path)
	if err != nil || res.Changed {
		t.Fatalf("unwrap on clean config should be a no-op, got %+v err=%v", res, err)
	}
}

func TestWrapMissingFile(t *testing.T) {
	if _, err := WrapServers(filepath.Join(t.TempDir(), "missing.toml")); err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestWrapInvalidTOML(t *testing.T) {
	path := writeConfig(t, "this is not = = toml")
	if _, err := WrapServers(path); err == nil {
		t.Fatal("expected parse error")
	}
}
