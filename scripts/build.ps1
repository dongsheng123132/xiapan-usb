$ErrorActionPreference='Stop'
$projectRoot=[IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..'))
$env:GOCACHE=Join-Path $projectRoot '.cache\go-build'
$env:GOMODCACHE=Join-Path $projectRoot '.cache\go-mod'
$originalGOOS=$env:GOOS;$originalGOARCH=$env:GOARCH;$originalCGO=$env:CGO_ENABLED
try{
  Push-Location -LiteralPath $projectRoot
  $env:CGO_ENABLED='0'
  & go test ./...
  if($LASTEXITCODE -ne 0){throw '核心测试失败'}
  foreach($target in @(@{os='windows';arch='amd64';dir='windows-x64';name='虾盘.exe'},@{os='darwin';arch='arm64';dir='macos-arm64';name='xiapan'},@{os='darwin';arch='amd64';dir='macos-x64';name='xiapan'},@{os='linux';arch='amd64';dir='linux-x64';name='xiapan'})){
    $env:GOOS=$target.os;$env:GOARCH=$target.arch
    $targetDir=Join-Path $projectRoot ('dist\'+$target.dir)
    New-Item -ItemType Directory -Path $targetDir -Force | Out-Null
    $ldflags='-s -w';if($target.os -eq 'windows'){$ldflags+=' -H windowsgui'}
    & go build -buildvcs=false -trimpath "-ldflags=$ldflags" -o (Join-Path $targetDir $target.name) .
    if($LASTEXITCODE -ne 0){throw "构建失败：$($target.dir)"}
    $binary=Get-Item -LiteralPath (Join-Path $targetDir $target.name)
    Write-Output "$($target.dir): $($binary.Length) bytes"
  }
}finally{Pop-Location;$env:GOOS=$originalGOOS;$env:GOARCH=$originalGOARCH;$env:CGO_ENABLED=$originalCGO}
