package cli

import (
	"errors"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/ogzhncnmr/doupass/internal/audit"
	"github.com/ogzhncnmr/doupass/internal/policy"
	"github.com/ogzhncnmr/doupass/internal/proxy"
	"github.com/ogzhncnmr/doupass/internal/tty"
	"github.com/spf13/cobra"
)

func newProxyCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "proxy [flags] -- <command> [args...]",
		Short: "Run an MCP server behind the policy engine",
		RunE: func(cmd *cobra.Command, _ []string) error {
			server, _ := cmd.Flags().GetString("server")
			if server == "" {
				return errors.New("--server is required")
			}
			policyFlag, _ := cmd.Flags().GetString("policy")
			policyPath, err := findPolicyFile(policyFlag)
			if err != nil {
				return err
			}
			engine, err := loadEngine(policyPath)
			if err != nil {
				return err
			}
			dash := cmd.ArgsLenAtDash()
			rest := cmd.Flags().Args()
			if dash < 0 || dash >= len(rest) {
				return errors.New("downstream command is required after --")
			}

			stderr := cmd.ErrOrStderr()
			var logger *audit.Logger
			if p := engine.Policy.Audit.Path; p != "" {
				logger = &audit.Logger{Path: expandHome(p)}
			}
			cfg := proxy.Config{
				ServerName: server,
				Engine:     engine,
				Command:    rest[dash:],
				Stderr:     stderr,
				Prompt:     tty.Prompt,
				OnDecision: func(call policy.Call, dec policy.Decision) {
					if dec.Action != policy.ActionAllow {
						fmt.Fprintf(stderr, "doupass: %s %s.%s (%s)\n", dec.Action, call.Server, call.Tool, dec.RuleID)
					}
					if logger != nil {
						if err := logger.Append(call, dec); err != nil {
							fmt.Fprintf(stderr, "doupass: audit error: %v\n", err)
						}
					}
				},
			}
			// On Ctrl+C the default handler would kill doupass and orphan the
			// downstream MCP server; routing the signal into the context lets
			// proxy.Run kill the whole process tree instead.
			ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
			defer stop()
			return proxy.Run(ctx, cfg)
		},
	}
	cmd.Flags().String("server", "", "MCP server name shown to the policy")
	cmd.Flags().String("policy", "", "policy file (default: ./doupass.yml or ~/.doupass/doupass.yml)")
	return cmd
}
