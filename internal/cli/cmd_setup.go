package cli

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"unicode/utf8"

	"github.com/ogzhncnmr/doupass/internal/claude"
	"github.com/ogzhncnmr/doupass/internal/codex"
	"github.com/ogzhncnmr/doupass/internal/mcpjson"
	"github.com/ogzhncnmr/doupass/internal/opencode"
	"github.com/spf13/cobra"
)

type setupTarget struct {
	Name string
	Kind string
	Path string
}

type setupRow struct {
	name   string
	result string
	path   string
	state  string
	detail string
}

func newSetupCmd() *cobra.Command {
	var dryRun bool
	cmd := &cobra.Command{
		Use:   "setup",
		Short: "Detect installed agent tools and wire doupass where supported",
		RunE: func(cmd *cobra.Command, _ []string) error {
			out := cmd.OutOrStdout()
			if _, err := findPolicyFile(""); err != nil {
				if p, perr := ensureStarterPolicy(); perr == nil {
					fmt.Fprintf(out, "%s no policy found — wrote the starter policy to %s\nedit it (or run doupass init --preset) to change what agents may do\n\n", stateGlyph("ok"), p)
				}
			}
			var rows []setupRow
			failures := 0
			for _, tgt := range setupTargets() {
				if _, err := os.Stat(tgt.Path); err != nil {
					continue
				}
				if dryRun {
					rows = append(rows, setupRow{name: tgt.Name, path: tgt.Path, state: "plan", result: planFor(tgt.Kind)})
					continue
				}
				row := setupRow{name: tgt.Name, path: tgt.Path, state: "skip"}
				switch tgt.Kind {
				case "codex":
					res, err := codex.WrapServers(tgt.Path)
					row = wrapResultRow(row, res.Changed, res.Servers, err)
					if err != nil {
						failures++
					}
				case "claude":
					res, err := claude.InstallHook(tgt.Path, "doupass hook claude")
					if err != nil {
						row.state, row.result, row.detail = "fail", "hook install failed", err.Error()
						failures++
					} else if res.Changed {
						row.state, row.result = "done", "hook installed"
					} else {
						row.result = "hook already installed"
					}
				case "opencode":
					res, err := opencode.WrapServers(tgt.Path)
					row = wrapResultRow(row, res.Changed, res.Servers, err)
					if err != nil {
						failures++
					}
				case "mcpjson":
					res, err := mcpjson.WrapServers(tgt.Path)
					row = wrapResultRow(row, res.Changed, res.Servers, err)
					if err != nil {
						failures++
					}
				}
				rows = append(rows, row)
			}
			printSetupReport(cmd.OutOrStdout(), dryRun, rows)
			if failures > 0 {
				return fmt.Errorf("%d target(s) failed", failures)
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "show what would change without touching any file")
	return cmd
}

func planFor(kind string) string {
	if kind == "claude" {
		return "would register the PreToolUse hook"
	}
	return "would wrap local MCP servers"
}

// ensureStarterPolicy writes the starter preset to the user-level policy
// location so a bare "doupass setup" leaves the machine protected.
func ensureStarterPolicy() (string, error) {
	home := homeDir()
	if home == "" {
		return "", errors.New("cannot determine home directory")
	}
	dir := filepath.Join(home, ".doupass")
	path := filepath.Join(dir, "doupass.yml")
	if _, err := os.Stat(path); err == nil {
		return path, nil
	}
	//#nosec G301 -- the policy directory is user-private
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	data, err := presetsFS.ReadFile("presets/starter.yml")
	if err != nil {
		return "", err
	}
	//#nosec G306 -- the policy lives under the user's home directory
	return path, os.WriteFile(path, data, 0o600)
}

func wrapResultRow(row setupRow, changed bool, servers int, err error) setupRow {
	switch {
	case err != nil:
		row.state, row.result, row.detail = "fail", "wrap failed", err.Error()
	case changed:
		row.state, row.result = "done", fmt.Sprintf("wrapped %d MCP server(s)", servers)
	default:
		row.result = "no local MCP servers to wrap"
	}
	return row
}

const (
	setupNameWidth   = 22
	setupResultWidth = 42
	setupPathWidth   = 56
)

func printSetupReport(out io.Writer, dryRun bool, rows []setupRow) {
	title := "doupass setup"
	if dryRun {
		title += " — dry run (no files will be changed)"
	}
	fmt.Fprintln(out, title)
	fmt.Fprintln(out)
	if len(rows) == 0 {
		fmt.Fprintln(out, "  no supported tools detected")
		fmt.Fprintln(out)
		fmt.Fprintln(out, "  Install Claude Code, opencode, Cursor, Windsurf, Kiro, Cline, or Roo Code and rerun.")
		return
	}

	secondHeader := "ACTION"
	resultWidth := len(secondHeader)
	if !dryRun {
		secondHeader = "RESULT"
		resultWidth = len(secondHeader)
	}
	nameWidth := len("TOOL")
	for _, r := range rows {
		if n := utf8.RuneCountInString(r.name); n > nameWidth {
			nameWidth = n
		}
		if n := utf8.RuneCountInString(r.result); n > resultWidth {
			resultWidth = n
		}
	}
	if nameWidth > setupNameWidth {
		nameWidth = setupNameWidth
	}
	if resultWidth > setupResultWidth {
		resultWidth = setupResultWidth
	}

	fmt.Fprintf(out, "    %s  %s  %s\n",
		dimStyle.Render(padRight("TOOL", nameWidth)),
		dimStyle.Render(padRight(secondHeader, resultWidth)),
		dimStyle.Render("CONFIG"))
	for _, r := range rows {
		fmt.Fprintf(out, "  %s %s  %s  %s\n",
			stateGlyph(r.state),
			padRight(truncateMiddle(r.name, nameWidth), nameWidth),
			padRight(truncateMiddle(r.result, resultWidth), resultWidth),
			truncateMiddle(r.path, setupPathWidth))
		if r.state == "fail" {
			fmt.Fprintf(out, "        %s\n", failStyle.Render(r.detail))
		}
	}

	fmt.Fprintln(out)
	if dryRun {
		fmt.Fprintf(out, "  %s detected. Run \"doupass setup\" to apply.\n", plural(len(rows), "tool"))
	} else {
		fmt.Fprintln(out, "  Restart any tool that changed.")
	}
	fmt.Fprintf(out, "  %s\n", dimStyle.Render("Backups are written next to each config file (<config>.doupass.bak)."))
	fmt.Fprintf(out, "  %s\n", dimStyle.Render("Native-tool hooks are available for Claude Code (hook) and opencode (plugin); other tools get MCP proxying."))
}

func setupTargets() []setupTarget {
	home := homeDir()
	targets := []setupTarget{
		{Name: "claude-code", Kind: "claude", Path: filepath.Join(home, ".claude", "settings.json")},
	}
	for _, c := range []string{
		filepath.Join(home, ".config", "opencode", "opencode.json"),
		filepath.Join(home, ".config", "opencode", "opencode.jsonc"),
	} {
		if _, err := os.Stat(c); err == nil {
			targets = append(targets, setupTarget{Name: "opencode", Kind: "opencode", Path: c})
			break
		}
	}
	targets = append(targets,
		setupTarget{Name: "codex", Kind: "codex", Path: filepath.Join(home, ".codex", "config.toml")},
		setupTarget{Name: "cursor", Kind: "mcpjson", Path: filepath.Join(home, ".cursor", "mcp.json")},
		setupTarget{Name: "windsurf", Kind: "mcpjson", Path: filepath.Join(home, ".codeium", "windsurf", "mcp_config.json")},
		setupTarget{Name: "kiro", Kind: "mcpjson", Path: filepath.Join(home, ".kiro", "settings", "mcp.json")},
	)
	if cfgDir, err := os.UserConfigDir(); err == nil {
		for _, app := range []string{"Code", "Cursor", "Windsurf"} {
			base := filepath.Join(cfgDir, app, "User", "globalStorage")
			targets = append(targets,
				setupTarget{Name: "cline (" + app + ")", Kind: "mcpjson",
					Path: filepath.Join(base, "saoudrizwan.claude-dev", "settings", "cline_mcp_settings.json")},
				setupTarget{Name: "roo-code (" + app + ")", Kind: "mcpjson",
					Path: filepath.Join(base, "rooveterinaryinc.roo-cline", "settings", "mcp_settings.json")},
			)
		}
	}
	return targets
}
