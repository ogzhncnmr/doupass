//go:build !windows

package proxy

import (
	"os/exec"
	"syscall"
)

type processGroup struct{}

func newProcessGroup() *processGroup { return &processGroup{} }

func (g *processGroup) configure(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

func (g *processGroup) attach(_ *exec.Cmd) error { return nil }

func (g *processGroup) kill(cmd *exec.Cmd) {
	if cmd.Process == nil {
		return
	}
	_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
}

func (g *processGroup) close() {}
