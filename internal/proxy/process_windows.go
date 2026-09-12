//go:build windows

package proxy

import (
	"os/exec"
	"strconv"
)

func configureProcess(cmd *exec.Cmd) {}

func killTree(cmd *exec.Cmd) {
	if cmd.Process == nil {
		return
	}
	_ = exec.Command("taskkill", "/T", "/F", "/PID", strconv.Itoa(cmd.Process.Pid)).Run()
}
