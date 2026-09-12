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
	//#nosec G204 -- PID comes from the process doupass itself spawned
	_ = exec.Command("taskkill", "/T", "/F", "/PID", strconv.Itoa(cmd.Process.Pid)).Run()
}
