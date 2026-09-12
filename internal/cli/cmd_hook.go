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
			out := cmd.OutOrStdout()
			stderr := cmd.ErrOrStderr()

			// A hook that exits non-zero is non-blocking for Claude Code, so
			// every failure mode below must deny out loud (exit 0) instead of
			// letting the tool call proceed unenforced.
			denyOnError := func(reason string) error {
				fmt.Fprintf(stderr, "doupass: deny: %s\n", reason)
				return emitHookDecision(out, "deny", reason)
			}

			policyPath, err := findPolicyFile(policyFlag)
			if err != nil {
				return denyOnError("policy unavailable: " + err.Error())
			}
			engine, err := loadEngine(policyPath)
			if err != nil {
				return denyOnError("policy unavailable: " + err.Error())
			}
			data, err := io.ReadAll(io.LimitReader(cmd.InOrStdin(), 1<<20))
			if err != nil {
				return denyOnError("could not read hook input")
			}
			input, err := hook.ParseInput(data)
			if err != nil {
				return denyOnError("could not parse hook input")
			}

			outDecision, dec := hook.Decide(engine, input)
			if dec.Action != policy.ActionAllow {
				fmt.Fprintf(stderr, "doupass: %s %s (%s)\n", dec.Action, input.ToolName, dec.RuleID)
			}
			call := policy.Call{Surface: "hook", Tool: input.ToolName, Args: input.ToolInput}
			appendAudit(engine, call, dec, stderr)
			if outDecision.HookSpecificOutput != nil {
				enc := json.NewEncoder(out)
				return enc.Encode(outDecision)
			}
			return nil
		},
	}
	claudeCmd.Flags().StringVar(&policyFlag, "policy", "", "policy file (default: ./doupass.yml or ~/.doupass/doupass.yml)")
	cmd.AddCommand(claudeCmd)
	return cmd
}

func emitHookDecision(out io.Writer, decision, reason string) error {
	return json.NewEncoder(out).Encode(map[string]any{
		"hookSpecificOutput": map[string]any{
			"hookEventName":            "PreToolUse",
			"permissionDecision":       decision,
			"permissionDecisionReason": reason,
		},
	})
}
