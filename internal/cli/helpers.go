package cli

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/ogzhncnmr/doupass/internal/audit"
	"github.com/ogzhncnmr/doupass/internal/policy"
)

func appendAudit(engine *policy.Engine, call policy.Call, dec policy.Decision, stderr io.Writer) {
	path := engine.Policy.Audit.Path
	if path == "" {
		return
	}
	logger := &audit.Logger{Path: expandHome(path)}
	if err := logger.Append(call, dec); err != nil {
		fmt.Fprintf(stderr, "doupass: audit error: %v\n", err)
	}
}

func homeDir() string {
	h, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return h
}

func expandHome(p string) string {
	home := homeDir()
	if home == "" {
		return p
	}
	if p == "~" {
		return home
	}
	if strings.HasPrefix(p, "~/") || strings.HasPrefix(p, "~\\") {
		return filepath.Join(home, p[2:])
	}
	return p
}

func findPolicyFile(flagValue string) (string, error) {
	if flagValue != "" {
		return flagValue, nil
	}
	candidates := []string{"doupass.yml"}
	if home := homeDir(); home != "" {
		candidates = append(candidates, filepath.Join(home, ".doupass", "doupass.yml"))
	}
	for _, c := range candidates {
		if _, err := os.Stat(c); err == nil {
			return c, nil
		}
	}
	return "", fmt.Errorf("no policy file found (looked in %s); pass --policy", strings.Join(candidates, ", "))
}

func loadEngine(policyPath string) (*policy.Engine, error) {
	p, err := policy.LoadFile(policyPath)
	if err != nil {
		return nil, err
	}
	wd, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	return policy.New(p, policy.Options{
		Home:         homeDir(),
		Workspace:    wd,
		CaseFold:     runtime.GOOS == "windows",
		EvalSymlinks: policy.OSResolver(),
	})
}
