package cli

import (
	"fmt"
	"io"

	"github.com/spf13/cobra"
)

var Version = "dev"

func Execute(args []string, stdout, stderr io.Writer) error {
	root := NewRootCmd()
	root.SetArgs(args)
	root.SetOut(stdout)
	root.SetErr(stderr)
	return root.Execute()
}

func NewRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:           "doupass",
		Short:         "Local-first policy engine for AI coding agents",
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.AddCommand(
		newVersionCmd(),
		newInitCmd(),
		newPolicyCmd(),
		newProxyCmd(),
		newLogCmd(),
	)
	return root
}

func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print the doupass version",
		RunE: func(cmd *cobra.Command, _ []string) error {
			fmt.Fprintf(cmd.OutOrStdout(), "doupass %s\n", Version)
			return nil
		},
	}
}
