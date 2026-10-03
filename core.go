package main

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

type SystemInfo struct {
	OS                   string   `json:"os"`
	OSVersion            string   `json:"os_version,omitempty"`
	HardwareModel        string   `json:"hardware_model,omitempty"`
	CPUModel             string   `json:"cpu_model,omitempty"`
	Arch                 string   `json:"arch"`
	CPUThreads           int      `json:"cpu_threads"`
	MemoryBytes          uint64   `json:"memory_bytes"`
	AvailableMemoryBytes uint64   `json:"available_memory_bytes,omitempty"`
	PortableRoot         string   `json:"portable_root"`
	Version              string   `json:"version"`
	Volumes              []Volume `json:"volumes"`
	Notes                []string `json:"notes"`
}

type Volume struct {
	Path       string `json:"path"`
	Label      string `json:"label"`
	FileSystem string `json:"file_system"`
	Removable  bool   `json:"removable"`
	SizeBytes  uint64 `json:"size_bytes"`
	FreeBytes  uint64 `json:"free_bytes"`
}

func inspectSystem(root string) (SystemInfo, error) {
	memory, available, volumes, notes := platformInfo(root)
	info := SystemInfo{OS: runtime.GOOS, Arch: runtime.GOARCH, CPUThreads: runtime.NumCPU(), MemoryBytes: memory, AvailableMemoryBytes: available, PortableRoot: root, Version: version, Volumes: volumes, Notes: notes}
	hostDetails(&info)
	return info, nil
}

func inspectNetwork() (any, error) {
	interfaces, err := net.Interfaces()
	if err != nil {
		return nil, err
	}
	rows := []map[string]any{}
	for _, n := range interfaces {
		if n.Flags&net.FlagLoopback != 0 {
			continue
		}
		addresses, _ := n.Addrs()
		ips := []string{}
		for _, a := range addresses {
			ips = append(ips, a.String())
		}
		rows = append(rows, map[string]any{"name": n.Name, "up": n.Flags&net.FlagUp != 0, "addresses": ips})
	}
	return map[string]any{"interfaces": rows, "internet_verified": false, "note": "网卡启用不代表互联网可用；本次只读检查，不修改 DNS、代理或防火墙。"}, nil
}

func writablePath(root string, parts ...string) (string, error) {
	current := root
	for i, part := range parts {
		if part == "" || part == "." || part == ".." || strings.ContainsAny(part, "/\\:") {
			return "", errors.New("无效的便携路径")
		}
		current = filepath.Join(current, part)
		info, err := os.Lstat(current)
		if err == nil {
			if info.Mode()&os.ModeSymlink != 0 {
				return "", errors.New("便携数据路径不能通过链接写入其他目录")
			}
			if i < len(parts)-1 && !info.IsDir() {
				return "", errors.New("便携数据目录被文件占用")
			}
		} else if !errors.Is(err, os.ErrNotExist) {
			return "", err
		}
	}
	return current, nil
}

func newFileAtomic(root string, parts []string, content []byte) (string, error) {
	path, err := writablePath(root, parts...)
	if err != nil {
		return "", err
	}
	if err = os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return "", err
	}
	if _, err = os.Lstat(path); !errors.Is(err, os.ErrNotExist) {
		return "", errors.New("目标文件已存在，不覆盖原文件")
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".xiapan-*.tmp")
	if err != nil {
		return "", err
	}
	tempName := tmp.Name()
	defer os.Remove(tempName)
	if _, err = tmp.Write(content); err == nil {
		err = tmp.Sync()
	}
	closeErr := tmp.Close()
	if err == nil {
		err = closeErr
	}
	if err != nil {
		return "", err
	}
	if err = os.Rename(tempName, path); err != nil {
		return "", err
	}
	return path, nil
}

func uniqueID() string {
	b := make([]byte, 6)
	rand.Read(b)
	return time.Now().Format("20060102-150405") + "-" + hex.EncodeToString(b)
}

func createReport(root string, inputs ...map[string]any) (any, error) {
	sys, err := inspectSystem(root)
	if err != nil {
		return nil, err
	}
	id := uniqueID()
	report := map[string]any{"id": id, "created_at": time.Now().Format(time.RFC3339), "system": sys, "scope": "本机只读检测；未执行修复、格式化、重装或云端模型调用。"}
	if len(inputs) > 0 && str(inputs[0], "session_id") != "" {
		s, err := loadChat(root, str(inputs[0], "session_id"))
		if err != nil {
			return nil, err
		}
		evidence := []map[string]any{}
		for _, m := range s.Messages {
			if m.Role == "action" && chatReadActions[m.Action] && m.Action != "reports.list" {
				evidence = append(evidence, map[string]any{"action": m.Action, "data": m.Data})
			}
		}
		report["session_id"], report["session_updated_at"] = s.ID, s.Updated
		report["evidence"] = evidence
		report["evidence_note"] = "会话中已记录的检测证据；跨电脑继续的会话可能包含其他电脑的结果，请核对后使用。"
	}
	content, _ := json.MarshalIndent(report, "", "  ")
	path, err := newFileAtomic(root, []string{"data", "reports", id + ".json"}, content)
	if err != nil {
		return nil, err
	}
	rel, _ := filepath.Rel(root, path)
	return map[string]any{"id": id, "path": filepath.ToSlash(rel), "report": report}, nil
}

func listReports(root string) (any, error) {
	dir, err := writablePath(root, "data", "reports")
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(dir)
	if errors.Is(err, os.ErrNotExist) {
		return []any{}, nil
	}
	if err != nil {
		return nil, err
	}
	rows := []any{}
	for i := len(entries) - 1; i >= 0 && len(rows) < 50; i-- {
		e := entries[i]
		if e.Type()&os.ModeSymlink != 0 || e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		info, err := e.Info()
		if err != nil || info.Size() > 2<<20 {
			continue
		}
		b, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			continue
		}
		var r map[string]any
		if json.Unmarshal(b, &r) == nil {
			rows = append(rows, r)
		}
	}
	return rows, nil
}

func portableEnv(root string) ([]string, error) {
	env := []string{}
	for _, entry := range os.Environ() {
		key := strings.ToUpper(strings.SplitN(entry, "=", 2)[0])
		if key == "TMP" || key == "TEMP" || key == "TMPDIR" {
			continue
		}
		env = append(env, entry)
	}
	temp, err := writablePath(root, "data", "tmp")
	if err != nil {
		return nil, err
	}
	if err = os.MkdirAll(temp, 0700); err != nil {
		return nil, err
	}
	return append(env, "TMP="+temp, "TEMP="+temp, "TMPDIR="+temp), nil
}
