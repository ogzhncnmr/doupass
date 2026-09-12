package opencode

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const fixture = `{
  // keep this top comment
  "$schema": "https://opencode.ai/config.json",
  "mcp": {
    "fs": {
      // local filesystem server
      "type": "local",
      "command": ["npx", "-y", "@modelcontextprotocol/server-filesystem", "/work"],
      "environment": { "DEBUG": "1" }
    },
    "remote": {
      "type": "remote",
      "url": "https://example.com/mcp"
    }
  }
}
`

func writeFixture(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "opencode.jsonc")
	if err := os.WriteFile(path, []byte(fixture), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestWrapServers(t *testing.T) {
	path := writeFixture(t)
	res, err := WrapServers(path)
	if err != nil {
		t.Fatal(err)
	}
	if !res.Changed || res.Servers != 1 {
		t.Fatalf("result = %+v, want changed with 1 server", res)
	}
	if res.Backup == "" {
		t.Fatal("expected backup path")
	}
	data, _ := os.ReadFile(path)
	text := string(data)
	for _, want := range []string{
		"keep this top comment",
		"local filesystem server",
		`"doupass"`,
		`"proxy"`,
		`"--server"`,
		`"fs"`,
		`"environment"`,
		`"https://example.com/mcp"`,
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("missing %q in:\n%s", want, text)
		}
	}
}

func TestWrapIsIdempotent(t *testing.T) {
	path := writeFixture(t)
	if _, err := WrapServers(path); err != nil {
		t.Fatal(err)
	}
	before, _ := os.ReadFile(path)
	res, err := WrapServers(path)
	if err != nil {
		t.Fatal(err)
	}
	if res.Changed {
		t.Fatal("second wrap should be a no-op")
	}
	after, _ := os.ReadFile(path)
	if string(before) != string(after) {
		t.Fatal("file changed on idempotent wrap")
	}
}

func TestUnwrapRestoresCommand(t *testing.T) {
	path := writeFixture(t)
	if _, err := WrapServers(path); err != nil {
		t.Fatal(err)
	}
	res, err := UnwrapServers(path)
	if err != nil {
		t.Fatal(err)
	}
	if !res.Changed || res.Servers != 1 {
		t.Fatalf("result = %+v, want changed with 1 server", res)
	}
	data, _ := os.ReadFile(path)
	text := string(data)
	if strings.Contains(text, "doupass") {
		t.Fatalf("doupass still present:\n%s", text)
	}
	if !strings.Contains(text, "keep this top comment") {
		t.Fatal("comments were lost")
	}
	if !strings.Contains(text, "@modelcontextprotocol/server-filesystem") {
		t.Fatal("original command not restored")
	}
	res, err = UnwrapServers(path)
	if err != nil {
		t.Fatal(err)
	}
	if res.Changed {
		t.Fatal("second unwrap should be a no-op")
	}
}

func TestInvalidConfigRejected(t *testing.T) {
	path := filepath.Join(t.TempDir(), "opencode.json")
	if err := os.WriteFile(path, []byte("{not json"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := WrapServers(path); err == nil {
		t.Fatal("expected parse error")
	}
	data, _ := os.ReadFile(path)
	if string(data) != "{not json" {
		t.Fatal("invalid config was modified")
	}
}

func TestMissingFile(t *testing.T) {
	if _, err := WrapServers(filepath.Join(t.TempDir(), "absent.json")); err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestNoLocalServers(t *testing.T) {
	path := filepath.Join(t.TempDir(), "opencode.json")
	if err := os.WriteFile(path, []byte(`{"mcp": {"r": {"type": "remote", "url": "https://x"}}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	res, err := WrapServers(path)
	if err != nil {
		t.Fatal(err)
	}
	if res.Changed {
		t.Fatal("remote-only config should not change")
	}
}
