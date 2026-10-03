//go:build !windows

package main

import (
	"context"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"time"
)

func launchElevatedTool(ctx context.Context, program string, args []string, cause error) (int, error) {
	return 0, cause
}

func quietProcess(cmd *exec.Cmd) {}

func nativeSystemDir() string { return "" }

func platformInfo(root string) (uint64, uint64, []Volume, []string) {
	var memory, available uint64
	if runtime.GOOS == "linux" {
		b, _ := os.ReadFile("/proc/meminfo")
		for _, line := range strings.Split(string(b), "\n") {
			if strings.HasPrefix(line, "MemTotal:") {
				f := strings.Fields(line)
				if len(f) >= 2 {
					n, _ := strconv.ParseUint(f[1], 10, 64)
					memory = n * 1024
				}
			}
			if strings.HasPrefix(line, "MemAvailable:") {
				f := strings.Fields(line)
				if len(f) >= 2 {
					n, _ := strconv.ParseUint(f[1], 10, 64)
					available = n * 1024
				}
			}
		}
	}
	if runtime.GOOS == "darwin" {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		b, _ := exec.CommandContext(ctx, "sysctl", "-n", "hw.memsize").Output()
		memory, _ = strconv.ParseUint(strings.TrimSpace(string(b)), 10, 64)
	}
	return memory, available, []Volume{}, []string{"此平台已提供体检与便携路径实现，磁盘详细信息和整机运行仍需对应平台实测。"}
}
