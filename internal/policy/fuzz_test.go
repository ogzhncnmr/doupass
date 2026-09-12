package policy

import (
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"testing"
)

const (
	maxFuzzParseBytes   = 64 << 10
	maxFuzzPatternBytes = 4096
)

func FuzzParse(f *testing.F) {
	for _, data := range fixturePolicies(f) {
		f.Add(data)
	}
	for _, data := range [][]byte{
		nil,
		{},
		[]byte(" "),
		[]byte("version: \"0.1\"\ndefaults: {action: allow}\nrules: []\n"),
		[]byte("version: \"9.9\"\ndefaults: {action: allow}\nrules: []\n"),
		[]byte("version: \"0.1\"\ndefaults: {action: allow}\nrules: []\nbogus: true\n"),
		[]byte("version: \"0.1\"\nrules: []\n"),
		[]byte("version: \"0.1\"\ndefaults: {action: allow}\nrules:\n  - id: r\n    match: {tool: \"re:(\"}\n    action: deny\n"),
		[]byte("version: \"0.1\"\ndefaults: {action: allow}\nrules: []\n---\nversion: \"0.1\"\n"),
		[]byte("{]"),
		[]byte("\x00\x01\xff"),
	} {
		f.Add(data)
	}

	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) > maxFuzzParseBytes {
			t.Skip()
		}
		_, _ = Parse(data)
	})
}

func FuzzPattern(f *testing.F) {
	for _, pat := range fixturePatterns(f) {
		f.Add(pat)
	}
	for _, pat := range []string{
		"",
		"*",
		"**",
		"?",
		`\`,
		`\*`,
		"**/",
		"/**",
		"/**/",
		"**/.ssh/**",
		"**/.env*",
		"${WORKSPACE}/**",
		"${HOME}/.ssh/**",
		"vendor/**/testdata/*.json",
		"re:",
		"re:(",
		"re:.*",
		"re:^(Read|Glob)$",
		"re:^(go test|npm test)\\b.*",
		"*npm install*",
		"*rm -rf /*",
		"filesystem.*",
		"?.txt",
		"a[b",
		"\x00",
	} {
		f.Add(pat)
	}

	f.Fuzz(func(t *testing.T, raw string) {
		if len(raw) > maxFuzzPatternBytes {
			t.Skip()
		}
		expr, err := patternExpr(raw)
		if err != nil {
			return
		}
		_, _ = regexp.Compile(expr)
		_, _ = compilePattern(raw)
	})
}

func fixturePolicies(tb testing.TB) [][]byte {
	tb.Helper()
	root := repoRoot(tb)
	globs := []string{
		filepath.Join(root, "spec", "examples", "*.yml"),
		filepath.Join(root, "rules", "starter.yml"),
		filepath.Join(root, "rules", "packs", "*.yml"),
		filepath.Join(root, "internal", "cli", "presets", "*.yml"),
	}
	var paths []string
	for _, g := range globs {
		matches, err := filepath.Glob(g)
		if err != nil {
			tb.Fatal(err)
		}
		paths = append(paths, matches...)
	}
	if len(paths) == 0 {
		tb.Fatal("no policy fixtures found to seed fuzz corpus")
	}
	out := make([][]byte, 0, len(paths))
	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			tb.Fatal(err)
		}
		out = append(out, data)
	}
	return out
}

func fixturePatterns(tb testing.TB) []string {
	tb.Helper()
	seen := make(map[string]struct{})
	var out []string
	add := func(s string) {
		if s == "" {
			return
		}
		if _, ok := seen[s]; ok {
			return
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}
	for _, data := range fixturePolicies(tb) {
		p, err := Parse(data)
		if err != nil {
			tb.Fatalf("seed policy parse: %v", err)
		}
		for _, r := range p.Rules {
			add(r.Match.Tool)
			add(r.Match.Server)
			for k, v := range r.Match.Args {
				add(k)
				add(v)
			}
		}
	}
	if len(out) == 0 {
		tb.Fatal("no patterns found in spec fixtures")
	}
	return out
}

func repoRoot(tb testing.TB) string {
	tb.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		tb.Fatal("cannot locate test file")
	}
	return filepath.Join(filepath.Dir(file), "..", "..")
}
