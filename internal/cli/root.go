package cli

import (
	"fmt"
	"io"
	"runtime/debug"
	"strings"

	"github.com/spf13/cobra"
)

var Version = "dev"

func ResolvedVersion() string {
	if Version != "dev" {
		return Version
	}
	if info, ok := debug.ReadBuildInfo(); ok {
		v := info.Main.Version
		if v != "" && v != "(devel)" {
			return strings.TrimPrefix(v, "v")
		}
	}
	return Version
}

func Execute(args []string, stdin io.Reader, stdout, stderr io.Writer) error {
	root := NewRootCmd()
	root.SetArgs(args)
	root.SetIn(stdin)
	root.SetOut(stdout)
	root.SetErr(stderr)
	return root.Execute()
}

func NewRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:   "doupass",
		Short: "Local-first policy engine for AI coding agents",
		Long: `doupass keeps your AI coding agents inside rules you define.

Every tool call an agent makes — shell commands, file reads, MCP calls — is
checked against one policy file and recorded in a tamper-evident local audit
log. Nothing leaves your machine and no model is involved in the decision.

Everyday flow:
  1. doupass doctor                see the current state
  2. doupass setup                 wire the tools installed on this machine
  3. (edit ~/.doupass/doupass.yml) change what agents may do
  4. doupass log tail              review what agents actually did`,
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				return cmd.Help()
			}
			return runLanding(cmd)
		},
	}
	root.AddCommand(
		newVersionCmd(),
		newInitCmd(),
		newSetupCmd(),
		newDoctorCmd(),
		newPolicyCmd(),
		newProxyCmd(),
		newLogCmd(),
		newHookCmd(),
		newDecideCmd(),
		newInstallCmd(),
		newUninstallCmd(),
	)
	return root
}

func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print the doupass version",
		RunE: func(cmd *cobra.Command, _ []string) error {
			fmt.Fprintf(cmd.OutOrStdout(), "doupass %s\n", ResolvedVersion())
			return nil
		},
	}
}
