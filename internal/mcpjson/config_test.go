package mcpjson

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const fixture = `{
  // cursor-style MCP config
  "mcpServers": {
    "filesystem": {
      "command": "npx",
      "args": ["-y", "@modelcontextprotocol/server-filesystem", "."],
      "env": { "DEBUG": "1" }
    },
    "remote-thing": {
      "url": "https://example.com/mcp"
    }
  }
}
`

func writeFixture(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "mcp.json")
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
	data, _ := os.ReadFile(path)
	text := string(data)
	for _, want := range []string{
		"cursor-style MCP config",
		`"command": "doupass"`,
		`"proxy"`,
		`"--server"`,
		`"filesystem"`,
		`"https://example.com/mcp"`,
		`"DEBUG"`,
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
		t.Fatalf("result = %+v", res)
	}
	data, _ := os.ReadFile(path)
	text := string(data)
	if strings.Contains(text, "doupass") {
		t.Fatalf("doupass still present:\n%s", text)
	}
	if !strings.Contains(text, "@modelcontextprotocol/server-filesystem") {
		t.Fatal("original command not restored")
	}
	if !strings.Contains(text, "cursor-style MCP config") {
		t.Fatal("comments were lost")
	}
}

func TestInvalidConfigRejected(t *testing.T) {
	path := filepath.Join(t.TempDir(), "mcp.json")
	if err := os.WriteFile(path, []byte("{broken"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := WrapServers(path); err == nil {
		t.Fatal("expected parse error")
	}
	data, _ := os.ReadFile(path)
	if string(data) != "{broken" {
		t.Fatal("invalid config was modified")
	}
}

func TestNoMcpServersIsNoop(t *testing.T) {
	path := filepath.Join(t.TempDir(), "mcp.json")
	if err := os.WriteFile(path, []byte(`{"other": true}`), 0o600); err != nil {
		t.Fatal(err)
	}
	res, err := WrapServers(path)
	if err != nil {
		t.Fatal(err)
	}
	if res.Changed {
		t.Fatal("config without mcpServers should not change")
	}
}
