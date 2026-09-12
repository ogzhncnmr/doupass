package cli

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"unicode/utf8"

	"github.com/ogzhncnmr/doupass/internal/audit"
	"github.com/ogzhncnmr/doupass/internal/claude"
	"github.com/ogzhncnmr/doupass/internal/codex"
	"github.com/ogzhncnmr/doupass/internal/opencode"
	"github.com/ogzhncnmr/doupass/internal/policy"
	"github.com/spf13/cobra"
)

const doctorLabelWidth = len("integrations")

func newDoctorCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "doctor",
		Short: "Show the current state: policy, audit log, and integrations",
		RunE: func(cmd *cobra.Command, _ []string) error {
			out := cmd.OutOrStdout()
			problems := 0
			fmt.Fprintf(out, "doupass doctor — %s (%s/%s)\n\n", ResolvedVersion(), runtime.GOOS, runtime.GOARCH)

			auditFile := filepath.Join(homeDir(), ".doupass", "audit.jsonl")
			problems += doctorPolicy(out, &auditFile)
			problems += doctorAudit(out, auditFile)
			active := doctorIntegrations(out)

			fmt.Fprintf(out, "\n%s\n", doctorSummary(active, problems))
			if problems > 0 {
				return fmt.Errorf("%d problem(s) found", problems)
			}
			return nil
		},
	}
}

func doctorPolicy(out io.Writer, auditFile *string) int {
	policyPath, err := findPolicyFile("")
	if err != nil {
		fmt.Fprintf(out, "  %s  %s  policy file not found — run doupass init, or doupass setup\n", padRight("policy", doctorLabelWidth), stateGlyph("fail"))
		return 1
	}
	engine, err := loadEngine(policyPath)
	if err != nil {
		fmt.Fprintf(out, "  %s  %s  invalid policy: %s\n", padRight("policy", doctorLabelWidth), stateGlyph("fail"), truncateMiddle(policyPath, 52))
		fmt.Fprintf(out, "  %s  %v\n", strings.Repeat(" ", doctorLabelWidth), failStyle.Render(err.Error()))
		return 1
	}
	issues := policy.Lint(engine.Policy)
	name := engine.Policy.Name
	if name == "" {
		name = "(unnamed)"
	}
	summary := fmt.Sprintf("%s · %s · %s · %s",
		truncateMiddle(policyPath, 52), name, plural(len(engine.Policy.Rules), "rule"), plural(len(issues), "lint issue"))
	fmt.Fprintf(out, "  %s  %s  %s\n", padRight("policy", doctorLabelWidth), stateGlyph("ok"), summary)
	for _, is := range issues {
		fmt.Fprintf(out, "  %s  %s\n", strings.Repeat(" ", doctorLabelWidth), dimStyle.Render("warning: "+is.Rule+": "+is.Message))
	}
	if engine.Policy.Audit.Path != "" {
		*auditFile = expandHome(engine.Policy.Audit.Path)
	}
	return 0
}

func doctorAudit(out io.Writer, auditFile string) int {
	if _, statErr := os.Stat(auditFile); os.IsNotExist(statErr) {
		fmt.Fprintf(out, "  %s  %s  no entries yet — %s\n", padRight("audit", doctorLabelWidth), stateGlyph("skip"), dimStyle.Render(truncateMiddle(auditFile, 52)))
		return 0
	}
	res, verifyErr := audit.Verify(auditFile)
	if verifyErr != nil {
		fmt.Fprintf(out, "  %s  %s  TAMPERED OR CORRUPT: %s\n", padRight("audit", doctorLabelWidth), stateGlyph("fail"), truncateMiddle(auditFile, 52))
		fmt.Fprintf(out, "  %s  %v\n", strings.Repeat(" ", doctorLabelWidth), failStyle.Render(verifyErr.Error()))
		return 1
	}
	fmt.Fprintf(out, "  %s  %s  %s · hash chain OK\n", padRight("audit", doctorLabelWidth), stateGlyph("ok"), plural(res.Entries, "entry"))
	return 0
}

func doctorIntegrations(out io.Writer) int {
	active := 0
	home := homeDir()

	type integrationRow struct {
		name   string
		state  string
		detail string
	}
	var rows []integrationRow
	claudePath := filepath.Join(home, ".claude", "settings.json")
	if fileExists(claudePath) {
		if claude.HasHook(claudePath) {
			rows = append(rows, integrationRow{"claude-code", "ok", "hook installed"})
			active++
		} else {
			rows = append(rows, integrationRow{"claude-code", "skip", "settings found, hook not installed (run: doupass install claude)"})
		}
	}
	pluginPath := filepath.Join(home, ".config", "opencode", "plugin", "doupass.js")
	if opencode.PluginInstalled(pluginPath) {
		rows = append(rows, integrationRow{"opencode", "ok", "native-tool plugin installed"})
		active++
	}
	for _, tgt := range setupTargets() {
		if tgt.Kind == "claude" || !fileExists(tgt.Path) {
			continue
		}
		switch tgt.Kind {
		case "codex":
			if codex.HasWrappers(tgt.Path) {
				rows = append(rows, integrationRow{tgt.Name, "ok", "MCP servers wrapped"})
				active++
			} else {
				rows = append(rows, integrationRow{tgt.Name, "skip", "config found, local MCP servers not wrapped"})
			}
		case "mcpjson", "opencode":
			if fileContains(tgt.Path, `"doupass"`) && fileContains(tgt.Path, `"proxy"`) {
				rows = append(rows, integrationRow{tgt.Name, "ok", "MCP servers wrapped"})
				active++
			} else {
				rows = append(rows, integrationRow{tgt.Name, "skip", "config found, local MCP servers not wrapped"})
			}
		}
	}
	if len(rows) == 0 {
		fmt.Fprintf(out, "  %s  %s  no agent tools detected\n", padRight("integrations", doctorLabelWidth), stateGlyph("skip"))
		return 0
	}
	state := "skip"
	if active > 0 {
		state = "ok"
	}
	fmt.Fprintf(out, "  %s  %s  %s wired\n", padRight("integrations", doctorLabelWidth), stateGlyph(state), plural(active, "integration"))
	fmt.Fprintln(out)
	nameWidth := 0
	for _, r := range rows {
		if n := utf8.RuneCountInString(r.name); n > nameWidth {
			nameWidth = n
		}
	}
	for _, r := range rows {
		fmt.Fprintf(out, "  %s  %s  %s\n", padRight(r.name, nameWidth), stateGlyph(r.state), r.detail)
	}
	return active
}

func doctorSummary(active, problems int) string {
	switch {
	case problems > 0:
		return failStyle.Render(fmt.Sprintf("%d problem(s) found", problems))
	case active == 0:
		return "nothing is wired yet — run doupass setup --dry-run"
	default:
		return okStyle.Render(fmt.Sprintf("%d integration(s) active — all checks passed", active))
	}
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func fileContains(path, needle string) bool {
	//#nosec G304 -- path comes from the known tool config locations
	data, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	return bytes.Contains(data, []byte(needle))
}
