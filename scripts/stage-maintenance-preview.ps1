param([ValidateSet('E')][string]$Drive='E')
$ErrorActionPreference='Stop'
$projectRoot=[IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..'))
$partition=Get-Partition -DriveLetter $Drive
$disk=Get-Disk -Number $partition.DiskNumber
$volume=Get-Volume -DriveLetter $Drive
if($disk.BusType -ne 'USB' -or $disk.IsBoot -or $disk.IsSystem -or $disk.FriendlyName -ne 'ORICO EB05-128GB' -or $volume.FileSystem -ne 'exFAT'){throw '目标不是已核验的 ORICO exFAT 测试盘'}
if($volume.SizeRemaining -lt 1GB){throw '空间不足'}
$driveRoot="$($Drive):\"
$target=[IO.Path]::GetFullPath((Join-Path $driveRoot ('虾盘 AI 维护台 0.2-'+[guid]::NewGuid().ToString('N').Substring(0,6))))
if(!$target.StartsWith($driveRoot,[StringComparison]::OrdinalIgnoreCase) -or (Test-Path -LiteralPath $target)){throw '目标目录边界检查失败'}
$before=@(Get-ChildItem -LiteralPath $driveRoot -Force | Select-Object Name,Length,LastWriteTimeUtc)
New-Item -ItemType Directory -Path $target | Out-Null
Copy-Item -LiteralPath (Join-Path $projectRoot 'dist/windows-x64/虾盘.exe') -Destination (Join-Path $target '虾盘.exe')
foreach($folder in @('runtime','models','licenses','skills','docs','catalog')){Copy-Item -LiteralPath (Join-Path $projectRoot $folder) -Destination (Join-Path $target $folder) -Recurse}
foreach($folder in @('packages','images','drivers')){New-Item -ItemType Directory -Path (Join-Path $target $folder) | Out-Null}
foreach($file in @('README.md','LICENSE','THIRD_PARTY.md')){Copy-Item -LiteralPath (Join-Path $projectRoot $file) -Destination (Join-Path $target $file)}
Copy-Item -LiteralPath (Join-Path $projectRoot 'launchers/终端维护菜单.ps1') -Destination (Join-Path $target '终端维护菜单.ps1')
Copy-Item -LiteralPath (Join-Path $projectRoot 'launchers/终端维护菜单.cmd') -Destination (Join-Path $target '终端维护菜单.cmd')
$after=@(Get-ChildItem -LiteralPath $driveRoot -Force | Where-Object FullName -ne $target | Select-Object Name,Length,LastWriteTimeUtc)
if(($before|ConvertTo-Json -Compress) -ne ($after|ConvertTo-Json -Compress)){throw '原有盘根条目变化，请检查'}
$state=@{root=$target;drive=$Drive;device=$disk.FriendlyName;disk=$disk.Number;filesystem=$volume.FileSystem;original_root_entries_unchanged=$true}
New-Item -ItemType Directory -Path (Join-Path $projectRoot 'evidence') -Force | Out-Null
$state|ConvertTo-Json|Set-Content -LiteralPath (Join-Path $projectRoot 'evidence/preview-target.json') -Encoding utf8NoBOM
$state|ConvertTo-Json
