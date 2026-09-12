package policy

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func mustParse(t *testing.T, yaml string) *Policy {
	t.Helper()
	p, err := Parse([]byte(yaml))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	return p
}

func mustEngine(t *testing.T, yaml string, opts Options) *Engine {
	t.Helper()
	engine, err := New(mustParse(t, yaml), opts)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	return engine
}

func TestParseErrors(t *testing.T) {
	cases := map[string]string{
		"unknown field": `
version: "0.1"
defaults: {action: allow}
rules: []
bogus: true
`,
		"unsupported version": `
version: "9.9"
defaults: {action: allow}
rules: []
`,
		"missing defaults action": `
version: "0.1"
rules: []
`,
		"invalid action": `
version: "0.1"
defaults: {action: allow}
rules:
  - id: bad
    match: {tool: "*"}
    action: explode
`,
		"invalid rule id": `
version: "0.1"
defaults: {action: allow}
rules:
  - id: "Bad ID"
    match: {tool: "*"}
    action: deny
`,
		"duplicate rule id": `
version: "0.1"
defaults: {action: allow}
rules:
  - id: dup
    match: {tool: "a"}
    action: deny
  - id: dup
    match: {tool: "b"}
    action: deny
`,
		"empty match": `
version: "0.1"
defaults: {action: allow}
rules:
  - id: empty
    match: {}
    action: deny
`,
		"bad surface": `
version: "0.1"
defaults: {action: allow}
rules:
  - id: s
    match: {surface: cli}
    action: deny
`,
		"invalid regex": `
version: "0.1"
defaults: {action: allow}
rules:
  - id: r
    match: {tool: "re:("}
    action: deny
`,
		"empty regex": `
version: "0.1"
defaults: {action: allow}
rules:
  - id: r
    match: {tool: "re:"}
    action: deny
`,
		"multiple documents": `
version: "0.1"
defaults: {action: allow}
rules: []
---
version: "0.1"
`,
		"bad ask fallback": `
version: "0.1"
defaults: {action: allow, ask_fallback: ask}
rules: []
`,
	}
	for name, yaml := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := Parse([]byte(yaml)); err == nil {
				t.Fatal("expected parse error, got nil")
			}
		})
	}
}

func TestGlobPatterns(t *testing.T) {
	cases := []struct {
		pattern string
		subject string
		want    bool
	}{
		{"*", "a/b", true},
		{"*", "", true},
		{"**/.ssh/**", "/home/dev/.ssh/id_rsa", true},
		{"**/.ssh/**", "cat ~/.ssh/id_rsa", true},
		{"**/.ssh/**", "/home/dev/.aws/credentials", false},
		{"?.txt", "a.txt", true},
		{"?.txt", "ab.txt", false},
		{`\*`, "*", true},
		{`\*`, "abc", false},
		{"re:^(Read|Glob)$", "Read", true},
		{"re:^(Read|Glob)$", "ReadFile", false},
		{"filesystem.*", "filesystem.read_file", true},
		{"filesystem.*", "shell.exec", false},
		{"*npm install*", "sudo npm install left-pad", true},
		{"*npm install*", "npm run build", false},
	}
	for _, tc := range cases {
		p, err := compilePattern(tc.pattern)
		if err != nil {
			t.Fatalf("compile %q: %v", tc.pattern, err)
		}
		if got := p.match(tc.subject, false); got != tc.want {
			t.Errorf("pattern %q vs %q = %v, want %v", tc.pattern, tc.subject, got, tc.want)
		}
	}
}

func TestPrecedenceIsOrderIndependent(t *testing.T) {
	allowFirst := `
version: "0.1"
defaults: {action: allow}
rules:
  - id: r-allow
    match: {tool: "*"}
    action: allow
  - id: r-ask
    match: {tool: "Bash"}
    action: ask
  - id: r-deny
    match: {args: {command: "*danger*"}}
    action: deny
`
	denyFirst := `
version: "0.1"
defaults: {action: allow}
rules:
  - id: r-deny
    match: {args: {command: "*danger*"}}
    action: deny
  - id: r-ask
    match: {tool: "Bash"}
    action: ask
  - id: r-allow
    match: {tool: "*"}
    action: allow
`
	for name, yaml := range map[string]string{"allow-first": allowFirst, "deny-first": denyFirst} {
		t.Run(name, func(t *testing.T) {
			e := mustEngine(t, yaml, Options{})
			if got := e.Decide(Call{Tool: "Bash", Args: map[string]any{"command": "run danger now"}}); got.Action != ActionDeny || got.RuleID != "r-deny" {
				t.Fatalf("danger call = %+v, want deny/r-deny", got)
			}
			if got := e.Decide(Call{Tool: "Bash", Args: map[string]any{"command": "safe"}}); got.Action != ActionAsk || got.RuleID != "r-ask" {
				t.Fatalf("safe bash call = %+v, want ask/r-ask", got)
			}
			if got := e.Decide(Call{Tool: "Read"}); got.Action != ActionAllow || got.RuleID != "r-allow" {
				t.Fatalf("read call = %+v, want allow/r-allow", got)
			}
		})
	}
}

func TestLogNeverOverrides(t *testing.T) {
	yaml := `
version: "0.1"
defaults: {action: deny}
rules:
  - id: r-log
    match: {tool: "*"}
    action: log
  - id: r-allow
    match: {tool: "Read"}
    action: allow
`
	e := mustEngine(t, yaml, Options{})
	if got := e.Decide(Call{Tool: "Read"}); got.Action != ActionAllow || got.RuleID != "r-allow" {
		t.Fatalf("got %+v, want allow/r-allow", got)
	}
	if got := e.Decide(Call{Tool: "Bash"}); got.Action != ActionLog || got.RuleID != "r-log" {
		t.Fatalf("got %+v, want log/r-log", got)
	}
}

func TestUnknownServer(t *testing.T) {
	withServerRules := `
version: "0.1"
defaults: {action: deny, unknown_server: deny}
rules:
  - id: allow-local
    match: {surface: mcp, server: "local-*"}
    action: allow
`
	withoutServerRules := `
version: "0.1"
defaults: {action: allow}
rules:
  - id: allow-tool
    match: {tool: "Read"}
    action: allow
`
	e := mustEngine(t, withServerRules, Options{})
	got := e.Decide(Call{Surface: "mcp", Server: "remote", Tool: "x"})
	if got.Action != ActionDeny || got.RuleID != "unknown_server" {
		t.Fatalf("unknown server = %+v, want deny/unknown_server", got)
	}
	got = e.Decide(Call{Surface: "mcp", Server: "local-fs", Tool: "x"})
	if got.Action != ActionAllow || got.RuleID != "allow-local" {
		t.Fatalf("known server = %+v, want allow/allow-local", got)
	}
	got = e.Decide(Call{Surface: "hook", Tool: "x"})
	if got.Action != ActionDeny || got.RuleID != "" {
		t.Fatalf("hook call = %+v, want default deny", got)
	}

	e = mustEngine(t, withoutServerRules, Options{})
	got = e.Decide(Call{Surface: "mcp", Server: "remote", Tool: "Bash"})
	if got.Action != ActionAllow || got.RuleID != "" {
		t.Fatalf("no server rules = %+v, want default allow", got)
	}

	unknownAllowed := `
version: "0.1"
defaults: {action: deny, unknown_server: allow}
rules:
  - id: allow-local
    match: {surface: mcp, server: "local-*"}
    action: allow
`
	e = mustEngine(t, unknownAllowed, Options{})
	got = e.Decide(Call{Surface: "mcp", Server: "remote", Tool: "x"})
	if got.Action != ActionAllow || got.RuleID != "unknown_server" {
		t.Fatalf("unknown server allowed = %+v, want allow/unknown_server", got)
	}
}

func TestCanonicalization(t *testing.T) {
	opts := Options{
		Home:      "/home/dev",
		Workspace: "/work",
		EvalSymlinks: func(p string) (string, error) {
			if strings.HasPrefix(p, "/work/link/") {
				return "/home/dev/.ssh/" + strings.TrimPrefix(p, "/work/link/"), nil
			}
			return p, nil
		},
	}
	p, err := compilePattern("**/.ssh/**")
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		value string
		fold  bool
		want  bool
	}{
		{"~/.ssh/id_rsa", false, true},
		{"${WORKSPACE}/.ssh/key", false, true},
		{"/work/link/id_rsa", false, true},
		{`C:\Users\dev\.SSH\id_rsa`, true, true},
		{`C:\Users\dev\.SSH\id_rsa`, false, false},
		{"/etc/passwd", false, false},
	}
	for _, tc := range cases {
		o := opts
		o.CaseFold = tc.fold
		if got := p.matchCandidates(tc.value, o); got != tc.want {
			t.Errorf("matchCandidates(%q, fold=%v) = %v, want %v", tc.value, tc.fold, got, tc.want)
		}
	}
}

func TestArgValueKinds(t *testing.T) {
	yaml := `
version: "0.1"
defaults: {action: allow}
rules:
  - id: deny-any
    match:
      args:
        "*": "*secret*"
    action: deny
  - id: deny-tags
    match:
      args:
        tags: "b"
    action: deny
  - id: deny-retries
    match:
      args:
        retries: "3"
    action: deny
  - id: deny-force
    match:
      args:
        force: "true"
    action: deny
`
	e := mustEngine(t, yaml, Options{})
	if got := e.Decide(Call{Tool: "x", Args: map[string]any{"a": "top secret", "b": "ok"}}); got.Action != ActionDeny || got.RuleID != "deny-any" {
		t.Fatalf("any-key match = %+v, want deny-any", got)
	}
	if got := e.Decide(Call{Tool: "x", Args: map[string]any{"tags": []any{"a", "b"}}}); got.Action != ActionDeny || got.RuleID != "deny-tags" {
		t.Fatalf("list match = %+v, want deny-tags", got)
	}
	if got := e.Decide(Call{Tool: "x", Args: map[string]any{"retries": float64(3)}}); got.Action != ActionDeny || got.RuleID != "deny-retries" {
		t.Fatalf("number match = %+v, want deny-retries", got)
	}
	if got := e.Decide(Call{Tool: "x", Args: map[string]any{"force": true}}); got.Action != ActionDeny || got.RuleID != "deny-force" {
		t.Fatalf("bool match = %+v, want deny-force", got)
	}
	if got := e.Decide(Call{Tool: "x", Args: map[string]any{"x": nil, "y": map[string]any{"k": "plain"}}}); got.Action != ActionAllow || got.RuleID != "" {
		t.Fatalf("nil/map fallthrough = %+v, want default allow", got)
	}
	if got := e.Decide(Call{Tool: "x"}); got.Action != ActionAllow || got.RuleID != "" {
		t.Fatalf("no args = %+v, want default allow", got)
	}
}

func TestMatchesExactArgKey(t *testing.T) {
	yaml := `
version: "0.1"
defaults: {action: allow}
rules:
  - id: deny-command
    match:
      surface: hook
      tool: Bash
      args:
        command: "*rm -rf*"
    action: deny
`
	e := mustEngine(t, yaml, Options{})
	got := e.Decide(Call{Surface: "hook", Tool: "Bash", Args: map[string]any{"command": "rm -rf ./tmp"}})
	if got.Action != ActionDeny || got.RuleID != "deny-command" {
		t.Fatalf("got %+v, want deny-command", got)
	}
	got = e.Decide(Call{Surface: "hook", Tool: "Bash", Args: map[string]any{"other": "rm -rf ./tmp"}})
	if got.Action != ActionAllow {
		t.Fatalf("exact key should not match other keys: %+v", got)
	}
	got = e.Decide(Call{Surface: "hook", Tool: "Read", Args: map[string]any{"command": "rm -rf ./tmp"}})
	if got.Action != ActionAllow {
		t.Fatalf("surface/tool should not match: %+v", got)
	}
}

func TestSurfaceAndServerPatterns(t *testing.T) {
	yaml := `
version: "0.1"
defaults: {action: allow}
rules:
  - id: any-surface
    match: {surface: any, tool: "Read"}
    action: log
  - id: mcp-only
    match: {surface: mcp, tool: "read_file"}
    action: deny
  - id: server-regex
    match: {server: "re:^prod-.*$", tool: "query"}
    action: deny
`
	e := mustEngine(t, yaml, Options{})
	if got := e.Decide(Call{Surface: "hook", Tool: "Read"}); got.Action != ActionLog || got.RuleID != "any-surface" {
		t.Fatalf("any surface = %+v", got)
	}
	if got := e.Decide(Call{Surface: "mcp", Tool: "read_file"}); got.Action != ActionDeny || got.RuleID != "mcp-only" {
		t.Fatalf("mcp only = %+v", got)
	}
	if got := e.Decide(Call{Surface: "hook", Tool: "read_file"}); got.Action != ActionAllow {
		t.Fatalf("hook should not match mcp rule = %+v", got)
	}
	if got := e.Decide(Call{Surface: "mcp", Server: "prod-db", Tool: "query"}); got.Action != ActionDeny || got.RuleID != "server-regex" {
		t.Fatalf("server regex = %+v", got)
	}
}

func TestLoadFile(t *testing.T) {
	dir := specDir(t)
	p, err := LoadFile(filepath.Join(dir, "examples", "default-dev.yml"))
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if p.Name != "default-dev" || len(p.Rules) == 0 {
		t.Fatalf("unexpected policy: %+v", p)
	}
	if _, err := LoadFile(filepath.Join(dir, "missing.yml")); err == nil {
		t.Fatal("expected error for missing file")
	}
	tmp := filepath.Join(t.TempDir(), "p.yml")
	if err := os.WriteFile(tmp, []byte("version: \"0.1\"\ndefaults: {action: deny}\nrules: []\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadFile(tmp); err != nil {
		t.Fatalf("load temp: %v", err)
	}
}

func TestReasonFromWinningRule(t *testing.T) {
	yaml := `
version: "0.1"
defaults: {action: allow}
rules:
  - id: rule-a
    match: {tool: "*"}
    action: ask
    reason: "first ask"
  - id: rule-b
    match: {tool: "*"}
    action: ask
    reason: "second ask"
`
	e := mustEngine(t, yaml, Options{})
	got := e.Decide(Call{Tool: "x"})
	if got.RuleID != "rule-a" || got.Reason != "first ask" {
		t.Fatalf("got %+v, want first rule reason", got)
	}
}
