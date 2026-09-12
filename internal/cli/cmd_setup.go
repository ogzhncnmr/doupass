package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/ogzhncnmr/doupass/internal/claude"
	"github.com/ogzhncnmr/doupass/internal/mcpjson"
	"github.com/ogzhncnmr/doupass/internal/opencode"
	"github.com/spf13/cobra"
)

type setupTarget struct {
	Name string
	Kind string
	Path string
}

func newSetupCmd() *cobra.Command {
	var dryRun bool
	cmd := &cobra.Command{
		Use:   "setup",
		Short: "Detect installed agent tools and wire doupass where supported",
		RunE: func(cmd *cobra.Command, _ []string) error {
			out := cmd.OutOrStdout()
			if dryRun {
				fmt.Fprintln(out, "doupass setup (dry-run)")
			} else {
				fmt.Fprintln(out, "doupass setup")
			}
			fmt.Fprintln(out)
			found := 0
			for _, tgt := range setupTargets() {
				if _, err := os.Stat(tgt.Path); err != nil {
					continue
				}
				found++
				switch tgt.Kind {
				case "claude":
					if dryRun {
						fmt.Fprintf(out, "  %-24s %s\n      -> would register the PreToolUse hook\n", tgt.Name, tgt.Path)
						continue
					}
					res, err := claude.InstallHook(tgt.Path, "doupass hook claude")
					if err != nil {
						return fmt.Errorf("%s: %w", tgt.Name, err)
					}
					status := "hook already installed"
					if res.Changed {
						status = "hook installed (backup: " + res.Backup + ")"
					}
					fmt.Fprintf(out, "  %-24s %s\n      -> %s\n", tgt.Name, tgt.Path, status)
				case "opencode":
					if dryRun {
						fmt.Fprintf(out, "  %-24s %s\n      -> would wrap local MCP servers\n", tgt.Name, tgt.Path)
						continue
					}
					res, err := opencode.WrapServers(tgt.Path)
					if err != nil {
						return fmt.Errorf("%s: %w", tgt.Name, err)
					}
					status := "no local MCP servers to wrap"
					if res.Changed {
						status = fmt.Sprintf("wrapped %d MCP server(s) (backup: %s)", res.Servers, res.Backup)
					}
					fmt.Fprintf(out, "  %-24s %s\n      -> %s\n", tgt.Name, tgt.Path, status)
				case "mcpjson":
					if dryRun {
						fmt.Fprintf(out, "  %-24s %s\n      -> would wrap local MCP servers\n", tgt.Name, tgt.Path)
						continue
					}
					res, err := mcpjson.WrapServers(tgt.Path)
					if err != nil {
						return fmt.Errorf("%s: %w", tgt.Name, err)
					}
					status := "no local MCP servers to wrap"
					if res.Changed {
						status = fmt.Sprintf("wrapped %d MCP server(s) (backup: %s)", res.Servers, res.Backup)
					}
					fmt.Fprintf(out, "  %-24s %s\n      -> %s\n", tgt.Name, tgt.Path, status)
				}
			}
			if found == 0 {
				fmt.Fprintln(out, "  no supported tools detected")
			}
			fmt.Fprintln(out)
			if dryRun {
				fmt.Fprintln(out, "Run without --dry-run to apply. Backups are written next to each config file.")
			} else {
				fmt.Fprintln(out, "Restart any tool that changed.")
			}
			fmt.Fprintln(out, "Native-tool hooks are available for Claude Code (hook) and opencode (plugin); other tools get MCP proxying.")
			return nil
		},
	}
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "show what would change without touching any file")
	return cmd
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
