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
	Program     string   `json:"-"`
	Args        []string `json:"-"`
}

var builtinTools = []BuiltinTool{
	{"task-manager", "Windows 任务管理器", "查看启动应用、进程与性能。", "Taskmgr.exe", nil},
	{"disk-cleanup", "Windows 磁盘清理", "由你选择清理项与目标磁盘；不自动删除文件。", "cleanmgr.exe", nil},
	{"device-manager", "Windows 设备管理器", "查看设备状态，手动选择已匹配的驱动；不自动装驱动。", "mmc.exe", []string{"devmgmt.msc"}},
	{"system-info", "Windows 系统信息", "查看系统和硬件详情，工具由 Windows 提供。", "msinfo32.exe", nil},
	{"resource-monitor", "Windows 资源监视器", "查看 CPU、磁盘、内存和网络占用。", "resmon.exe", nil},
	{"event-viewer", "Windows 事件查看器", "查看系统与应用错误记录。", "mmc.exe", []string{"eventvwr.msc"}},
}

func builtinToolIDs() []string {
	ids := []string{}
	for _, t := range nativeTools() {
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
	for _, t := range nativeTools() {
		if t.ID == id {
			return t.Name
		}
	}
	return id
}
func builtinCommand(t BuiltinTool) (string, []string, bool) {
	if runtime.GOOS == "darwin" {
		// LaunchServices opens the fixed Apple bundle or settings pane.
		info, err := os.Stat(t.Program)
		if err != nil || !info.IsDir() {
			return "", nil, false
		}
		if len(t.Args) > 0 {
			return "/usr/bin/open", append([]string{}, t.Args...), true
		}
		return "/usr/bin/open", []string{t.Program}, true
	}
	if runtime.GOOS != "windows" {
		return "", nil, false
	}
	dir := nativeSystemDir()
	if dir == "" {
		return "", nil, false
	}
	program := filepath.Join(dir, t.Program)
	args := append([]string{}, t.Args...)
	for i, a := range args {
		if strings.HasSuffix(a, ".msc") {
			args[i] = filepath.Join(dir, a)
		}
	}
	for _, path := range append([]string{program}, args...) {
		info, err := os.Stat(path)
		if err != nil || !info.Mode().IsRegular() {
			return "", nil, false
		}
	}
	return program, args, true
}
func toolCatalog(root string) (any, error) {
	rows := []any{}
	for _, t := range nativeTools() {
		_, _, ready := builtinCommand(t)
		rows = append(rows, map[string]any{"id": t.ID, "name": t.Name, "description": t.Description, "ready": ready, "source": "当前 " + osLabel(runtime.GOOS) + " 系统", "platform": runtime.GOOS, "action": "tools.launch"})
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
	return map[string]any{"builtin_tools": rows, "portable_tools": portableToolRows(root), "library_tools": library, "resources": resources, "resource_counts": counts, "note": "已内置的便携工具可离线打开；扩展清单提供官方入口，未下载的不会显示为已内置。"}, nil
}
func launchTool(ctx context.Context, id string, confirmed bool) (any, error) {
	if !confirmed {
		return nil, errors.New("请先确认打开页面展示的系统工具")
	}
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	for _, t := range nativeTools() {
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
		out["devices"] = macNetworkDevices(ctx)
		out["next_steps"] = []string{"先确认网络端口是否出现、是否已连接；Wi-Fi 问题可打开无线诊断。", "Mac 的网卡驱动随系统提供，一般不需要离线驱动包；第三方 USB 网卡按厂商说明安装并在系统设置中允许。", "可尝试有线网络或手机 USB 共享网络。", "系统扩展或驱动的安装会改变系统，应确认来源并保留卸载方式。"}
		out["note"] = "读取 macOS 网络端口与网卡地址；macOS 不提供 Windows 式的驱动故障码。离线文件仅为库存，不自动安装。"
		n, _ := inspectNetwork()
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
