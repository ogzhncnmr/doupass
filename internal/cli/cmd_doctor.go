package cli

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"github.com/ogzhncnmr/doupass/internal/audit"
	"github.com/ogzhncnmr/doupass/internal/claude"
	"github.com/ogzhncnmr/doupass/internal/opencode"
	"github.com/ogzhncnmr/doupass/internal/policy"
	"github.com/spf13/cobra"
)

func newDoctorCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "doctor",
		Short: "Show the current state: policy, audit log, and integrations",
		RunE: func(cmd *cobra.Command, _ []string) error {
			out := cmd.OutOrStdout()
			problems := 0
			fmt.Fprintf(out, "doupass doctor — %s (%s/%s)\n\n", ResolvedVersion(), runtime.GOOS, runtime.GOARCH)

			policyPath, err := findPolicyFile("")
			var engine *policy.Engine
			if err != nil {
				fmt.Fprintln(out, "policy       MISSING — run: doupass init, or doupass setup")
				problems++
			} else if engine, err = loadEngine(policyPath); err != nil {
				fmt.Fprintf(out, "policy       INVALID %s\n             %v\n", policyPath, err)
				problems++
			} else {
				issues := policy.Lint(engine.Policy)
				fmt.Fprintf(out, "policy       %s (%s, %d rules, %d lint issue(s))\n",
					policyPath, engine.Policy.Name, len(engine.Policy.Rules), len(issues))
				for _, is := range issues {
					fmt.Fprintf(out, "             warning: %s: %s\n", is.Rule, is.Message)
				}
			}

			auditFile := filepath.Join(homeDir(), ".doupass", "audit.jsonl")
			if engine != nil && engine.Policy.Audit.Path != "" {
				auditFile = expandHome(engine.Policy.Audit.Path)
			}
			if _, statErr := os.Stat(auditFile); os.IsNotExist(statErr) {
				fmt.Fprintf(out, "audit        %s (no entries yet)\n", auditFile)
			} else if res, verifyErr := audit.Verify(auditFile); verifyErr != nil {
				fmt.Fprintf(out, "audit        TAMPERED OR CORRUPT %s\n             %v\n", auditFile, verifyErr)
				problems++
			} else {
				fmt.Fprintf(out, "audit        %s (%d entries, hash chain OK)\n", auditFile, res.Entries)
			}

			fmt.Fprintln(out, "\nintegrations")
			active := 0
			home := homeDir()
			claudePath := filepath.Join(home, ".claude", "settings.json")
			if fileExists(claudePath) {
				if claude.HasHook(claudePath) {
					fmt.Fprintln(out, "  claude-code   hook installed")
					active++
				} else {
					fmt.Fprintln(out, "  claude-code   settings found, hook NOT installed (run: doupass install claude)")
				}
			}
			pluginPath := filepath.Join(home, ".config", "opencode", "plugin", "doupass.js")
			if opencode.PluginInstalled(pluginPath) {
				fmt.Fprintln(out, "  opencode      native-tool plugin installed")
				active++
			}
			for _, tgt := range setupTargets() {
				if tgt.Kind == "claude" {
					continue
				}
				if !fileExists(tgt.Path) {
					continue
				}
				if tgt.Kind == "mcpjson" || tgt.Kind == "opencode" {
					if fileContains(tgt.Path, "doupass") {
						fmt.Fprintf(out, "  %-13s MCP servers wrapped\n", tgt.Name)
						active++
					} else {
						fmt.Fprintf(out, "  %-13s config found, no local MCP servers wrapped\n", tgt.Name)
					}
				}
			}
			if active == 0 {
				fmt.Fprintln(out, "\n  nothing is wired yet — run: doupass setup --dry-run")
			}
			fmt.Fprintf(out, "\n%d integration(s) active\n", active)

			if problems > 0 {
				return fmt.Errorf("%d problem(s) found", problems)
			}
			return nil
		},
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
