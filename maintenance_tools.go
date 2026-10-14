package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

type BuiltinTool struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Platform    string   `json:"platform"`
	Category    string   `json:"category,omitempty"`
	Program     string   `json:"-"`
	Args        []string `json:"-"`
	AppPaths    []string `json:"-"`
}

var builtinTools = []BuiltinTool{
	{ID: "memory-diagnostic", Name: "Windows 内存诊断", Description: "打开内存诊断选择页；是否重启由你选择。", Platform: "windows", Program: "mdsched.exe"},
	{ID: "network-adapters", Name: "Windows 网络适配器", Description: "查看网络连接与适配器属性，不自动修改配置。", Platform: "windows", Program: "control.exe", Args: []string{"ncpa.cpl"}},
	{ID: "network-settings", Name: "Windows 网络设置", Description: "打开网络状态页；所有更改由你在系统页面选择。", Platform: "windows", Program: "rundll32.exe", Args: []string{"url.dll,FileProtocolHandler", "ms-settings:network-status"}},
	{ID: "task-manager", Name: "Windows 任务管理器", Description: "查看启动应用、进程与性能。", Platform: "windows", Program: "Taskmgr.exe"},
	{ID: "disk-cleanup", Name: "Windows 磁盘清理", Description: "由你选择清理项与目标磁盘；不自动删除文件。", Platform: "windows", Program: "cleanmgr.exe"},
	{ID: "device-manager", Name: "Windows 设备管理器", Description: "查看设备状态，手动选择已匹配的驱动；不自动装驱动。", Platform: "windows", Program: "mmc.exe", Args: []string{"devmgmt.msc"}},
	{ID: "system-info", Name: "Windows 系统信息", Description: "查看系统和硬件详情，工具由 Windows 提供。", Platform: "windows", Program: "msinfo32.exe"},
	{ID: "resource-monitor", Name: "Windows 资源监视器", Description: "查看 CPU、磁盘、内存和网络占用。", Platform: "windows", Program: "resmon.exe"},
	{ID: "event-viewer", Name: "Windows 事件查看器", Description: "查看系统与应用错误记录。", Platform: "windows", Program: "mmc.exe", Args: []string{"eventvwr.msc"}},
	{ID: "mac-activity-monitor", Name: "macOS 活动监视器", Description: "查看应用进程、CPU、内存、磁盘与网络占用。", Platform: "darwin", Category: "系统诊断", AppPaths: []string{"/System/Applications/Utilities/Activity Monitor.app", "/Applications/Utilities/Activity Monitor.app"}},
	{ID: "mac-disk-utility", Name: "macOS 磁盘工具", Description: "查看磁盘与卷；急救、分区或抹除由你在工具内选择，不自动执行。", Platform: "darwin", Category: "磁盘维护", AppPaths: []string{"/System/Applications/Utilities/Disk Utility.app", "/Applications/Utilities/Disk Utility.app"}},
	{ID: "mac-console", Name: "macOS 控制台", Description: "查看系统和应用日志，辅助定位错误。", Platform: "darwin", Category: "系统诊断", AppPaths: []string{"/System/Applications/Utilities/Console.app", "/Applications/Utilities/Console.app"}},
	{ID: "mac-system-information", Name: "macOS 系统信息", Description: "查看硬件、软件与网络配置详情。", Platform: "darwin", Category: "系统诊断", AppPaths: []string{"/System/Applications/Utilities/System Information.app", "/Applications/Utilities/System Information.app"}},
	{ID: "mac-wireless-diagnostics", Name: "macOS 无线诊断", Description: "打开系统无线网络诊断助手；是否开始诊断由你选择。", Platform: "darwin", Category: "网络应急", AppPaths: []string{"/System/Library/CoreServices/Applications/Wireless Diagnostics.app", "/System/Library/CoreServices/Wireless Diagnostics.app"}},
}

func builtinToolsForPlatform(platform string) []BuiltinTool {
	tools := []BuiltinTool{}
	for _, tool := range builtinTools {
		if tool.Platform == platform {
			tools = append(tools, tool)
		}
	}
	return tools
}

func builtinToolIDs() []string {
	ids := []string{}
	for _, t := range builtinToolsForPlatform(runtime.GOOS) {
		ids = append(ids, t.ID)
	}
	for _, t := range portableTools() {
		ids = append(ids, t.ID)
	}
	return ids
}
func builtinToolName(id string) string {
	for _, t := range portableTools() {
		if t.ID == id {
			return t.Name
		}
	}
	for _, t := range builtinTools {
		if t.ID == id {
			return t.Name
		}
	}
	return id
}
func builtinCommand(t BuiltinTool) (string, []string, bool) {
	return builtinCommandForPlatform(t, runtime.GOOS, nativeSystemDir(), os.Stat)
}

// Command inputs come only from the embedded registry. The stat dependency lets
// platform availability be checked without launching tools or changing a host.
func builtinCommandForPlatform(t BuiltinTool, platform, systemDir string, stat func(string) (os.FileInfo, error)) (string, []string, bool) {
	if t.Platform != platform {
		return "", nil, false
	}
	if platform == "darwin" {
		const program = "/usr/bin/open"
		info, err := stat(program)
		if err != nil || !info.Mode().IsRegular() {
			return "", nil, false
		}
		for _, app := range t.AppPaths {
			info, err := stat(app)
			if err == nil && info.IsDir() {
				// Open this exact system bundle, without passing repair arguments.
				return program, []string{app}, true
			}
		}
		return "", nil, false
	}
	if platform != "windows" {
		return "", nil, false
	}
	dir := systemDir
	if dir == "" {
		return "", nil, false
	}
	program := filepath.Join(dir, t.Program)
	args := append([]string{}, t.Args...)
	for i, a := range args {
		if strings.HasSuffix(a, ".msc") || strings.HasSuffix(a, ".cpl") {
			args[i] = filepath.Join(dir, a)
		}
	}
	for _, path := range append([]string{program}, args...) {
		if !filepath.IsAbs(path) {
			continue
		} // Fixed literal switches/URIs are not file paths.
		info, err := stat(path)
		if err != nil || !info.Mode().IsRegular() {
			return "", nil, false
		}
	}
	return program, args, true
}
func catalogRowsForPlatform(rows []map[string]any, platform string) []map[string]any {
	out := []map[string]any{}
	label := map[string]string{"windows": "Windows", "darwin": "macOS", "linux": "Linux"}[platform]
	for _, row := range rows {
		declared, _ := row["platform"].(string)
		// Legacy rescue resources target PCs; they are not Mac recovery tools.
		if declared == "" && platform != "darwin" || strings.EqualFold(declared, platform) || label != "" && strings.EqualFold(declared, label) {
			out = append(out, row)
		}
	}
	return out
}

func toolCatalog(root string) (any, error) {
	rows := []any{}
	for _, t := range builtinToolsForPlatform(runtime.GOOS) {
		_, _, ready := builtinCommand(t)
		source := "当前 Windows 系统"
		if t.Platform == "darwin" {
			source = "当前 macOS 系统"
		}
		rows = append(rows, map[string]any{"id": t.ID, "name": t.Name, "description": t.Description, "category": t.Category, "ready": ready, "source": source, "platform": t.Platform, "action": "tools.launch"})
	}
	b, err := assets.ReadFile("catalog/maintenance.json")
	if err != nil {
		return nil, err
	}
	var resources []map[string]any
	if err = json.Unmarshal(b, &resources); err != nil {
		return nil, err
	}
	var library []map[string]any
	b, err = assets.ReadFile("catalog/tool-library.json")
	if err != nil {
		return nil, err
	}
	if err = json.Unmarshal(b, &library); err != nil {
		return nil, err
	}
	// A discovered filename does not imply a validated, licensed installer.
	scan, err := scanResources(root)
	if err != nil {
		return nil, err
	}
	counts := map[string]int{}
	for _, f := range scan.Files {
		counts[f.Kind]++
	}
	return map[string]any{"builtin_tools": rows, "portable_tools": portableToolRows(root), "library_tools": catalogRowsForPlatform(library, runtime.GOOS), "resources": catalogRowsForPlatform(resources, runtime.GOOS), "resource_counts": counts, "note": "已内置的便携工具可离线打开；扩展清单提供官方入口，未下载的不会显示为已内置。"}, nil
}
func launchTool(ctx context.Context, id string, confirmed bool) (any, error) {
	if !confirmed {
		return nil, errors.New("请先确认打开页面展示的系统工具")
	}
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	for _, t := range builtinTools {
		if t.ID != id {
			continue
		}
		program, args, ready := builtinCommand(t)
		if !ready {
			return nil, errors.New("此平台未检测到该系统工具；未执行")
		}
		// This is an explicitly requested interactive GUI. It outlives the short
		// launch action; no arbitrary path, command or argument comes from input.
		cmd := exec.Command(program, args...)
		if err := cmd.Start(); err != nil {
			if pid, launchErr := launchElevatedTool(ctx, program, args, err); launchErr == nil {
				return map[string]any{"tool_id": id, "name": t.Name, "pid": pid, "launch_requested": true, "maintenance_performed": false, "note": "系统已接受提权启动请求，后续操作由你在工具中完成。"}, nil
			} else {
				return nil, fmt.Errorf("打开 %s 失败：%w", t.Name, launchErr)
			}
		}
		pid := cmd.Process.Pid
		go cmd.Wait()
		return map[string]any{"tool_id": id, "name": t.Name, "pid": pid, "launch_requested": true, "maintenance_performed": false, "note": "系统已接受启动请求，后续操作由你在工具中完成。"}, nil
	}
	return nil, errors.New("没有该工具的启动适配；未执行")
}

func inspectDrivers(ctx context.Context, root string) (any, error) {
	scan, err := scanResources(root)
	if err != nil {
		return nil, err
	}
	files := []Resource{}
	for _, f := range scan.Files {
		if f.Kind == "drivers" {
			files = append(files, f)
		}
	}
	out := map[string]any{"platform": runtime.GOOS, "devices": []any{}, "offline_files": files, "offline_truncated": scan.Truncated, "scan_warnings": scan.Warnings, "internet_required": false, "driver_install_supported": false,
		"next_steps": []string{"先看设备故障码与硬件 ID，缺驱动不能只按文件名猜。", "可尝试有线网络或手机 USB 共享网络；手机也可能需要驱动。", "在有网电脑从整机或网卡厂商官网下载对应系统/架构的驱动，保留安装包和校验值，放到 drivers/<系统-架构>/<厂商-型号>/。", "驱动安装会改变系统，应确认匹配、备份并保留回滚方式，再到设备管理器操作。"}}
	if runtime.GOOS == "darwin" {
		out["next_steps"] = []string{"检查 Wi-Fi 或有线连接，查看接口是否启用及是否获得地址。", "使用无线诊断和系统信息查看当前 Mac 的网络设备。", "外接网卡软件需匹配设备型号与 macOS 版本；盘内 Windows 驱动包不适用于 Mac。"}
		out["note"] = "此检查提供 macOS 网络接口与地址；不读取 Windows 硬件 ID、INF 或设备故障码，也不会安装驱动。"
		n, networkErr := inspectNetwork()
		if networkErr != nil {
			return nil, networkErr
		}
		out["interfaces"] = n
		return out, nil
	}
	if runtime.GOOS != "windows" {
		out["note"] = "此平台暂提供网卡信息与离线驱动库存，尚未实现硬件 ID 和驱动故障码读取。"
		n, _ := inspectNetwork()
		out["interfaces"] = n
		return out, nil
	}
	// Include unknown devices with code 28: an uninstalled network controller
	// may not yet have class Net. Unknown devices are not labelled as NICs.
	script := `$ErrorActionPreference='Stop';[Console]::OutputEncoding=[Text.UTF8Encoding]::new($false);$drivers=@{};Get-WmiObject Win32_PnPSignedDriver -Filter "DeviceClass='NET'" | ForEach-Object {$drivers[$_.DeviceID]=$_};$rows=@(Get-WmiObject Win32_PnPEntity -Filter "PNPClass='Net' OR ConfigManagerErrorCode=28" | Where-Object {$_.PNPClass -eq 'Net' -or !$_.PNPClass} | Select-Object -First 100 | ForEach-Object {$d=$drivers[$_.DeviceID];[pscustomobject]@{name=$_.Name;device_id=$_.DeviceID;hardware_ids=@($_.HardwareID);compatible_ids=@($_.CompatibleID);class=$_.PNPClass;status=$_.Status;error_code=$_.ConfigManagerErrorCode;manufacturer=$_.Manufacturer;driver_version=$d.DriverVersion;driver_provider=$d.DriverProviderName;inf_name=$d.InfName}});ConvertTo-Json -InputObject $rows -Depth 5 -Compress`
	deadline, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	cmd := exec.CommandContext(deadline, filepath.Join(nativeSystemDir(), "WindowsPowerShell", "v1.0", "powershell.exe"), "-NoProfile", "-NonInteractive", "-Command", script)
	quietProcess(cmd)
	cmd.Env, err = portableEnv(root)
	if err != nil {
		return nil, err
	}
	b, err := cmd.Output()
	if err != nil {
		return nil, errors.New("网络设备与驱动读取未完成；未修改设备或安装驱动")
	}
	var rows []map[string]any
	if err = json.Unmarshal(b, &rows); err != nil {
		return nil, errors.New("设备检测结果格式异常")
	}
	if rows == nil {
		rows = []map[string]any{}
	}
	out["devices"] = rows
	out["note"] = "读取网络类设备及未识别且缺驱动的设备。故障码 0 仅代表设备管理器未报告故障，不证明网络正常；未知类设备不一定是网卡。离线文件尚未自动匹配或安装。"
	return out, nil
}
