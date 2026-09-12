package policy

import (
	"fmt"
	"path"
	"path/filepath"
	"regexp"
	"strings"
)

type pattern struct {
	raw string
	re  *regexp.Regexp
	reF *regexp.Regexp
}

func compilePattern(raw string) (*pattern, error) {
	expr, err := patternExpr(raw)
	if err != nil {
		return nil, err
	}
	p := &pattern{raw: raw}
	p.re, err = regexp.Compile(expr)
	if err != nil {
		return nil, fmt.Errorf("invalid pattern %q: %w", raw, err)
	}
	if foldExpr, err := regexp.Compile("(?i)" + expr); err == nil {
		p.reF = foldExpr
	}
	return p, nil
}

func patternExpr(raw string) (string, error) {
	if strings.HasPrefix(raw, "re:") {
		expr := strings.TrimPrefix(raw, "re:")
		if expr == "" {
			return "", fmt.Errorf("empty regex pattern")
		}
		return "^(?:" + expr + ")$", nil
	}
	var b strings.Builder
	b.WriteString("(?s)^")
	for i := 0; i < len(raw); i++ {
		switch c := raw[i]; c {
		case '*':
			for i+1 < len(raw) && raw[i+1] == '*' {
				i++
			}
			b.WriteString(".*")
		case '?':
			b.WriteString(".")
		case '\\':
			if i+1 < len(raw) {
				i++
				b.WriteString(regexp.QuoteMeta(string(raw[i])))
			} else {
				b.WriteString(regexp.QuoteMeta("\\"))
			}
		default:
			b.WriteString(regexp.QuoteMeta(string(c)))
		}
	}
	b.WriteString("$")
	return b.String(), nil
}

func (p *pattern) match(s string, fold bool) bool {
	if p == nil {
		return false
	}
	if fold && p.reF != nil {
		return p.reF.MatchString(s)
	}
	return p.re.MatchString(s)
}

func (p *pattern) matchCandidates(s string, opts Options) bool {
	if p.match(s, false) {
		return true
	}
	expanded := expandVars(s, opts)
	if expanded != s && p.match(expanded, false) {
		return true
	}
	if looksLikePath(expanded) {
		canonical := canonicalizePath(expanded, opts)
		if p.match(canonical, opts.CaseFold) {
			return true
		}
	}
	return false
}

func expandVars(s string, opts Options) string {
	s = strings.ReplaceAll(s, "${HOME}", opts.Home)
	s = strings.ReplaceAll(s, "${WORKSPACE}", opts.Workspace)
	return s
}

func canonicalizePath(s string, opts Options) string {
	s = expandVars(s, opts)
	s = strings.ReplaceAll(s, "\\", "/")
	if opts.Home != "" {
		switch {
		case s == "~":
			s = opts.Home
		case strings.HasPrefix(s, "~/"):
			s = path.Join(opts.Home, s[2:])
		}
	}
	if !isAbsolute(s) {
		s = path.Join(opts.Workspace, s)
	}
	s = path.Clean(s)
	if opts.EvalSymlinks != nil {
		if resolved, err := opts.EvalSymlinks(s); err == nil {
			s = resolved
		}
	}
	return s
}

// OSResolver returns an EvalSymlinks implementation that resolves paths on the
// local filesystem. Input and output use forward slashes.
func OSResolver() func(string) (string, error) {
	return func(p string) (string, error) {
		resolved, err := filepath.EvalSymlinks(filepath.FromSlash(p))
		if err != nil {
			return "", err
		}
		return filepath.ToSlash(resolved), nil
	}
}

func isAbsolute(s string) bool {
	if strings.HasPrefix(s, "/") {
		return true
	}
	return isDrivePath(s)
}

func isDrivePath(s string) bool {
	return len(s) >= 2 && s[1] == ':' &&
		((s[0] >= 'a' && s[0] <= 'z') || (s[0] >= 'A' && s[0] <= 'Z'))
}

func looksLikePath(s string) bool {
	if s == "" {
		return false
	}
	if s[0] == '/' || s[0] == '~' {
		return true
	}
	if strings.HasPrefix(s, "./") || strings.HasPrefix(s, "../") ||
		strings.HasPrefix(s, ".\\") || strings.HasPrefix(s, "..\\") {
		return true
	}
	return isDrivePath(s)
}
