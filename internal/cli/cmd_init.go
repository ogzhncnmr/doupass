package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

const starterPolicy = `version: "0.1"
name: default-dev

defaults:
  action: allow
  ask_fallback: deny
  ask_timeout_seconds: 60

audit:
  path: "~/.doupass/audit.jsonl"
  hash_chain: true

rules:
  - id: block-ssh-credentials
    match:
      args:
        "*": "**/.ssh/**"
    action: deny
    reason: "SSH credentials are off-limits"

  - id: block-env-files
    match:
      args:
        "*": "**/.env*"
    action: deny
    reason: "Environment files may contain secrets"

  - id: ask-npm-install
    match:
      surface: hook
      tool: Bash
      args:
        command: "*npm install*"
    action: ask
    reason: "Package installs run lifecycle scripts"

  - id: ask-rm-recursive
    match:
      surface: hook
      tool: Bash
      args:
        command: "*rm -rf*"
    action: ask
    reason: "Recursive delete needs confirmation"
`

func newInitCmd() *cobra.Command {
	var dir string
	var force bool
	cmd := &cobra.Command{
		Use:   "init",
		Short: "Write a starter policy file",
		RunE: func(cmd *cobra.Command, _ []string) error {
			target := filepath.Join(dir, "doupass.yml")
			if _, err := os.Stat(target); err == nil && !force {
				return fmt.Errorf("%s already exists (use --force to overwrite)", target)
			}
			if err := os.MkdirAll(dir, 0o755); err != nil {
				return err
			}
			if err := os.WriteFile(target, []byte(starterPolicy), 0o644); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "wrote %s\n", target)
			return nil
		},
	}
	cmd.Flags().StringVar(&dir, "dir", ".", "directory to write doupass.yml into")
	cmd.Flags().BoolVar(&force, "force", false, "overwrite an existing file")
	return cmd
}
