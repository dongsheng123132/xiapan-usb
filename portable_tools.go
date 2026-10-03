package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

type PortableTool struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Version     string            `json:"version"`
	Platform    string            `json:"platform"`
	Directory   string            `json:"directory"`
	Executable  string            `json:"executable"`
	License     string            `json:"license"`
	OfficialURL string            `json:"official_url"`
	Files       map[string]string `json:"files"`
}

func portableTools() []PortableTool {
	b, err := assets.ReadFile("catalog/portable-tools.json")
	var rows []PortableTool
	if err != nil || json.Unmarshal(b, &rows) != nil {
		return nil
	}
	return rows
}

func portableToolPath(root string, tool PortableTool, relative string) (string, error) {
	parts := append(strings.Split(tool.Directory, "/"), strings.Split(relative, "/")...)
	return writablePath(root, parts...)
}

func verifyPortableTool(ctx context.Context, root string, tool PortableTool) (string, error) {
	if tool.Platform != runtime.GOOS+"/"+runtime.GOARCH {
		return "", errors.New("此工具未提供当前系统的便携版本")
	}
	if tool.Files[tool.Executable] == "" || len(tool.Files) == 0 {
		return "", errors.New("工具入口未列入固定校验清单")
	}
	for relative, expected := range tool.Files {
		if ctx.Err() != nil {
			return "", ctx.Err()
		}
		path, err := portableToolPath(root, tool, relative)
		if err != nil {
			return "", err
		}
		f, err := os.Open(path)
		if err != nil {
			return "", errors.New("此工具未随盘准备完整")
		}
		info, statErr := f.Stat()
		if statErr != nil || !info.Mode().IsRegular() || info.Size() > 128<<20 {
			f.Close()
			return "", errors.New("工具文件类型或大小异常")
		}
		h := sha256.New()
		_, copyErr := io.Copy(h, io.LimitReader(f, 128<<20+1))
		f.Close()
		if copyErr != nil || hex.EncodeToString(h.Sum(nil)) != expected {
			return "", fmt.Errorf("%s 文件校验不符；请重新准备官方工具包", tool.Name)
		}
	}
	return portableToolPath(root, tool, tool.Executable)
}

func portableToolRows(root string) []any {
	rows := []any{}
	for _, tool := range portableTools() {
		// The list checks presence only. Every immutable file is verified before launch.
		path, err := portableToolPath(root, tool, tool.Executable)
		ready := err == nil && tool.Platform == runtime.GOOS+"/"+runtime.GOARCH
		if ready {
			info, statErr := os.Stat(path)
			ready = statErr == nil && info.Mode().IsRegular()
		}
		rows = append(rows, map[string]any{"id": tool.ID, "name": tool.Name, "version": tool.Version,
			"ready": ready, "source": "本盘便携工具", "platform": tool.Platform,
			"license": tool.License, "official_url": tool.OfficialURL, "action": "tools.launch"})
	}
	return rows
}

func launchRegisteredTool(ctx context.Context, root, id string, confirmed bool) (any, error) {
	for _, tool := range portableTools() {
		if id != tool.ID {
			continue
		}
		if !confirmed {
			return nil, errors.New("请先确认打开所选便携工具")
		}
		program, err := verifyPortableTool(ctx, root, tool)
		if err != nil {
			return nil, err
		}
		if tool.ID == "windirstat" {
			// The upstream app reads the registry unless its adjacent ini exists.
			ini, pathErr := portableToolPath(root, tool, "WinDirStat.ini")
			if pathErr != nil {
				return nil, pathErr
			}
			f, openErr := os.OpenFile(ini, os.O_CREATE|os.O_RDWR, 0600)
			if openErr != nil {
				return nil, errors.New("无法启用随盘配置；未启动工具")
			}
			f.Close()
		}
		// Only pinned GUI entrypoints are allowed, with no model-supplied arguments.
		cmd := exec.Command(program)
		cmd.Dir = filepath.Dir(program)
		cmd.Env, err = portableEnv(root)
		if err != nil {
			return nil, err
		}
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		if err = cmd.Start(); err != nil {
			return nil, fmt.Errorf("打开 %s 失败：%w", tool.Name, err)
		}
		pid := cmd.Process.Pid
		go cmd.Wait()
		return map[string]any{"tool_id": id, "name": tool.Name, "pid": pid, "verified": true,
			"launch_requested": true, "maintenance_performed": false,
			"note": "已校验并启动本盘便携工具；后续文件操作由你在工具中选择。"}, nil
	}
	return launchTool(ctx, id, confirmed)
}
