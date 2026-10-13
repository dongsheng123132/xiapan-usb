//go:build windows

package main

import (
	"context"
	"os/exec"
	"path/filepath"
	"strconv"
	"time"
)

func preparePicoProcess(cmd *exec.Cmd) { quietProcess(cmd) }
func killPicoProcessTree(cmd *exec.Cmd) {
	// The launcher may restart a gateway and own that replacement as a child.
	// Select only the exec.Cmd we created, never a PID read from the USB.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	killer := exec.CommandContext(ctx, filepath.Join(nativeSystemDir(), "taskkill.exe"), "/PID", strconv.Itoa(cmd.Process.Pid), "/T", "/F")
	quietProcess(killer)
	if killer.Run() != nil {
		_ = cmd.Process.Kill()
	}
}
