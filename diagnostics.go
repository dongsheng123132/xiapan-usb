package main

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"
)

type ProcessInfo struct {
	PID    int    `json:"pid"`
	Name   string `json:"name"`
	Memory uint64 `json:"memory_bytes"`
}

func inspectProcesses(ctx context.Context, root string) (any, error) {
	rows := []ProcessInfo{}
	if runtime.GOOS == "windows" {
		script := `Get-Process | Sort-Object WorkingSet64 -Descending | Select-Object -First 20 @{Name='pid';Expression={$_.Id}},@{Name='name';Expression={$_.ProcessName}},@{Name='memory_bytes';Expression={$_.WorkingSet64}} | ConvertTo-Json -Compress`
		deadline, cancel := context.WithTimeout(ctx, 8*time.Second)
		defer cancel()
		cmd := exec.CommandContext(deadline, "powershell.exe", "-NoProfile", "-NonInteractive", "-Command", script)
		quietProcess(cmd)
		var err error
		cmd.Env, err = portableEnv(root)
		if err != nil {
			return nil, err
		}
		b, err := cmd.Output()
		if err != nil {
			return nil, errors.New("进程检测暂时未完成；未修改或结束任何进程")
		}
		if json.Unmarshal(b, &rows) != nil {
			return nil, errors.New("进程检测结果格式异常")
		}
	} else if runtime.GOOS == "linux" {
		files, _ := os.ReadDir("/proc")
		for _, f := range files {
			pid, err := strconv.Atoi(f.Name())
			if err != nil {
				continue
			}
			b, err := os.ReadFile(filepath.Join("/proc", f.Name(), "status"))
			if err != nil {
				continue
			}
			p := ProcessInfo{PID: pid}
			for _, line := range strings.Split(string(b), "\n") {
				if strings.HasPrefix(line, "Name:") {
					p.Name = strings.TrimSpace(strings.TrimPrefix(line, "Name:"))
				}
				if strings.HasPrefix(line, "VmRSS:") {
					fields := strings.Fields(line)
					if len(fields) > 1 {
						n, _ := strconv.ParseUint(fields[1], 10, 64)
						p.Memory = n * 1024
					}
				}
			}
			rows = append(rows, p)
		}
	} else {
		deadline, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		b, err := exec.CommandContext(deadline, "ps", "-axo", "pid=,rss=,comm=").Output()
		if err != nil {
			return nil, errors.New("进程检测不可用")
		}
		for _, line := range strings.Split(string(b), "\n") {
			f := strings.Fields(line)
			if len(f) < 3 {
				continue
			}
			pid, _ := strconv.Atoi(f[0])
			rss, _ := strconv.ParseUint(f[1], 10, 64)
			rows = append(rows, ProcessInfo{pid, filepath.Base(strings.Join(f[2:], " ")), rss * 1024})
		}
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].Memory > rows[j].Memory })
	if len(rows) > 20 {
		rows = rows[:20]
	}
	return map[string]any{"processes": rows, "note": "按当前工作集内存排序，非实时 CPU 占用；未读取命令行、未结束进程。"}, nil
}
