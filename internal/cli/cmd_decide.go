package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/ogzhncnmr/doupass/internal/policy"
	"github.com/spf13/cobra"
)

type decideInput struct {
	Surface string         `json:"surface"`
	Server  string         `json:"server"`
	Tool    string         `json:"tool"`
	Args    map[string]any `json:"args"`
}

func newDecideCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "decide",
		Short: "Evaluate one call (JSON on stdin or --input) and print the decision; for custom integrations",
		RunE: func(cmd *cobra.Command, _ []string) error {
			policyFlag, _ := cmd.Flags().GetString("policy")
			policyPath, err := findPolicyFile(policyFlag)
			if err != nil {
				return err
			}
			engine, err := loadEngine(policyPath)
			if err != nil {
				return err
			}
			inputPath, _ := cmd.Flags().GetString("input")
			var data []byte
			if inputPath != "" {
				//#nosec G304 -- input path is chosen by the local user
				data, err = os.ReadFile(inputPath)
			} else {
				data, err = io.ReadAll(io.LimitReader(cmd.InOrStdin(), 1<<20))
			}
			if err != nil {
				return err
			}
			var input decideInput
			if err := json.Unmarshal(data, &input); err != nil {
				return fmt.Errorf("decide: invalid input JSON: %w", err)
			}
			surface := input.Surface
			if surface == "" {
				surface = "hook"
			}
			call := policy.Call{Surface: surface, Server: input.Server, Tool: input.Tool, Args: input.Args}
			dec := engine.Decide(call)
			appendAudit(engine, call, dec, cmd.ErrOrStderr())
			result := struct {
				Action string `json:"action"`
				Rule   string `json:"rule,omitempty"`
				Reason string `json:"reason,omitempty"`
			}{Action: string(dec.Action), Rule: dec.RuleID, Reason: dec.Reason}
			return json.NewEncoder(cmd.OutOrStdout()).Encode(result)
		},
	}
	cmd.Flags().String("policy", "", "policy file (default: ./doupass.yml or ~/.doupass/doupass.yml)")
	cmd.Flags().String("input", "", "read the call JSON from a file instead of stdin")
	return cmd
}
