//go:build !windows

package main

import (
	"os/exec"
	"syscall"
	"time"
)

type picoProcessControl struct{ cmd *exec.Cmd }

func preparePicoProcess(cmd *exec.Cmd) { cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true} }
func newPicoProcessControl(cmd *exec.Cmd) (*picoProcessControl, error) {
	return &picoProcessControl{cmd: cmd}, nil
}
func (c *picoProcessControl) alive() bool { return c.cmd.Process.Signal(syscall.Signal(0)) == nil }
func (c *picoProcessControl) kill() {
	// Signal uses the original os.Process lifecycle lock. Once Wait has reaped
	// it, Signal refuses an old/reused PID. Stop the live group leader first so
	// it cannot exit and surrender our process-group identity before cleanup.
	if c.cmd.Process.Signal(syscall.SIGSTOP) != nil {
		return
	}
	deadline := time.Now().Add(500 * time.Millisecond)
	for time.Now().Before(deadline) {
		var state syscall.WaitStatus
		pid, err := syscall.Wait4(c.cmd.Process.Pid, &state, syscall.WNOHANG|syscall.WUNTRACED, nil)
		if err != nil {
			return
		}
		if pid == c.cmd.Process.Pid {
			// Only a confirmed stopped child keeps the original PID/group
			// reserved. An exited child can have been reaped; never use its
			// former PGID to send a signal.
			// Go's BSD WaitStatus methods classify a SIGSTOP report as
			// Continued. We requested WUNTRACED without WCONTINUED, so
			// the POSIX stop tag identifies a genuinely stopped child.
			if state&0xff == 0x7f {
				if syscall.Kill(-pid, syscall.SIGKILL) != nil {
					_ = c.cmd.Process.Kill()
				}
			}
			return
		}
		time.Sleep(time.Millisecond)
	}
	_ = c.cmd.Process.Signal(syscall.SIGCONT)
}
func (c *picoProcessControl) close() {}

// Native launcher starts the replacement directly, without a new process group.
// These read-only probes never authorize killing a PID read from the USB.
func picoLauncherOwnsProcess(pid, launcherPID int) bool {
	group, err := syscall.Getpgid(pid)
	return err == nil && group == launcherPID && picoProcessExists(pid)
}
func picoProcessExists(pid int) bool { return pid > 0 && syscall.Kill(pid, 0) == nil }
