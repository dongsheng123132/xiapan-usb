$ErrorActionPreference='Stop'
$projectRoot=[IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..'))
$originalGOOS=$env:GOOS;$originalGOARCH=$env:GOARCH;$originalCGO=$env:CGO_ENABLED
try {
  Push-Location -LiteralPath $projectRoot
  $env:GOOS='windows';$env:GOARCH='amd64';$env:CGO_ENABLED='0'
  & go test ./...
  if($LASTEXITCODE -ne 0){throw '核心测试失败'}
  $targetDir=Join-Path $projectRoot 'dist/windows-x64'
  New-Item -ItemType Directory -Path $targetDir -Force | Out-Null
  & go build -trimpath '-ldflags=-s -w -H windowsgui' -o (Join-Path $targetDir '虾盘.exe') .
  if($LASTEXITCODE -ne 0){throw 'Windows 构建失败'}
  Get-Item -LiteralPath (Join-Path $targetDir '虾盘.exe') | Select-Object Name,Length
} finally { Pop-Location;$env:GOOS=$originalGOOS;$env:GOARCH=$originalGOARCH;$env:CGO_ENABLED=$originalCGO }
