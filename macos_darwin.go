package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

const (
	mntReadOnly   = 0x00000001
	mntLocal      = 0x00001000
	mntDontBrowse = 0x00100000
	mntNoWait     = 2
)

func sysctlValues(ctx context.Context, keys ...string) map[string]string {
	// Unknown keys are reported on stderr; the remaining keys still print.
	out, _ := exec.CommandContext(ctx, "/usr/sbin/sysctl", keys...).Output()
	values := map[string]string{}
	for _, line := range strings.Split(string(out), "\n") {
		if key, value, ok := strings.Cut(line, ": "); ok {
			values[key] = strings.TrimSpace(value)
		}
	}
	return values
}

func platformInfo(root string) (uint64, uint64, []Volume, []string) {
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	values := sysctlValues(ctx, "hw.memsize", "kern.memorystatus_level")
	memory, _ := strconv.ParseUint(values["hw.memsize"], 10, 64)
	var available uint64
	// The kernel's free-memory estimate, the figure memory_pressure reports.
	if level, err := strconv.ParseUint(values["kern.memorystatus_level"], 10, 64); err == nil && level <= 100 {
		available = memory / 100 * level
	}
	volumes, notes := macVolumes(ctx, root)
	return memory, available, volumes, notes
}

func hostDetails(info *SystemInfo) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	values := sysctlValues(ctx, "kern.osproductversion", "hw.model", "machdep.cpu.brand_string")
	info.OSVersion, info.HardwareModel, info.CPUModel = values["kern.osproductversion"], values["hw.model"], values["machdep.cpu.brand_string"]
}

func cString(b []int8) string {
	out := make([]byte, 0, len(b))
	for _, c := range b {
		if c == 0 {
			break
		}
		out = append(out, byte(c))
	}
	return string(out)
}

func mounts() []syscall.Statfs_t {
	n, err := syscall.Getfsstat(nil, mntNoWait)
	if err != nil || n <= 0 {
		return nil
	}
	buf := make([]syscall.Statfs_t, n+8)
	if n, err = syscall.Getfsstat(buf, mntNoWait); err != nil {
		return nil
	}
	return buf[:n]
}

func fileSystemName(kind string) string {
	switch kind {
	case "apfs":
		return "APFS"
	case "hfs":
		return "HFS+"
	case "exfat":
		return "exFAT"
	case "msdos":
		return "FAT"
	case "ntfs":
		return "NTFS"
	}
	return kind
}

func macVolumes(ctx context.Context, root string) ([]Volume, []string) {
	volumes, notes := []Volume{}, []string{}
	for _, m := range mounts() {
		mount := cString(m.Mntonname[:])
		if mount != "/" && !strings.HasPrefix(mount, "/Volumes/") || m.Flags&mntLocal == 0 || m.Flags&mntDontBrowse != 0 {
			continue
		}
		var info map[string]any
		if xml, err := exec.CommandContext(ctx, "/usr/sbin/diskutil", "info", "-plist", cString(m.Mntfromname[:])).Output(); err == nil {
			if b, err := plistJSON(ctx, "-", xml); err == nil {
				json.Unmarshal(b, &info)
			}
		}
		// Mounted installers and other disk images are not drives to maintain.
		if info["BusProtocol"] == "Disk Image" {
			continue
		}
		label, _ := info["VolumeName"].(string)
		if label == "" {
			label = filepath.Base(mount)
		}
		v := Volume{Path: mount, Label: label, FileSystem: fileSystemName(cString(m.Fstypename[:])), Removable: info["Internal"] == false || info["RemovableMedia"] == true,
			SizeBytes: m.Blocks * uint64(m.Bsize), FreeBytes: m.Bavail * uint64(m.Bsize)}
		volumes = append(volumes, v)
		if v.Removable && v.FileSystem == "NTFS" {
			notes = append(notes, "检测到 NTFS 移动盘「"+label+"」：macOS 默认只能读取、不能写入。")
		}
	}
	var st syscall.Statfs_t
	if syscall.Statfs(root, &st) == nil && st.Flags&mntReadOnly != 0 {
		notes = append(notes, "虾盘所在位置为只读（"+fileSystemName(cString(st.Fstypename[:]))+"），设置、对话和报告无法保存。Mac 不能直接写入 NTFS；要在 Windows 与 Mac 间共用，建议使用 exFAT（重新格式化会清空 U 盘，请先备份）。")
	}
	return volumes, notes
}

// App Translocation runs a quarantined bundle from a read-only nullfs mirror.
// The mount table still names the original bundle on the USB drive.
func translocatedOrigin(path string) (string, bool) {
	if real, err := filepath.EvalSymlinks(path); err == nil {
		path = real
	}
	if !strings.Contains(path, "/AppTranslocation/") {
		return "", false
	}
	for _, m := range mounts() {
		if cString(m.Fstypename[:]) != "nullfs" {
			continue
		}
		if rel, ok := strings.CutPrefix(path, cString(m.Mntonname[:])+"/"); ok {
			origin := filepath.Join(cString(m.Mntfromname[:]), rel)
			if _, err := os.Stat(origin); err == nil {
				return origin, true
			}
		}
	}
	return "", false
}

func bundleLog(root string) *os.File {
	if path, err := writablePath(root, "data", "logs", "app.log"); err == nil && os.MkdirAll(filepath.Dir(path), 0700) == nil {
		flags := os.O_CREATE | os.O_WRONLY | os.O_APPEND
		if info, err := os.Lstat(path); err == nil && info.Size() > 1<<20 {
			flags |= os.O_TRUNC
		}
		if f, err := os.OpenFile(path, flags, 0600); err == nil {
			return f
		}
	}
	f, _ := os.OpenFile(os.DevNull, os.O_WRONLY, 0)
	return f
}

// Finder starts a bundle once; opening it again only sends a reopen event that
// a plain process ignores. The launcher starts a detached service and exits, so
// every double-click opens the browser like the Windows entry does.
func launchBundle(exe string) error {
	root, err := resolveRoot("")
	if err != nil {
		return err
	}
	// The first read of a USB drive waits for the user's privacy decision.
	if _, err = os.ReadDir(root); errors.Is(err, fs.ErrPermission) {
		return errors.New(removableHelp)
	}
	reservation, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return err
	}
	port := reservation.Addr().(*net.TCPAddr).Port
	reservation.Close()
	logFile := bundleLog(root)
	defer logFile.Close()
	cmd := exec.Command(exe, "serve", "--root", root, "--port", strconv.Itoa(port))
	cmd.Dir = root
	cmd.Stdout, cmd.Stderr = logFile, logFile
	detachProcess(cmd)
	if err = cmd.Start(); err != nil {
		return fmt.Errorf("无法启动虾盘服务：%w", err)
	}
	exited := make(chan struct{})
	go func() { cmd.Wait(); close(exited) }()
	client, address := localHTTP(time.Second), fmt.Sprintf("http://127.0.0.1:%d/api/actions", port)
	timeout := time.After(20 * time.Second)
	for {
		select {
		case <-exited:
			return errors.New("虾盘服务启动后退出，详情见本盘 data/logs/app.log")
		case <-timeout:
			cmd.Process.Kill()
			return errors.New("虾盘服务启动超时，已停止本次启动")
		case <-time.After(150 * time.Millisecond):
			if resp, err := client.Get(address); err == nil {
				resp.Body.Close()
				if resp.StatusCode == http.StatusOK {
					return nil
				}
			}
		}
	}
}

func alertUser(title, message string) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	// Text travels as argv, never as AppleScript source.
	exec.CommandContext(ctx, "/usr/bin/osascript", "-e", "on run argv", "-e", "display alert (item 1 of argv) message (item 2 of argv) as critical giving up after 170", "-e", "end run", title, message).Run()
}
