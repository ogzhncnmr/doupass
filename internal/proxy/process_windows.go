//go:build windows

package proxy

import (
	"os/exec"
	"strconv"
	"unsafe"

	"golang.org/x/sys/windows"
)

// processGroup tracks the downstream server in a Windows Job Object with
// kill-on-close, so the whole tree dies even if doupass itself crashes.
type processGroup struct {
	job windows.Handle
}

func newProcessGroup() *processGroup {
	g := &processGroup{}
	job, err := windows.CreateJobObject(nil, nil)
	if err != nil {
		return g
	}
	info := windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION{
		BasicLimitInformation: windows.JOBOBJECT_BASIC_LIMIT_INFORMATION{
			LimitFlags: windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE,
		},
	}
	//#nosec G103 -- the uintptr conversion is required by the SetInformationJobObject Win32 signature
	if _, err := windows.SetInformationJobObject(job, windows.JobObjectExtendedLimitInformation, uintptr(unsafe.Pointer(&info)), uint32(unsafe.Sizeof(info))); err != nil {
		_ = windows.CloseHandle(job)
		return g
	}
	g.job = job
	return g
}

func (g *processGroup) configure(_ *exec.Cmd) {}

func (g *processGroup) attach(cmd *exec.Cmd) error {
	if g.job == 0 || cmd.Process == nil {
		return nil
	}
	//#nosec G115 -- Windows PIDs are unsigned 32-bit by definition
	h, err := windows.OpenProcess(windows.PROCESS_SET_QUOTA|windows.PROCESS_TERMINATE, false, uint32(cmd.Process.Pid))
	if err != nil {
		return err
	}
	defer windows.CloseHandle(h)
	return windows.AssignProcessToJobObject(g.job, h)
}

// detach drops the job when assignment fails so kill falls back to taskkill;
// otherwise TerminateJobObject would be a no-op for a process outside the job.
func (g *processGroup) detach() {
	if g.job != 0 {
		_ = windows.CloseHandle(g.job)
		g.job = 0
	}
}

func (g *processGroup) kill(cmd *exec.Cmd) {
	if cmd.Process == nil {
		return
	}
	if g.job != 0 {
		_ = windows.TerminateJobObject(g.job, 1)
		return
	}
	//#nosec G204 -- PID comes from the process doupass itself spawned
	_ = exec.Command("taskkill", "/T", "/F", "/PID", strconv.Itoa(cmd.Process.Pid)).Run()
}

func (g *processGroup) close() {
	g.detach()
}
