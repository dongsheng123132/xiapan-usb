//go:build !windows

package main

import (
	"context"
	"os/exec"
	"syscall"
)

func launchElevatedTool(ctx context.Context, program string, args []string, cause error) (int, error) {
	return 0, cause
}

func quietProcess(cmd *exec.Cmd) {}

// A detached child survives the launcher and a closed terminal.
func detachProcess(cmd *exec.Cmd) { cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true} }

func nativeSystemDir() string { return "" }
