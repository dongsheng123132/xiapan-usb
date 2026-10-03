package main

import (
	"context"
	"encoding/json"
	"errors"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"
)

// Unknown approval metadata is never guessed enabled. Services and scheduled
// tasks are separate from the startup locations enumerated here.
const startupScript = `$ErrorActionPreference='Stop'
[Console]::OutputEncoding=[Text.UTF8Encoding]::new($false)
$rows=[Collections.Generic.List[object]]::new();$warnings=[Collections.Generic.List[string]]::new()
function Read-State($path,$name){
 try{$v=Get-ItemPropertyValue -LiteralPath $path -Name $name -ErrorAction Stop;if($v -is [byte[]] -and $v.Length -gt 0){if($v[0] -eq 2){return 'enabled'};if($v[0] -eq 3){return 'disabled'}}}catch{}
 return 'unknown'
}
$locations=@(
 @{path='HKCU:\Software\Microsoft\Windows\CurrentVersion\Run';approved='HKCU:\Software\Microsoft\Windows\CurrentVersion\Explorer\StartupApproved\Run';source='当前用户 · Run'},
 @{path='HKLM:\Software\Microsoft\Windows\CurrentVersion\Run';approved='HKLM:\Software\Microsoft\Windows\CurrentVersion\Explorer\StartupApproved\Run';source='所有用户 · Run'},
 @{path='HKLM:\Software\WOW6432Node\Microsoft\Windows\CurrentVersion\Run';approved='HKLM:\Software\Microsoft\Windows\CurrentVersion\Explorer\StartupApproved\Run32';source='所有用户 · Run32'}
)
foreach($loc in $locations){
 try{if(Test-Path -LiteralPath $loc.path){$key=Get-Item -LiteralPath $loc.path;foreach($name in $key.GetValueNames()){$rows.Add([pscustomobject]@{name=$name;source=$loc.source;state=(Read-State $loc.approved $name)})}}}catch{$warnings.Add($loc.source+' 无法读取')}
}
$folders=@(
 @{path=[Environment]::GetFolderPath('Startup');approved='HKCU:\Software\Microsoft\Windows\CurrentVersion\Explorer\StartupApproved\StartupFolder';source='当前用户 · 启动文件夹'},
 @{path=[Environment]::GetFolderPath('CommonStartup');approved='HKLM:\Software\Microsoft\Windows\CurrentVersion\Explorer\StartupApproved\StartupFolder';source='所有用户 · 启动文件夹'}
)
foreach($loc in $folders){
 try{if($loc.path -and (Test-Path -LiteralPath $loc.path)){foreach($file in (Get-ChildItem -LiteralPath $loc.path -File -Force | Where-Object Name -ne 'desktop.ini')){$rows.Add([pscustomobject]@{name=$file.Name;source=$loc.source;state=(Read-State $loc.approved $file.Name)})}}}catch{$warnings.Add($loc.source+' 无法读取')}
}
ConvertTo-Json -InputObject @{items=@($rows.ToArray());warnings=@($warnings.ToArray());scope='Run / Run32 注册表与当前用户、公共启动文件夹';note='只读检测，未关闭任何启动项。状态未知表示没有可判定的启用记录；不包含全部服务、计划任务及其他自启机制。'} -Depth 4 -Compress`

func inspectStartup(ctx context.Context, root string) (any, error) {
	if runtime.GOOS == "darwin" {
		return inspectLaunchItems(ctx)
	}
	if runtime.GOOS != "windows" {
		return nil, errors.New("此版本的启动项读取支持 Windows 与 macOS")
	}
	deadline, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	cmd := exec.CommandContext(deadline, filepath.Join(nativeSystemDir(), "WindowsPowerShell", "v1.0", "powershell.exe"), "-NoProfile", "-NonInteractive", "-Command", startupScript)
	quietProcess(cmd)
	var err error
	cmd.Env, err = portableEnv(root)
	if err != nil {
		return nil, err
	}
	b, err := cmd.Output()
	if err != nil {
		return nil, errors.New("启动项读取失败或超时，未修改系统")
	}
	var data map[string]any
	if len(b) > 2<<20 || json.Unmarshal(b, &data) != nil {
		return nil, errors.New("启动项返回格式异常")
	}
	return data, nil
}
