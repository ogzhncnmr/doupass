package cli

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/ogzhncnmr/doupass/internal/hook"
	"github.com/ogzhncnmr/doupass/internal/policy"
	"github.com/spf13/cobra"
)

func newHookCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "hook",
		Short: "Harness hook adapters",
	}
	var policyFlag string
	claudeCmd := &cobra.Command{
		Use:   "claude",
		Short: "Claude Code PreToolUse adapter (hook JSON on stdin)",
		RunE: func(cmd *cobra.Command, _ []string) error {
			policyPath, err := findPolicyFile(policyFlag)
			if err != nil {
				return err
			}
			engine, err := loadEngine(policyPath)
			if err != nil {
				return err
			}
			data, err := io.ReadAll(io.LimitReader(cmd.InOrStdin(), 1<<20))
			if err != nil {
				return err
			}
			input, err := hook.ParseInput(data)
			if err != nil {
				return err
			}
			out, dec := hook.Decide(engine, input)

			stderr := cmd.ErrOrStderr()
			if dec.Action != policy.ActionAllow {
				fmt.Fprintf(stderr, "doupass: %s %s (%s)\n", dec.Action, input.ToolName, dec.RuleID)
			}
			call := policy.Call{Surface: "hook", Tool: input.ToolName, Args: input.ToolInput}
			appendAudit(engine, call, dec, stderr)
			if out.HookSpecificOutput != nil {
				enc := json.NewEncoder(cmd.OutOrStdout())
				return enc.Encode(out)
			}
			return nil
		},
	}
	claudeCmd.Flags().StringVar(&policyFlag, "policy", "", "policy file (default: ./doupass.yml or ~/.doupass/doupass.yml)")
	cmd.AddCommand(claudeCmd)
	return cmd
}
