package cli

import (
	"bytes"
	"strings"
	"testing"
	"unicode/utf8"
)

func TestTruncateMiddle(t *testing.T) {
	long := `C:\Users\OGZHNCNMR\AppData\Roaming\Code\User\globalStorage\saoudrizwan.claude-dev\settings\cline_mcp_settings.json`
	got := truncateMiddle(long, 56)
	if utf8.RuneCountInString(got) != 56 {
		t.Fatalf("length = %d, want 56: %q", utf8.RuneCountInString(got), got)
	}
	if !strings.HasPrefix(got, `C:\Users`) {
		t.Fatalf("head lost: %q", got)
	}
	if !strings.HasSuffix(got, "_mcp_settings.json") {
		t.Fatalf("tail lost: %q", got)
	}
	if !strings.Contains(got, "…") {
		t.Fatalf("missing ellipsis: %q", got)
	}
	if short := truncateMiddle("C:\\x\\y.json", 56); short != "C:\\x\\y.json" {
		t.Fatalf("short path altered: %q", short)
	}
	if got := truncateMiddle("abcdefghij", 0); got != "" {
		t.Fatalf("max=0 should be empty, got %q", got)
	}
}

func TestPlural(t *testing.T) {
	cases := []struct {
		n    int
		word string
		want string
	}{
		{1, "rule", "1 rule"},
		{0, "rule", "0 rules"},
		{18, "rule", "18 rules"},
		{1, "entry", "1 entry"},
		{5, "entry", "5 entries"},
		{2, "lint issue", "2 lint issues"},
	}
	for _, tc := range cases {
		if got := plural(tc.n, tc.word); got != tc.want {
			t.Errorf("plural(%d, %q) = %q, want %q", tc.n, tc.word, got, tc.want)
		}
	}
}

func TestPrintSetupReportDryRun(t *testing.T) {
	var out bytes.Buffer
	rows := []setupRow{
		{name: "claude-code", result: "would register the PreToolUse hook", path: `C:\u\.claude\settings.json`, state: "plan"},
		{name: "cursor", result: "would wrap local MCP servers", path: `C:\u\.cursor\mcp.json`, state: "plan"},
		{name: "cline (Code)", result: "would wrap local MCP servers", path: `C:\u\AppData\Roaming\Code\User\globalStorage\saoudrizwan.claude-dev\settings\cline_mcp_settings.json`, state: "plan"},
	}
	printSetupReport(&out, true, rows)
	text := out.String()
	for _, want := range []string{
		"doupass setup — dry run (no files will be changed)",
		"TOOL", "ACTION", "CONFIG",
		"would register the PreToolUse hook",
		"would wrap local MCP servers",
		"3 tools detected",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("dry-run report missing %q:\n%s", want, text)
		}
	}
	if strings.Contains(text, "saoudrizwan.claude-dev\\settings\\cline_mcp_settings.json") && !strings.Contains(text, "…") {
		t.Fatalf("long path not truncated:\n%s", text)
	}
	if strings.Contains(text, "\x1b[") {
		t.Fatalf("ANSI escapes leaked into non-styled output:\n%q", text)
	}

	lines := strings.Split(strings.TrimRight(text, "\n"), "\n")
	var toolLines []string
	for _, l := range lines {
		if strings.Contains(l, "would") {
			toolLines = append(toolLines, l)
		}
	}
	cfgCol := strings.Index(toolLines[0], `C:\u\.claude\settings.json`)
	if cfgCol < 0 {
		t.Fatalf("config column missing: %q", toolLines[0])
	}
	for _, l := range toolLines {
		if !strings.HasPrefix(l[cfgCol:], `C:\u\`) {
			t.Fatalf("config column misaligned: %q", l)
		}
	}
}

func TestPrintSetupReportApply(t *testing.T) {
	var out bytes.Buffer
	rows := []setupRow{
		{name: "claude-code", result: "hook installed", path: `C:\u\.claude\settings.json`, state: "done"},
		{name: "cursor", result: "wrapped 1 MCP server(s)", path: `C:\u\.cursor\mcp.json`, state: "done"},
		{name: "kiro", result: "no local MCP servers to wrap", path: `C:\u\.kiro\settings\mcp.json`, state: "skip"},
		{name: "windsurf", result: "wrap failed", path: `C:\u\.codeium\windsurf\mcp_config.json`, state: "fail", detail: "unexpected end of JSON input"},
	}
	printSetupReport(&out, false, rows)
	text := out.String()
	for _, want := range []string{
		"doupass setup\n",
		"RESULT",
		"hook installed",
		"wrapped 1 MCP server(s)",
		"no local MCP servers to wrap",
		"wrap failed",
		"unexpected end of JSON input",
		"Restart any tool that changed.",
		"<config>.doupass.bak",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("apply report missing %q:\n%s", want, text)
		}
	}
	if strings.Contains(text, "ACTION") {
		t.Fatalf("apply report should use RESULT header:\n%s", text)
	}
}

func TestPrintSetupReportNoTools(t *testing.T) {
	var out bytes.Buffer
	printSetupReport(&out, true, nil)
	if !strings.Contains(out.String(), "no supported tools detected") {
		t.Fatalf("out = %q", out.String())
	}
}
