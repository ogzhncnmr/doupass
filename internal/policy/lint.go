package policy

import (
	"fmt"
	"sort"
	"strings"
)

type LintIssue struct {
	Level   string `json:"level"`
	Rule    string `json:"rule,omitempty"`
	Message string `json:"message"`
}

func Lint(p *Policy) []LintIssue {
	var issues []LintIssue
	seen := make(map[string]string)
	for i := range p.Rules {
		r := &p.Rules[i]
		sig := ruleSignature(r)
		if prev, ok := seen[sig]; ok {
			issues = append(issues, LintIssue{
				Level:   "warning",
				Rule:    r.ID,
				Message: fmt.Sprintf("duplicates rule %q (same match and action)", prev),
			})
		} else {
			seen[sig] = r.ID
		}
		if (r.Action == ActionDeny || r.Action == ActionAsk) && r.Reason == "" {
			issues = append(issues, LintIssue{
				Level:   "warning",
				Rule:    r.ID,
				Message: "deny/ask rule has no reason; neither the agent nor the audit log can explain it",
			})
		}
		keys := make([]string, 0, len(r.Match.Args))
		for key := range r.Match.Args {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			if isBroadPattern(r.Match.Args[key]) {
				issues = append(issues, LintIssue{
					Level:   "warning",
					Rule:    r.ID,
					Message: fmt.Sprintf("args[%q] pattern %q matches every string value", key, r.Match.Args[key]),
				})
			}
		}
		if r.Action == ActionAsk && r.Match.Surface == "" && r.Match.Tool == "" && r.Match.Server == "" {
			issues = append(issues, LintIssue{
				Level:   "warning",
				Rule:    r.ID,
				Message: "ask rule has no surface/tool/server constraint and applies to every surface",
			})
		}
		if r.Action == ActionAllow && r.Match.Tool == "*" && r.Match.Surface == "" &&
			r.Match.Server == "" && len(r.Match.Args) == 0 {
			issues = append(issues, LintIssue{
				Level:   "warning",
				Rule:    r.ID,
				Message: "allow rule matches every tool on every surface",
			})
		}
	}
	return issues
}

func isBroadPattern(pat string) bool {
	switch pat {
	case "*", "**", "re:.*", "re:.+":
		return true
	}
	return false
}

func ruleSignature(r *Rule) string {
	keys := make([]string, 0, len(r.Match.Args))
	for key := range r.Match.Args {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	var args strings.Builder
	for _, key := range keys {
		args.WriteString(key)
		args.WriteString("=")
		args.WriteString(r.Match.Args[key])
		args.WriteString(";")
	}
	return strings.Join([]string{
		r.Match.Surface, r.Match.Tool, r.Match.Server, args.String(), string(r.Action),
	}, "|")
}
