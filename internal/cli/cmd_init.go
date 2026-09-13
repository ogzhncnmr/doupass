package cli

import (
	"embed"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/ogzhncnmr/doupass/internal/fsutil"
	"github.com/spf13/cobra"
)

//go:embed presets/*.yml
var presetsFS embed.FS

var presetNames = []string{"starter", "minimal", "locked-down", "red-team"}

func newInitCmd() *cobra.Command {
	var dir string
	var force bool
	var preset string
	cmd := &cobra.Command{
		Use:   "init",
		Short: "Write a starter policy file",
		RunE: func(cmd *cobra.Command, _ []string) error {
			data, err := presetsFS.ReadFile("presets/" + preset + ".yml")
			if err != nil {
				return fmt.Errorf("unknown preset %q (available: %s)", preset, strings.Join(presetNames, ", "))
			}
			target := filepath.Join(expandHome(dir), "doupass.yml")
			if _, err := os.Stat(target); err == nil && !force {
				return fmt.Errorf("%s already exists (use --force to overwrite)", target)
			}
			//#nosec G301 -- the target is the user's project directory
			if err := os.MkdirAll(dir, 0o755); err != nil {
				return err
			}
			//#nosec G306 -- the starter policy is meant to be committed to the user's repository
			if err := fsutil.WriteFileAtomic(target, data, 0o644); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%s wrote %s (preset %s)\n", stateGlyph("ok"), target, preset)
			return nil
		},
	}
	cmd.Flags().StringVar(&dir, "dir", ".", "directory to write doupass.yml into")
	cmd.Flags().BoolVar(&force, "force", false, "overwrite an existing file")
	cmd.Flags().StringVar(&preset, "preset", "starter", "policy preset: "+strings.Join(presetNames, ", "))
	return cmd
}
