package cli

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/ogzhncnmr/doupass/internal/claude"
	"github.com/ogzhncnmr/doupass/internal/codex"
	"github.com/ogzhncnmr/doupass/internal/mcpjson"
	"github.com/ogzhncnmr/doupass/internal/opencode"
	"github.com/spf13/cobra"
)

func newInstallCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "install",
		Short: "Install harness integrations",
	}
	cmd.AddCommand(newInstallClaudeCmd(), newInstallOpenCodeCmd(), newInstallCodexCmd(), newInstallGenericCmd())
	return cmd
}

func newUninstallCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "uninstall",
		Short: "Remove harness integrations",
	}
	cmd.AddCommand(newUninstallClaudeCmd(), newUninstallOpenCodeCmd(), newUninstallCodexCmd(), newUninstallGenericCmd())
	return cmd
}

func newInstallGenericCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "generic",
		Short: "Wrap local MCP servers in any mcpServers-style JSON config",
		RunE: func(cmd *cobra.Command, _ []string) error {
			config, _ := cmd.Flags().GetString("config")
			if config == "" {
				return errors.New("--config is required")
			}
			res, err := mcpjson.WrapServers(expandHome(config))
			if err != nil {
				return err
			}
			out := cmd.OutOrStdout()
			if res.Changed {
				fmt.Fprintf(out, "wrapped %d MCP server(s) in %s\n", res.Servers, res.Path)
				if res.Backup != "" {
					fmt.Fprintf(out, "backup: %s\n", res.Backup)
				}
			} else {
				fmt.Fprintf(out, "nothing to wrap in %s\n", res.Path)
			}
			return nil
		},
	}
	cmd.Flags().String("config", "", "path to an mcpServers-style JSON config file (required)")
	return cmd
}

func newUninstallGenericCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "generic",
		Short: "Unwrap doupass-wrapped MCP servers in any mcpServers-style JSON config",
		RunE: func(cmd *cobra.Command, _ []string) error {
			config, _ := cmd.Flags().GetString("config")
			if config == "" {
				return errors.New("--config is required")
			}
			res, err := mcpjson.UnwrapServers(expandHome(config))
			if err != nil {
				return err
			}
			out := cmd.OutOrStdout()
			if res.Changed {
				fmt.Fprintf(out, "unwrapped %d MCP server(s) in %s\n", res.Servers, res.Path)
			} else {
				fmt.Fprintf(out, "no doupass-wrapped servers found in %s\n", res.Path)
			}
			return nil
		},
	}
	cmd.Flags().String("config", "", "path to an mcpServers-style JSON config file (required)")
	return cmd
}

func newInstallClaudeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "claude",
		Short: "Register the doupass PreToolUse hook in Claude Code settings",
		RunE: func(cmd *cobra.Command, _ []string) error {
			path, err := claudeSettingsPath(cmd)
			if err != nil {
				return err
			}
			command, _ := cmd.Flags().GetString("command")
			res, err := claude.InstallHook(path, command)
			if err != nil {
				return err
			}
			out := cmd.OutOrStdout()
			if res.Changed {
				fmt.Fprintf(out, "installed hook in %s\n", res.Path)
				if res.Backup != "" {
					fmt.Fprintf(out, "backup: %s\n", res.Backup)
				}
			} else {
				fmt.Fprintf(out, "already installed in %s\n", res.Path)
			}
			return nil
		},
	}
	cmd.Flags().String("settings", "", "settings file (default: ~/.claude/settings.json)")
	cmd.Flags().String("command", "doupass hook claude", "hook command to register")
	return cmd
}

func newUninstallClaudeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "claude",
		Short: "Remove the doupass PreToolUse hook from Claude Code settings",
		RunE: func(cmd *cobra.Command, _ []string) error {
			path, err := claudeSettingsPath(cmd)
			if err != nil {
				return err
			}
			res, err := claude.UninstallHook(path)
			if err != nil {
				return err
			}
			out := cmd.OutOrStdout()
			if res.Changed {
				fmt.Fprintf(out, "removed doupass hook from %s\n", res.Path)
			} else {
				fmt.Fprintf(out, "no doupass hook found in %s\n", res.Path)
			}
			return nil
		},
	}
	cmd.Flags().String("settings", "", "settings file (default: ~/.claude/settings.json)")
	return cmd
}

func newInstallOpenCodeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "opencode",
		Short: "Wrap local MCP servers or install the native-tool plugin for opencode",
		RunE: func(cmd *cobra.Command, _ []string) error {
			withPlugin, _ := cmd.Flags().GetBool("plugin")
			if withPlugin {
				pluginPath := filepath.Join(opencodePluginDir(cmd), "doupass.js")
				binary, err := os.Executable()
				if err != nil {
					binary = "doupass"
				}
				res, err := opencode.InstallPlugin(pluginPath, binary)
				if err != nil {
					return err
				}
				out := cmd.OutOrStdout()
				if res.Changed {
					fmt.Fprintf(out, "installed native-tool plugin in %s\n", res.Path)
					fmt.Fprintln(out, "restart opencode for it to take effect")
				} else {
					fmt.Fprintf(out, "plugin already installed in %s\n", res.Path)
				}
				return nil
			}
			path, err := opencodeConfigPath(cmd)
			if err != nil {
				return err
			}
			res, err := opencode.WrapServers(path)
			if err != nil {
				return err
			}
			out := cmd.OutOrStdout()
			if res.Changed {
				fmt.Fprintf(out, "wrapped %d MCP server(s) in %s\n", res.Servers, res.Path)
				if res.Backup != "" {
					fmt.Fprintf(out, "backup: %s\n", res.Backup)
				}
			} else {
				fmt.Fprintf(out, "nothing to wrap in %s\n", res.Path)
			}
			return nil
		},
	}
	cmd.Flags().String("config", "", "opencode config file (default: ./opencode.json[c] or ~/.config/opencode/opencode.json[c])")
	cmd.Flags().Bool("plugin", false, "install the native-tool plugin instead of wrapping MCP servers")
	cmd.Flags().String("plugin-dir", "", "directory for the plugin file (default: ~/.config/opencode/plugin; with --plugin)")
	return cmd
}

func newUninstallOpenCodeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "opencode",
		Short: "Unwrap doupass-wrapped MCP servers or remove the plugin from opencode",
		RunE: func(cmd *cobra.Command, _ []string) error {
			withPlugin, _ := cmd.Flags().GetBool("plugin")
			if withPlugin {
				res, err := opencode.UninstallPlugin(filepath.Join(opencodePluginDir(cmd), "doupass.js"))
				if err != nil {
					return err
				}
				out := cmd.OutOrStdout()
				if res.Changed {
					fmt.Fprintf(out, "removed plugin %s\n", res.Path)
				} else {
					fmt.Fprintf(out, "no doupass plugin found in %s\n", res.Path)
				}
				return nil
			}
			path, err := opencodeConfigPath(cmd)
			if err != nil {
				return err
			}
			res, err := opencode.UnwrapServers(path)
			if err != nil {
				return err
			}
			out := cmd.OutOrStdout()
			if res.Changed {
				fmt.Fprintf(out, "unwrapped %d MCP server(s) in %s\n", res.Servers, res.Path)
			} else {
				fmt.Fprintf(out, "no doupass-wrapped servers found in %s\n", res.Path)
			}
			return nil
		},
	}
	cmd.Flags().String("config", "", "opencode config file (default: ./opencode.json[c] or ~/.config/opencode/opencode.json[c])")
	cmd.Flags().Bool("plugin", false, "remove the native-tool plugin instead of unwrapping MCP servers")
	cmd.Flags().String("plugin-dir", "", "directory for the plugin file (default: ~/.config/opencode/plugin; with --plugin)")
	return cmd
}

// opencodePluginDir resolves the plugin location: the global opencode
// directory by default (what doctor and the menu count), or wherever
// --plugin-dir points for a project-local install.
func opencodePluginDir(cmd *cobra.Command) string {
	if dir, _ := cmd.Flags().GetString("plugin-dir"); dir != "" {
		return expandHome(dir)
	}
	if home := homeDir(); home != "" {
		return filepath.Join(home, ".config", "opencode", "plugin")
	}
	return filepath.Join(".opencode", "plugin")
}

func newInstallCodexCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "codex",
		Short: "Wrap local MCP servers in the Codex CLI config (config.toml)",
		RunE: func(cmd *cobra.Command, _ []string) error {
			path, err := codexConfigPath(cmd)
			if err != nil {
				return err
			}
			res, err := codex.WrapServers(path)
			if err != nil {
				return err
			}
			out := cmd.OutOrStdout()
			if res.Changed {
				fmt.Fprintf(out, "wrapped %d MCP server(s) in %s\n", res.Servers, res.Path)
				if res.Backup != "" {
					fmt.Fprintf(out, "backup: %s\n", res.Backup)
				}
			} else {
				fmt.Fprintf(out, "nothing to wrap in %s\n", res.Path)
			}
			return nil
		},
	}
	cmd.Flags().String("config", "", "Codex config file (default: ~/.codex/config.toml)")
	return cmd
}

func newUninstallCodexCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "codex",
		Short: "Unwrap doupass-wrapped MCP servers in the Codex CLI config",
		RunE: func(cmd *cobra.Command, _ []string) error {
			path, err := codexConfigPath(cmd)
			if err != nil {
				return err
			}
			res, err := codex.UnwrapServers(path)
			if err != nil {
				return err
			}
			out := cmd.OutOrStdout()
			if res.Changed {
				fmt.Fprintf(out, "unwrapped %d MCP server(s) in %s\n", res.Servers, res.Path)
			} else {
				fmt.Fprintf(out, "no doupass-wrapped servers found in %s\n", res.Path)
			}
			return nil
		},
	}
	cmd.Flags().String("config", "", "Codex config file (default: ~/.codex/config.toml)")
	return cmd
}

func codexConfigPath(cmd *cobra.Command) (string, error) {
	if p, _ := cmd.Flags().GetString("config"); p != "" {
		return expandHome(p), nil
	}
	home := homeDir()
	if home == "" {
		return "", errors.New("cannot determine home directory; pass --config")
	}
	return filepath.Join(home, ".codex", "config.toml"), nil
}

func claudeSettingsPath(cmd *cobra.Command) (string, error) {
	if p, _ := cmd.Flags().GetString("settings"); p != "" {
		return expandHome(p), nil
	}
	home := homeDir()
	if home == "" {
		return "", errors.New("cannot determine home directory; pass --settings")
	}
	return filepath.Join(home, ".claude", "settings.json"), nil
}

func opencodeConfigPath(cmd *cobra.Command) (string, error) {
	if p, _ := cmd.Flags().GetString("config"); p != "" {
		return expandHome(p), nil
	}
	candidates := []string{"opencode.json", "opencode.jsonc"}
	if home := homeDir(); home != "" {
		candidates = append(candidates,
			filepath.Join(home, ".config", "opencode", "opencode.json"),
			filepath.Join(home, ".config", "opencode", "opencode.jsonc"),
		)
	}
	for _, c := range candidates {
		if _, err := os.Stat(c); err == nil {
			return c, nil
		}
	}
	return "", errors.New("no opencode config found (looked for opencode.json[c] and ~/.config/opencode/opencode.json[c]); pass --config")
}
