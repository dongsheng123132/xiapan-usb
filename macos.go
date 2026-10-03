package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"
)

// Apple-provided apps opened through LaunchServices. The bundle and optional
// settings URL are fixed here; page or model input never selects either.
var macTools = []BuiltinTool{
	{"activity-monitor", "活动监视器", "查看进程、CPU、内存、能耗与网络占用；是否结束进程由你在工具内决定。", "/System/Applications/Utilities/Activity Monitor.app", nil},
	{"login-items", "登录项设置", "查看登录项与后台项目，是否关闭由你在设置中决定。", "/System/Applications/System Settings.app", []string{"x-apple.systempreferences:com.apple.LoginItems-Settings.extension"}},
	{"storage-settings", "存储空间设置", "查看空间占用与系统建议；删除前由你在设置中确认。", "/System/Applications/System Settings.app", []string{"x-apple.systempreferences:com.apple.settings.Storage"}},
	{"disk-utility", "磁盘工具", "查看磁盘、分区并手动急救；不自动抹盘、分区或修复。", "/System/Applications/Utilities/Disk Utility.app", nil},
	{"system-information", "系统信息", "查看硬件、存储、USB 与网络详情。", "/System/Applications/Utilities/System Information.app", nil},
	{"console", "控制台", "查看系统与应用日志、崩溃报告。", "/System/Applications/Utilities/Console.app", nil},
	{"wireless-diagnostics", "无线诊断", "检查 Wi-Fi 连接并生成诊断报告。", "/System/Library/CoreServices/Applications/Wireless Diagnostics.app", nil},
}

func nativeTools() []BuiltinTool {
	switch runtime.GOOS {
	case "windows":
		return builtinTools
	case "darwin":
		return macTools
	}
	return nil
}

func osLabel(goos string) string {
	switch goos {
	case "windows":
		return "Windows"
	case "darwin":
		return "macOS"
	case "linux":
		return "Linux"
	}
	return goos
}

// Each platform may embed its own pinned runtime manifest, for example
// catalog/picoclaw.darwin-arm64.json. The unsuffixed file stays the Windows one.
func platformCatalog(name string) ([]byte, error) {
	if b, err := assets.ReadFile("catalog/" + name + "." + runtime.GOOS + "-" + runtime.GOARCH + ".json"); err == nil {
		return b, nil
	}
	return assets.ReadFile("catalog/" + name + ".json")
}

// The macOS app keeps its executable in 虾盘.app/Contents/MacOS. The portable
// root is the folder holding the bundle, beside app/ and data/.
func bundleRoot(dir string) (string, bool) {
	contents := filepath.Dir(dir)
	bundle := filepath.Dir(contents)
	if filepath.Base(dir) == "MacOS" && filepath.Base(contents) == "Contents" && strings.HasSuffix(strings.ToLower(bundle), ".app") {
		return filepath.Dir(bundle), true
	}
	return dir, false
}

const removableHelp = "macOS 没有允许虾盘访问这只 U 盘。请打开「系统设置 › 隐私与安全性 › 文件与文件夹」，为「虾盘」打开「可移除宗卷」，然后重新打开虾盘。"

const translocationHelp = "macOS 正在隔离运行虾盘（App Translocation），程序看不到 U 盘里的 app/ 与 data/。请在访达中把「虾盘.app」拖到其他文件夹再拖回原处，然后重新打开；不需要关闭系统安全功能。"

// Older LaunchServices versions may pass a process serial number argument.
func withoutProcessSerial(args []string) []string {
	out := []string{}
	for _, a := range args {
		if !strings.HasPrefix(a, "-psn_") {
			out = append(out, a)
		}
	}
	return out
}

func plistJSON(ctx context.Context, path string, data []byte) ([]byte, error) {
	cmd := exec.CommandContext(ctx, "/usr/bin/plutil", "-convert", "json", "-o", "-", path)
	if data != nil {
		cmd.Stdin = bytes.NewReader(data)
	}
	return cmd.Output()
}

var disabledLine = regexp.MustCompile(`^\s*"([^"]+)"\s*=>\s*(\w+)`)

// launchctl print-disabled prints enabled/disabled; older releases print false/true.
func parseLaunchctlDisabled(out string) map[string]bool {
	overrides := map[string]bool{}
	for _, line := range strings.Split(out, "\n") {
		if g := disabledLine.FindStringSubmatch(line); g != nil {
			switch g[2] {
			case "disabled", "true":
				overrides[g[1]] = true
			case "enabled", "false":
				overrides[g[1]] = false
			}
		}
	}
	return overrides
}

// Program paths and arguments are not returned, matching the Windows reader.
func launchItemRow(file, source string, plist []byte, overrides map[string]bool) map[string]any {
	row := map[string]any{"name": strings.TrimSuffix(file, ".plist"), "source": source, "state": "unknown", "trigger": "无法读取配置"}
	var item map[string]any
	if json.Unmarshal(plist, &item) != nil {
		return row
	}
	label, _ := item["Label"].(string)
	if label != "" {
		row["name"] = label
	}
	state := "enabled"
	if disabled, ok := overrides[label]; ok && label != "" {
		if disabled {
			state = "disabled"
		}
	} else if item["Disabled"] == true {
		state = "disabled"
	}
	row["state"] = state
	keepAlive, _ := item["KeepAlive"].(map[string]any)
	switch {
	case item["RunAtLoad"] == true || item["KeepAlive"] == true || len(keepAlive) > 0:
		row["trigger"] = "登录或开机即运行"
	case item["StartInterval"] != nil || item["StartCalendarInterval"] != nil:
		row["trigger"] = "定时运行"
	default:
		row["trigger"] = "按需或条件触发"
	}
	return row
}

// Login Items shown in System Settings need administrator rights to enumerate
// (sfltool dumpbtm); Apple's own /System services are outside this view.
func inspectLaunchItems(ctx context.Context) (any, error) {
	deadline, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	readOverrides := func(domain string) map[string]bool {
		out, _ := exec.CommandContext(deadline, "/bin/launchctl", "print-disabled", domain).Output()
		return parseLaunchctlDisabled(string(out))
	}
	user, system := readOverrides(fmt.Sprintf("gui/%d", os.Getuid())), readOverrides("system")
	home, _ := os.UserHomeDir()
	locations := []struct {
		dir, source string
		overrides   map[string]bool
	}{
		{filepath.Join(home, "Library", "LaunchAgents"), "当前用户 · LaunchAgents", user},
		{"/Library/LaunchAgents", "所有用户 · LaunchAgents", user},
		{"/Library/LaunchDaemons", "系统 · LaunchDaemons", system},
	}
	rows, warnings := []map[string]any{}, []string{}
	for _, loc := range locations {
		if home == "" && loc.source == "当前用户 · LaunchAgents" {
			continue
		}
		entries, err := os.ReadDir(loc.dir)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			warnings = append(warnings, loc.source+" 无法读取")
			continue
		}
		for _, e := range entries {
			if e.IsDir() || !strings.HasSuffix(e.Name(), ".plist") {
				continue
			}
			if len(rows) >= 300 {
				warnings = append(warnings, "启动项过多，只列出前 300 项")
				break
			}
			b, err := plistJSON(deadline, filepath.Join(loc.dir, e.Name()), nil)
			if err != nil {
				b = nil
			}
			rows = append(rows, launchItemRow(e.Name(), loc.source, b, loc.overrides))
		}
	}
	if deadline.Err() != nil {
		return nil, errors.New("启动项读取超时，未修改系统")
	}
	return map[string]any{"items": rows, "warnings": warnings, "scope": "~/Library/LaunchAgents、/Library/LaunchAgents 与 /Library/LaunchDaemons",
		"note": "只读检测，未停用或卸载任何项目。状态取自 launchctl 的启用记录；不包含系统设置「登录项」中的 App（读取需管理员权限）及 Apple 系统服务。"}, nil
}

func parseHardwarePorts(out string) []map[string]any {
	rows := []map[string]any{}
	var current map[string]any
	for _, line := range strings.Split(out, "\n") {
		key, value, ok := strings.Cut(strings.TrimSpace(line), ": ")
		if !ok {
			continue
		}
		switch key {
		case "Hardware Port":
			current = map[string]any{"name": value}
			rows = append(rows, current)
		case "Device":
			if current != nil {
				current["device"] = value
			}
		}
	}
	return rows
}

func macNetworkDevices(ctx context.Context) []map[string]any {
	deadline, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	out, err := exec.CommandContext(deadline, "/usr/sbin/networksetup", "-listallhardwareports").Output()
	if err != nil {
		return []map[string]any{}
	}
	return parseHardwarePorts(string(out))
}

func rescueAdvice() string {
	if runtime.GOOS == "darwin" {
		return "Mac 无法进入系统时使用 macOS 恢复功能：Apple 芯片机型关机后按住电源键，直到出现启动选项；Intel 机型开机时按住 Command-R。可先用磁盘工具急救，再从时间机器恢复或重新安装 macOS。先备份重要数据；当前程序不写引导、不抹盘、不重装。"
	}
	return "如果电脑无法进入系统，需要从独立的 WinPE 或 Linux 救援环境启动。先备份重要数据，再辨认目标磁盘。当前程序不写引导、不重装；资源页可查看已准备的镜像。格式化成 exFAT 本身不会让 U 盘可启动。"
}

func agentToolHints() string {
	if runtime.GOOS == "darwin" {
		return "这台是 Mac：开机自启可申请 login-items，进程与内存占用可申请 activity-monitor，清理空间先检查磁盘再申请 storage-settings，磁盘检查用 disk-utility，Wi-Fi 问题可申请 wireless-diagnostics，日志用 console。macOS 网卡驱动由系统提供，一般不需要离线驱动包；不要建议 Windows 专用工具。"
	}
	return "空间分析可申请 windirstat，压缩解压可申请 peazip，先看工具清单确认是否已准备。清理 C 盘可先检查磁盘再申请打开 disk-cleanup；驱动修复先识别硬件与错误码再申请 device-manager。"
}

func hostLabel(sys SystemInfo) string {
	if sys.OSVersion != "" {
		return osLabel(sys.OS) + " " + sys.OSVersion + " / " + sys.Arch
	}
	return sys.OS + "/" + sys.Arch
}
