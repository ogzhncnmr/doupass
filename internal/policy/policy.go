package policy

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"regexp"

	"gopkg.in/yaml.v3"
)

type Action string

const (
	ActionAllow Action = "allow"
	ActionDeny  Action = "deny"
	ActionAsk   Action = "ask"
	ActionLog   Action = "log"
)

var validActions = map[Action]bool{
	ActionAllow: true,
	ActionDeny:  true,
	ActionAsk:   true,
	ActionLog:   true,
}

var ruleIDPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,63}$`)

type Policy struct {
	Version  string   `yaml:"version"`
	Name     string   `yaml:"name"`
	Rules    []Rule   `yaml:"rules"`
	Defaults Defaults `yaml:"defaults"`
	Audit    Audit    `yaml:"audit"`
}

type Rule struct {
	ID     string `yaml:"id"`
	Match  Match  `yaml:"match"`
	Action Action `yaml:"action"`
	Reason string `yaml:"reason"`
}

type Match struct {
	Surface string            `yaml:"surface"`
	Tool    string            `yaml:"tool"`
	Server  string            `yaml:"server"`
	Args    map[string]string `yaml:"args"`
}

func (m Match) isEmpty() bool {
	return m.Surface == "" && m.Tool == "" && m.Server == "" && len(m.Args) == 0
}

type Defaults struct {
	Action            Action `yaml:"action"`
	UnknownServer     Action `yaml:"unknown_server"`
	AskFallback       Action `yaml:"ask_fallback"`
	AskTimeoutSeconds int    `yaml:"ask_timeout_seconds"`
}

type Audit struct {
	Path      string `yaml:"path"`
	HashChain bool   `yaml:"hash_chain"`
}

type Call struct {
	Surface string
	Server  string
	Tool    string
	Args    map[string]any
}

type Decision struct {
	Action Action
	RuleID string
	Reason string
}

type Options struct {
	Home         string
	Workspace    string
	CaseFold     bool
	EvalSymlinks func(string) (string, error)
}

func (a Action) rank() int {
	switch a {
	case ActionDeny:
		return 3
	case ActionAsk:
		return 2
	case ActionAllow:
		return 1
	case ActionLog:
		return 0
	}
	return -1
}

func LoadFile(path string) (*Policy, error) {
	//#nosec G304 -- path is the policy file selected by the local user
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return Parse(data)
}

func Parse(data []byte) (*Policy, error) {
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	var p Policy
	if err := dec.Decode(&p); err != nil {
		return nil, fmt.Errorf("parse policy: %w", err)
	}
	var extra any
	if err := dec.Decode(&extra); err != io.EOF {
		return nil, errors.New("parse policy: multiple YAML documents are not supported")
	}
	if err := p.Validate(); err != nil {
		return nil, err
	}
	return &p, nil
}

func (p *Policy) Validate() error {
	if p.Version != "0.1" {
		return fmt.Errorf("unsupported policy version %q", p.Version)
	}
	if !validActions[p.Defaults.Action] {
		return fmt.Errorf("defaults.action %q is not a valid action", p.Defaults.Action)
	}
	if p.Defaults.UnknownServer != "" && !validActions[p.Defaults.UnknownServer] {
		return fmt.Errorf("defaults.unknown_server %q is not a valid action", p.Defaults.UnknownServer)
	}
	if p.Defaults.AskFallback != "" && p.Defaults.AskFallback != ActionAllow && p.Defaults.AskFallback != ActionDeny {
		return fmt.Errorf("defaults.ask_fallback %q must be allow or deny", p.Defaults.AskFallback)
	}
	seen := make(map[string]bool, len(p.Rules))
	for i := range p.Rules {
		r := &p.Rules[i]
		if !ruleIDPattern.MatchString(r.ID) {
			return fmt.Errorf("rule %d: invalid id %q", i, r.ID)
		}
		if seen[r.ID] {
			return fmt.Errorf("duplicate rule id %q", r.ID)
		}
		seen[r.ID] = true
		if !validActions[r.Action] {
			return fmt.Errorf("rule %q: invalid action %q", r.ID, r.Action)
		}
		if r.Match.isEmpty() {
			return fmt.Errorf("rule %q: match must not be empty", r.ID)
		}
		switch r.Match.Surface {
		case "", "any", "hook", "mcp":
		default:
			return fmt.Errorf("rule %q: invalid surface %q", r.ID, r.Match.Surface)
		}
		if err := validateRulePatterns(r); err != nil {
			return err
		}
	}
	return nil
}

func validateRulePatterns(r *Rule) error {
	check := func(field, pat string) error {
		if pat == "" {
			return nil
		}
		if _, err := compilePattern(pat); err != nil {
			return fmt.Errorf("rule %q: %s: %w", r.ID, field, err)
		}
		return nil
	}
	if err := check("tool", r.Match.Tool); err != nil {
		return err
	}
	if err := check("server", r.Match.Server); err != nil {
		return err
	}
	names := make([]string, 0, len(r.Match.Args))
	for name := range r.Match.Args {
		names = append(names, name)
	}
	sortStrings(names)
	for _, name := range names {
		if err := check("args["+name+"]", r.Match.Args[name]); err != nil {
			return err
		}
	}
	return nil
}
