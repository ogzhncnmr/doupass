package policy

import (
	"strings"
	"testing"
)

func lintPolicy(t *testing.T, yaml string) []LintIssue {
	t.Helper()
	return Lint(mustParse(t, yaml))
}

func TestLintCleanPolicy(t *testing.T) {
	issues := lintPolicy(t, `
version: "0.1"
defaults: {action: allow}
rules:
  - id: block-ssh
    match:
      args: {"*": "**/.ssh/**"}
    action: deny
    reason: "off-limits"
`)
	if len(issues) != 0 {
		t.Fatalf("issues = %+v, want none", issues)
	}
}

func TestLintDuplicateRule(t *testing.T) {
	issues := lintPolicy(t, `
version: "0.1"
defaults: {action: allow}
rules:
  - id: one
    match: {tool: Bash}
    action: deny
    reason: a
  - id: two
    match: {tool: Bash}
    action: deny
    reason: b
`)
	if !hasIssue(issues, "two", "duplicates") {
		t.Fatalf("issues = %+v", issues)
	}
}

func TestLintMissingReason(t *testing.T) {
	issues := lintPolicy(t, `
version: "0.1"
defaults: {action: allow}
rules:
  - id: quiet-deny
    match: {tool: Bash}
    action: deny
`)
	if !hasIssue(issues, "quiet-deny", "no reason") {
		t.Fatalf("issues = %+v", issues)
	}
}

func TestLintBroadPattern(t *testing.T) {
	issues := lintPolicy(t, `
version: "0.1"
defaults: {action: allow}
rules:
  - id: broad
    match:
      args: {"*": "*"}
    action: deny
    reason: x
`)
	if !hasIssue(issues, "broad", "every string value") {
		t.Fatalf("issues = %+v", issues)
	}
}

func TestLintUnscopedAsk(t *testing.T) {
	issues := lintPolicy(t, `
version: "0.1"
defaults: {action: allow}
rules:
  - id: ask-all
    match:
      args: {"*": "*secret*"}
    action: ask
    reason: x
`)
	if !hasIssue(issues, "ask-all", "every surface") {
		t.Fatalf("issues = %+v", issues)
	}
}

func TestLintBroadAllow(t *testing.T) {
	issues := lintPolicy(t, `
version: "0.1"
defaults: {action: deny}
rules:
  - id: allow-all-tools
    match: {tool: "*"}
    action: allow
    reason: x
`)
	if !hasIssue(issues, "allow-all-tools", "every tool") {
		t.Fatalf("issues = %+v", issues)
	}
}

func hasIssue(issues []LintIssue, rule, substr string) bool {
	for _, is := range issues {
		if is.Rule == rule && strings.Contains(is.Message, substr) {
			return true
		}
	}
	return false
}
