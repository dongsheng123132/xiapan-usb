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
	// The settings page is opened through the same shell handler that opens links;
	// the page, like every argument here, is fixed in this table.
	{"network-settings", "Windows 网络设置", "打开网络状态页；代理、网络重置、网卡属性由你在页面中自己选择，虾盘不替你修改。", "rundll32.exe", []string{"url.dll,FileProtocolHandler", "ms-settings:network-status"}},
}

func builtinToolIDs() []string {
	ids := []string{}
	for _, t := range builtinTools {
		ids = append(ids, t.ID)
	}
	return ids
}
func builtinToolName(id string) string {
	for _, t := range builtinTools {
		if t.ID == id {
			return t.Name
		}
	}
	return id
}
func builtinCommand(t BuiltinTool) (string, []string, bool) {
	if runtime.GOOS != "windows" {
		return "", nil, false
	}
	dir := nativeSystemDir()
	if dir == "" {
		return "", nil, false
	}
	program := filepath.Join(dir, t.Program)
	args := append([]string{}, t.Args...)
	files := []string{program}
	for i, a := range args {
		if strings.HasSuffix(a, ".msc") {
			args[i] = filepath.Join(dir, a)
			files = append(files, args[i])
		}
	}
	for _, path := range files {
		info, err := os.Stat(path)
		if err != nil || !info.Mode().IsRegular() {
			return "", nil, false
		}
	}
	return program, args, true
}
func toolCatalog(root string) (any, error) {
	rows := []any{}
	for _, t := range builtinTools {
		_, _, ready := builtinCommand(t)
		rows = append(rows, map[string]any{"id": t.ID, "name": t.Name, "description": t.Description, "ready": ready, "source": "当前 Windows 系统", "platform": runtime.GOOS, "action": "tools.launch"})
	}
	return map[string]any{"builtin_tools": rows, "note": "仅打开 Windows 自带工具；后续操作由网管在工具中完成。"}, nil
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
	var err error
	out := map[string]any{"platform": runtime.GOOS, "devices": []any{}, "internet_required": false, "driver_install_supported": false,
		"next_steps": []string{"根据故障码与硬件 ID 核对公司批准的驱动；不自动下载或安装。"}}
	if runtime.GOOS != "windows" {
		return nil, errors.New("仅支持 Windows 网络设备读取")
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
