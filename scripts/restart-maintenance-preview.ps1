$ErrorActionPreference='Stop'
$projectRoot=[IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..'))
$target=Get-Content -LiteralPath (Join-Path $projectRoot 'evidence/preview-target.json') -Raw|ConvertFrom-Json
$root=[IO.Path]::GetFullPath($target.root)
$partition=Get-Partition -DriveLetter E
$disk=Get-Disk -Number $partition.DiskNumber
if($disk.FriendlyName -ne 'ORICO EB05-128GB' -or $disk.BusType -ne 'USB' -or $disk.IsBoot -or $disk.IsSystem -or (Get-Volume -DriveLetter E).FileSystem -ne 'exFAT' -or !$root.StartsWith('E:\虾盘 AI 维护台 0.2-',[StringComparison]::OrdinalIgnoreCase)){throw '测试盘与目录核验失败'}
& python (Join-Path $PSScriptRoot 'stop-owned-preview.py')
$serverRecord=Join-Path $projectRoot 'evidence/preview-server.json'
if(Test-Path -LiteralPath $serverRecord){$old=Get-Content -LiteralPath $serverRecord -Raw|ConvertFrom-Json;for($i=0;$i -lt 30;$i++){if(!(Get-Process -Id $old.pid -ErrorAction SilentlyContinue)){break};Start-Sleep -Milliseconds 100}}
$binary=Join-Path $root '虾盘.exe'
$pending=Join-Path $root ('虾盘.new-'+[guid]::NewGuid().ToString('N')+'.exe')
Copy-Item -LiteralPath (Join-Path $projectRoot 'dist/windows-x64/虾盘.exe') -Destination $pending
$backup=Join-Path $root ('data/backups/app-'+[guid]::NewGuid().ToString('N')+'.exe')
New-Item -ItemType Directory -Path (Split-Path -Parent $backup) -Force|Out-Null
Move-Item -LiteralPath $binary -Destination $backup
try{Move-Item -LiteralPath $pending -Destination $binary}catch{Move-Item -LiteralPath $backup -Destination $binary;throw}
foreach($file in @('README.md','LICENSE','THIRD_PARTY.md')){Copy-Item -LiteralPath (Join-Path $projectRoot $file) -Destination (Join-Path $root $file) -Force}
foreach($file in @('终端维护菜单.cmd','终端维护菜单.ps1')){Copy-Item -LiteralPath (Join-Path $projectRoot ('launchers/'+$file)) -Destination (Join-Path $root $file) -Force}
foreach($folder in @('macos-arm64','macos-x64','linux-x64')){Copy-Item -LiteralPath (Join-Path $projectRoot ('dist/'+$folder)) -Destination (Join-Path $root $folder) -Recurse -Force}
foreach($folder in @('docs','catalog')){Copy-Item -LiteralPath (Join-Path $projectRoot $folder) -Destination (Join-Path $root $folder) -Recurse -Force}
$process=Start-Process -FilePath $binary -ArgumentList @('serve','--port','18797','--no-open') -WindowStyle Hidden -PassThru -RedirectStandardOutput (Join-Path $projectRoot 'evidence/preview-out.log') -RedirectStandardError (Join-Path $projectRoot 'evidence/preview-error.log')
@{pid=$process.Id;root=$root;url='http://127.0.0.1:18797'}|ConvertTo-Json|Set-Content -LiteralPath $serverRecord -Encoding utf8NoBOM
Write-Output "新版已从 $root 启动：http://127.0.0.1:18797"
