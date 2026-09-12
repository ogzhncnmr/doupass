package policy

import (
	"encoding/json"
	"fmt"
)

type Engine struct {
	Policy         *Policy
	Rules          []Rule
	opts           Options
	rules          []compiledRule
	hasServerRules bool
}

type compiledRule struct {
	rule   Rule
	tool   *pattern
	server *pattern
	args   []argPattern
}

type argPattern struct {
	name string
	pat  *pattern
}

func New(p *Policy, opts Options) (*Engine, error) {
	e := &Engine{Policy: p, Rules: p.Rules, opts: opts}
	for i := range p.Rules {
		cr, err := compileRule(&p.Rules[i], opts)
		if err != nil {
			return nil, err
		}
		if cr.server != nil {
			e.hasServerRules = true
		}
		e.rules = append(e.rules, cr)
	}
	return e, nil
}

func compileRule(r *Rule, opts Options) (compiledRule, error) {
	cr := compiledRule{rule: *r}
	var err error
	if r.Match.Tool != "" {
		cr.tool, err = compilePattern(expandVars(r.Match.Tool, opts))
		if err != nil {
			return cr, fmt.Errorf("rule %q: tool pattern: %w", r.ID, err)
		}
	}
	if r.Match.Server != "" {
		cr.server, err = compilePattern(expandVars(r.Match.Server, opts))
		if err != nil {
			return cr, fmt.Errorf("rule %q: server pattern: %w", r.ID, err)
		}
	}
	names := make([]string, 0, len(r.Match.Args))
	for name := range r.Match.Args {
		names = append(names, name)
	}
	sortStrings(names)
	for _, name := range names {
		pat, err := compilePattern(expandVars(r.Match.Args[name], opts))
		if err != nil {
			return cr, fmt.Errorf("rule %q: args[%q]: %w", r.ID, name, err)
		}
		cr.args = append(cr.args, argPattern{name: name, pat: pat})
	}
	return cr, nil
}

func sortStrings(s []string) {
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && s[j] < s[j-1]; j-- {
			s[j], s[j-1] = s[j-1], s[j]
		}
	}
}

func (e *Engine) Decide(call Call) Decision {
	if call.Surface == "mcp" && call.Server != "" && e.hasServerRules {
		covered := false
		for i := range e.rules {
			if e.rules[i].server != nil && e.rules[i].server.match(call.Server, false) {
				covered = true
				break
			}
		}
		if !covered {
			action := e.Policy.Defaults.UnknownServer
			if action == "" {
				action = ActionDeny
			}
			return Decision{
				Action: action,
				RuleID: "unknown_server",
				Reason: "server is not covered by any server-scoped rule",
			}
		}
	}

	var best *compiledRule
	for i := range e.rules {
		if !e.rules[i].matches(call, e.opts) {
			continue
		}
		if best == nil || e.rules[i].rule.Action.rank() > best.rule.Action.rank() {
			best = &e.rules[i]
		}
	}
	if best != nil {
		return Decision{Action: best.rule.Action, RuleID: best.rule.ID, Reason: best.rule.Reason}
	}
	return Decision{Action: e.Policy.Defaults.Action}
}

func (r *compiledRule) matches(call Call, opts Options) bool {
	if r.rule.Match.Surface != "" && r.rule.Match.Surface != "any" && r.rule.Match.Surface != call.Surface {
		return false
	}
	if r.tool != nil && !r.tool.match(call.Tool, false) {
		return false
	}
	if r.server != nil && !r.server.match(call.Server, false) {
		return false
	}
	for _, ap := range r.args {
		if !ap.matches(call.Args, opts) {
			return false
		}
	}
	return true
}

func (ap argPattern) matches(args map[string]any, opts Options) bool {
	if ap.name == "*" {
		for _, v := range args {
			if anyValueMatches(ap.pat, v, opts) {
				return true
			}
		}
		return false
	}
	v, ok := args[ap.name]
	if !ok {
		return false
	}
	return anyValueMatches(ap.pat, v, opts)
}

func anyValueMatches(p *pattern, v any, opts Options) bool {
	switch x := v.(type) {
	case nil:
		return false
	case string:
		return p.matchCandidates(x, opts)
	case []any:
		for _, elem := range x {
			if anyValueMatches(p, elem, opts) {
				return true
			}
		}
		return false
	default:
		data, err := json.Marshal(x)
		if err != nil {
			return false
		}
		return p.matchCandidates(string(data), opts)
	}
}
