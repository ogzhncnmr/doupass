package hook

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/ogzhncnmr/doupass/internal/policy"
)

type transcriptFile struct {
	Cases []struct {
		Name   string `yaml:"name"`
		Policy string `yaml:"policy"`
		Input  Input  `yaml:"input"`
		Expect struct {
			Action   string `yaml:"action"`
			Rule     string `yaml:"rule"`
			Decision string `yaml:"decision"`
		} `yaml:"expect"`
	} `yaml:"cases"`
}

func repoRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot locate test file")
	}
	return filepath.Join(filepath.Dir(file), "..", "..")
}

func TestTranscripts(t *testing.T) {
	root := repoRoot(t)
	data, err := os.ReadFile(filepath.Join(root, "internal", "hook", "testdata", "transcripts.yml"))
	if err != nil {
		t.Fatalf("read transcripts: %v", err)
	}
	var tf transcriptFile
	if err := yaml.Unmarshal(data, &tf); err != nil {
		t.Fatalf("parse transcripts: %v", err)
	}
	if len(tf.Cases) == 0 {
		t.Fatal("no transcript cases")
	}
	for _, tc := range tf.Cases {
		t.Run(tc.Name, func(t *testing.T) {
			p, err := policy.LoadFile(filepath.Join(root, tc.Policy))
			if err != nil {
				t.Fatalf("load policy: %v", err)
			}
			engine, err := policy.New(p, policy.Options{Home: "/home/dev", Workspace: "/work"})
			if err != nil {
				t.Fatalf("engine: %v", err)
			}
			out, dec := Decide(engine, tc.Input)
			if string(dec.Action) != tc.Expect.Action {
				t.Fatalf("action = %q, want %q (rule %q)", dec.Action, tc.Expect.Action, dec.RuleID)
			}
			if dec.RuleID != tc.Expect.Rule {
				t.Fatalf("rule = %q, want %q", dec.RuleID, tc.Expect.Rule)
			}
			gotDecision := ""
			if out.HookSpecificOutput != nil {
				gotDecision = out.HookSpecificOutput.PermissionDecision
			}
			if gotDecision != tc.Expect.Decision {
				t.Fatalf("hook decision = %q, want %q", gotDecision, tc.Expect.Decision)
			}
		})
	}
}

func TestParseInputInvalid(t *testing.T) {
	if _, err := ParseInput([]byte("not json")); err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestAskOutputHasReason(t *testing.T) {
	p, err := policy.Parse([]byte(`
version: "0.1"
defaults: {action: allow}
rules:
  - id: ask-x
    match: {tool: "Bash"}
    action: ask
`))
	if err != nil {
		t.Fatal(err)
	}
	engine, err := policy.New(p, policy.Options{})
	if err != nil {
		t.Fatal(err)
	}
	out, _ := Decide(engine, Input{ToolName: "Bash", ToolInput: map[string]any{"command": "ls"}})
	if out.HookSpecificOutput == nil || out.HookSpecificOutput.PermissionDecision != "ask" {
		t.Fatalf("unexpected output: %+v", out)
	}
	if out.HookSpecificOutput.PermissionDecisionReason != "blocked by doupass policy" {
		t.Fatalf("reason = %q", out.HookSpecificOutput.PermissionDecisionReason)
	}
}
