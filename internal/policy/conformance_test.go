package policy

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

type conformanceFile struct {
	Vars  map[string]string `yaml:"vars"`
	Cases []conformanceCase `yaml:"cases"`
}

type conformanceCase struct {
	Name     string            `yaml:"name"`
	Policy   string            `yaml:"policy"`
	CaseFold bool              `yaml:"case_fold"`
	Symlinks map[string]string `yaml:"symlinks"`
	Call     struct {
		Surface string            `yaml:"surface"`
		Server  string            `yaml:"server"`
		Tool    string            `yaml:"tool"`
		Args    map[string]string `yaml:"args"`
	} `yaml:"call"`
	Expect struct {
		Action string `yaml:"action"`
		Rule   string `yaml:"rule"`
	} `yaml:"expect"`
}

func specDir(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot locate test file")
	}
	return filepath.Join(filepath.Dir(file), "..", "..", "spec")
}

func TestConformance(t *testing.T) {
	dir := specDir(t)
	data, err := os.ReadFile(filepath.Join(dir, "fixtures", "conformance.yml"))
	if err != nil {
		t.Fatalf("read fixtures: %v", err)
	}
	var conf conformanceFile
	if err := yaml.Unmarshal(data, &conf); err != nil {
		t.Fatalf("parse fixtures: %v", err)
	}
	if len(conf.Cases) == 0 {
		t.Fatal("no conformance cases found")
	}
	for _, tc := range conf.Cases {
		t.Run(tc.Name, func(t *testing.T) {
			p, err := LoadFile(filepath.Join(dir, "fixtures", tc.Policy))
			if err != nil {
				t.Fatalf("load policy: %v", err)
			}
			opts := Options{
				Home:         conf.Vars["HOME"],
				Workspace:    conf.Vars["WORKSPACE"],
				CaseFold:     tc.CaseFold,
				EvalSymlinks: fakeResolver(tc.Symlinks, conf.Vars),
			}
			engine, err := New(p, opts)
			if err != nil {
				t.Fatalf("compile policy: %v", err)
			}
			call := Call{Surface: tc.Call.Surface, Server: tc.Call.Server, Tool: tc.Call.Tool}
			if len(tc.Call.Args) > 0 {
				call.Args = make(map[string]any, len(tc.Call.Args))
				for k, v := range tc.Call.Args {
					call.Args[k] = v
				}
			}
			got := engine.Decide(call)
			if string(got.Action) != tc.Expect.Action {
				t.Fatalf("action = %q, want %q (rule %q, reason %q)", got.Action, tc.Expect.Action, got.RuleID, got.Reason)
			}
			if got.RuleID != tc.Expect.Rule {
				t.Fatalf("rule = %q, want %q", got.RuleID, tc.Expect.Rule)
			}
		})
	}
}

func fakeResolver(symlinks map[string]string, vars map[string]string) func(string) (string, error) {
	if len(symlinks) == 0 {
		return nil
	}
	base := Options{Home: vars["HOME"], Workspace: vars["WORKSPACE"]}
	resolved := make(map[string]string, len(symlinks))
	for k, v := range symlinks {
		resolved[expandVars(k, base)] = expandVars(v, base)
	}
	return func(p string) (string, error) {
		for from, to := range resolved {
			if p == from {
				return to, nil
			}
			if strings.HasPrefix(p, from+"/") {
				return to + strings.TrimPrefix(p, from), nil
			}
		}
		return p, nil
	}
}
