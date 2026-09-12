package cli

import (
	"fmt"
	"path/filepath"

	"github.com/ogzhncnmr/doupass/internal/audit"
	"github.com/spf13/cobra"
)

func newLogCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "log",
		Short: "Inspect the audit log",
	}
	cmd.PersistentFlags().String("path", "", "audit log path (default: ~/.doupass/audit.jsonl)")
	cmd.AddCommand(newLogTailCmd(), newLogVerifyCmd())
	return cmd
}

func auditPath(cmd *cobra.Command) string {
	p, _ := cmd.Flags().GetString("path")
	if p != "" {
		return p
	}
	return filepath.Join(homeDir(), ".doupass", "audit.jsonl")
}

func newLogTailCmd() *cobra.Command {
	var n int
	cmd := &cobra.Command{
		Use:   "tail",
		Short: "Print the most recent audit entries",
		RunE: func(cmd *cobra.Command, _ []string) error {
			entries, err := audit.ReadAll(auditPath(cmd))
			if err != nil {
				return err
			}
			if n > 0 && len(entries) > n {
				entries = entries[len(entries)-n:]
			}
			out := cmd.OutOrStdout()
			for _, e := range entries {
				target := e.Tool
				if e.Server != "" {
					target = e.Server + "." + e.Tool
				}
				fmt.Fprintf(out, "%s  %-5s %-24s %s\n", e.Time, e.Action, target, e.Rule)
			}
			return nil
		},
	}
	cmd.Flags().IntVar(&n, "n", 20, "number of entries to show")
	return cmd
}

func newLogVerifyCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "verify",
		Short: "Verify the audit log hash chain",
		RunE: func(cmd *cobra.Command, _ []string) error {
			res, err := audit.Verify(auditPath(cmd))
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "OK: %d entries, last hash %s\n", res.Entries, shortHash(res.LastHash))
			return nil
		},
	}
}

func shortHash(h string) string {
	if len(h) > 12 {
		return h[:12] + "..."
	}
	return h
}
