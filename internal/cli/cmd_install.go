package cli

import (
	"errors"
	"fmt"
	"path/filepath"

	"github.com/ogzhncnmr/doupass/internal/claude"
	"github.com/spf13/cobra"
)

func newInstallCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "install",
		Short: "Install harness integrations",
	}
	cmd.PersistentFlags().String("settings", "", "settings file (default: ~/.claude/settings.json)")
	cmd.PersistentFlags().String("command", "doupass hook claude", "hook command to register")
	cmd.AddCommand(&cobra.Command{
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
	})
	return cmd
}

func newUninstallCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "uninstall",
		Short: "Remove harness integrations",
	}
	cmd.PersistentFlags().String("settings", "", "settings file (default: ~/.claude/settings.json)")
	cmd.AddCommand(&cobra.Command{
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
	})
	return cmd
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
