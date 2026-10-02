$ErrorActionPreference='Stop'
$projectRoot=[IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..'))
$suffix=[guid]::NewGuid().ToString('N').Substring(0,6)
$expected=@{E='ORICO EB05-128GB';F='USB Sandisk 3.2Gen1'}
$rows=@()
foreach($letter in @('E','F')){
  $partition=Get-Partition -DriveLetter $letter
  $disk=Get-Disk -Number $partition.DiskNumber
  if($disk.BusType -ne 'USB' -or $disk.IsBoot -or $disk.IsSystem -or $disk.FriendlyName -ne $expected[$letter]){throw "磁盘身份与已核验目标不符：$letter"}
  $volume=Get-Volume -DriveLetter $letter
  if($volume.SizeRemaining -lt 1GB){throw "测试盘空间不足：$letter"}
  $driveRoot="$($letter):\"
  $target=[IO.Path]::GetFullPath((Join-Path $driveRoot "虾盘 AI 维护盘 - 试用 20261002-$suffix"))
  if(!$target.StartsWith($driveRoot,[StringComparison]::OrdinalIgnoreCase) -or (Test-Path -LiteralPath $target)){throw '测试目录边界或新目录检查失败'}
  $before=@(Get-ChildItem -LiteralPath $driveRoot -Force | Select-Object Name,Length,LastWriteTimeUtc)
  New-Item -ItemType Directory -Path $target | Out-Null
  Copy-Item -LiteralPath (Join-Path $projectRoot 'dist\windows-x64\虾盘.exe') -Destination (Join-Path $target '虾盘.exe')
  foreach($folder in @('models','runtime')){Copy-Item -LiteralPath (Join-Path $projectRoot $folder) -Destination (Join-Path $target $folder) -Recurse}
  foreach($file in @('README.md','LICENSE','THIRD_PARTY.md')){Copy-Item -LiteralPath (Join-Path $projectRoot $file) -Destination (Join-Path $target $file)}
  foreach($folder in @('licenses','skills')){Copy-Item -LiteralPath (Join-Path $projectRoot $folder) -Destination (Join-Path $target $folder) -Recurse}
  @('双击「虾盘.exe」打开维护工作台。','本次是独立试用目录，原来的 U-Claw、AI 精灵和其他文件保留。','离线 AI 页面可启动本盘模型。退出时点击界面右上角的电源按钮，再安全移除 U 盘。','此版本没有格式化、分区、重装系统或实际数据恢复功能。') | Set-Content -LiteralPath (Join-Path $target '先看这里.txt') -Encoding utf8
  $after=@(Get-ChildItem -LiteralPath $driveRoot -Force | Where-Object {$_.FullName -ne $target} | Select-Object Name,Length,LastWriteTimeUtc)
  if(($before | ConvertTo-Json -Depth 3 -Compress) -ne ($after | ConvertTo-Json -Depth 3 -Compress)){throw "盘根原有条目发生变化，请检查：$letter"}
  $rows+=@{drive=$letter;disk_number=$disk.Number;device=$disk.FriendlyName;file_system=$volume.FileSystem;root=$target;original_root_entries_unchanged=$true}
}
New-Item -ItemType Directory -Path (Join-Path $projectRoot 'evidence') -Force | Out-Null
$rows | ConvertTo-Json -Depth 4 | Set-Content -LiteralPath (Join-Path $projectRoot 'evidence\usb-targets.json') -Encoding utf8NoBOM
$rows | ConvertTo-Json -Depth 4
