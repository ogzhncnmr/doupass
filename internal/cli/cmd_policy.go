package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/ogzhncnmr/doupass/internal/policy"
	"github.com/spf13/cobra"
)

func newPolicyCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "policy",
		Short: "Inspect and test policies",
	}
	cmd.AddCommand(newPolicyTestCmd(), newPolicyLintCmd())
	return cmd
}

func newPolicyLintCmd() *cobra.Command {
	var strict bool
	cmd := &cobra.Command{
		Use:   "lint <policy.yml> [more...]",
		Short: "Check policies for common mistakes",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			out := cmd.OutOrStdout()
			failed := false
			for _, path := range args {
				p, err := policy.LoadFile(expandHome(path))
				if err != nil {
					if os.IsNotExist(err) {
						return fmt.Errorf("policy file not found: %s", expandHome(path))
					}
					return err
				}
				issues := policy.Lint(p)
				fmt.Fprintf(out, "%s: %s (%d rules, %d issue(s))\n", path, p.Name, len(p.Rules), len(issues))
				for _, is := range issues {
					fmt.Fprintf(out, "  %s %s: %s\n", is.Level, is.Rule, is.Message)
				}
				if len(issues) > 0 {
					failed = true
				}
			}
			if failed && strict {
				return errors.New("policy lint failed (--strict)")
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&strict, "strict", false, "exit non-zero when any issue is found")
	return cmd
}

func newPolicyTestCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "test <policy.yml>",
		Short: "Validate a policy and optionally evaluate a single call",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			engine, err := loadEngine(expandHome(args[0]))
			if err != nil {
				return err
			}
			tool, _ := cmd.Flags().GetString("tool")
			if tool == "" {
				fmt.Fprintf(cmd.OutOrStdout(), "OK: %s (%d rules)\n", engine.Policy.Name, len(engine.Policy.Rules))
				return nil
			}
			surface, _ := cmd.Flags().GetString("surface")
			server, _ := cmd.Flags().GetString("server")
			rawArgs, _ := cmd.Flags().GetStringArray("arg")
			callArgs := make(map[string]any, len(rawArgs))
			for _, kv := range rawArgs {
				k, v, ok := strings.Cut(kv, "=")
				if !ok || k == "" {
					return fmt.Errorf("--arg must be key=value, got %q", kv)
				}
				callArgs[k] = v
			}
			dec := engine.Decide(policy.Call{Surface: surface, Server: server, Tool: tool, Args: callArgs})
			result := struct {
				Action string `json:"action"`
				Rule   string `json:"rule,omitempty"`
				Reason string `json:"reason,omitempty"`
			}{Action: string(dec.Action), Rule: dec.RuleID, Reason: dec.Reason}
			enc := json.NewEncoder(cmd.OutOrStdout())
			enc.SetIndent("", "  ")
			return enc.Encode(result)
		},
	}
	cmd.Flags().String("surface", "hook", "surface to evaluate: hook or mcp")
	cmd.Flags().String("server", "", "MCP server name for mcp calls")
	cmd.Flags().String("tool", "", "tool name; when set, a call is evaluated instead of only validating")
	cmd.Flags().StringArray("arg", nil, "argument as key=value (repeatable)")
	return cmd
}
