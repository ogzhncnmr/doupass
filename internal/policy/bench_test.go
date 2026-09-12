package policy

import "testing"

const benchPolicy = `
version: "0.1"
name: bench
defaults:
  action: allow
  ask_fallback: deny
rules:
  - id: block-ssh-credentials
    match:
      args:
        "*": "**/.ssh/**"
    action: deny
    reason: "off-limits"
  - id: block-env-files
    match:
      args:
        "*": "**/.env*"
    action: deny
    reason: "secrets"
  - id: ask-npm-install
    match:
      surface: hook
      tool: Bash
      args:
        command: "*npm install*"
    action: ask
    reason: "lifecycle scripts"
  - id: ask-rm-recursive
    match:
      surface: hook
      tool: Bash
      args:
        command: "*rm -rf*"
    action: ask
    reason: "recursive delete"
  - id: allow-workspace
    match:
      args:
        "*": "${WORKSPACE}/**"
    action: allow
    reason: "workspace"
`

func BenchmarkDecide(b *testing.B) {
	p, err := Parse([]byte(benchPolicy))
	if err != nil {
		b.Fatal(err)
	}
	engine, err := New(p, Options{Home: "/home/dev", Workspace: "/work"})
	if err != nil {
		b.Fatal(err)
	}
	call := Call{Surface: "hook", Tool: "Bash", Args: map[string]any{"command": "npm run build"}}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = engine.Decide(call)
	}
}
