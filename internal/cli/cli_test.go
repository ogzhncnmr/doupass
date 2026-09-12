package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ogzhncnmr/doupass/internal/audit"
	"github.com/ogzhncnmr/doupass/internal/policy"
)

func run(t *testing.T, args ...string) (string, string, error) {
	t.Helper()
	var out, errOut bytes.Buffer
	err := Execute(args, &out, &errOut)
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
