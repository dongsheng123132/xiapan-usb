param([switch]$FingerprintOnly)

$ErrorActionPreference = 'Stop'
$projectRoot = [IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..'))
$fingerprintAlgorithm = 'sha256-ordinal-path-nul-file-sha256-lf-v1'

function Get-SourceEvidence {
  # Keep this input set and canonical byte format in sync with package-portable.py.
  # Hash actual build inputs, including uncommitted work; never enumerate data/.
  $relativePaths = [Collections.Generic.List[string]]::new()
  foreach ($file in Get-ChildItem -LiteralPath $projectRoot -File -Filter '*.go') {
    $relativePaths.Add($file.Name)
  }
  foreach ($name in @('go.mod', 'go.sum', 'scripts/build.ps1')) {
    if (Test-Path -LiteralPath (Join-Path $projectRoot $name) -PathType Leaf) {
      $relativePaths.Add($name)
    }
  }
  foreach ($file in Get-ChildItem -LiteralPath (Join-Path $projectRoot 'web') -Recurse -File) {
    $relativePaths.Add([IO.Path]::GetRelativePath($projectRoot, $file.FullName).Replace('\', '/'))
  }
  foreach ($file in Get-ChildItem -LiteralPath (Join-Path $projectRoot 'catalog') -File -Filter '*.json') {
    $relativePaths.Add('catalog/' + $file.Name)
  }
  foreach ($dir in Get-ChildItem -LiteralPath (Join-Path $projectRoot 'skills') -Directory) {
    if (Test-Path -LiteralPath (Join-Path $dir.FullName 'SKILL.md') -PathType Leaf) {
      $relativePaths.Add('skills/' + $dir.Name + '/SKILL.md')
    }
  }
  $sorted = $relativePaths.ToArray()
  [Array]::Sort($sorted, [StringComparer]::Ordinal)
  $records = [Collections.Generic.List[object]]::new()
  $canonical = [Text.StringBuilder]::new()
  foreach ($relative in $sorted) {
    $current = $projectRoot
    foreach ($component in $relative.Split('/')) {
      $current = Join-Path $current $component
      $item = Get-Item -LiteralPath $current -Force
      if (($item.Attributes -band [IO.FileAttributes]::ReparsePoint) -ne 0) {
        throw "Source input may not be a link: $relative"
      }
    }
    $hash = (Get-FileHash -LiteralPath $current -Algorithm SHA256).Hash.ToLowerInvariant()
    $records.Add([ordered]@{path=$relative; sha256=$hash})
    [void]$canonical.Append($relative).Append([char]0).Append($hash).Append([char]10)
  }
  $sha = [Security.Cryptography.SHA256]::Create()
  try {
    $bytes = [Text.Encoding]::UTF8.GetBytes($canonical.ToString())
    $fingerprint = ([BitConverter]::ToString($sha.ComputeHash($bytes))).Replace('-', '').ToLowerInvariant()
  } finally { $sha.Dispose() }
  return [ordered]@{fingerprint_algorithm=$fingerprintAlgorithm; source_fingerprint=$fingerprint; source_files=$records.ToArray()}
}

if ($FingerprintOnly) {
  Get-SourceEvidence | ConvertTo-Json -Depth 6
  exit 0
}

$originalEnvironment = @{}
foreach ($name in @('GOCACHE', 'GOMODCACHE', 'GOOS', 'GOARCH', 'CGO_ENABLED')) {
  $originalEnvironment[$name] = [Environment]::GetEnvironmentVariable($name, 'Process')
}
$pushed = $false
try {
  $env:GOCACHE = Join-Path $projectRoot '.cache/go-build'
  $env:GOMODCACHE = Join-Path $projectRoot '.cache/go-mod'
  Push-Location -LiteralPath $projectRoot
  $pushed = $true
  $source = Get-SourceEvidence
  $versionMatch = [regex]::Match((Get-Content -LiteralPath 'main.go' -Raw), '(?m)^const version = "([^"\r\n]+)"')
  if (!$versionMatch.Success) { throw 'Cannot read application version' }
  $buildVersion = $versionMatch.Groups[1].Value
  $sourceCommit = (& git rev-parse HEAD).Trim()
  if ($LASTEXITCODE -ne 0) { throw 'Cannot read source commit' }
  $goVersion = (& go version).Trim()
  if ($LASTEXITCODE -ne 0) { throw 'Cannot locate Go compiler' }
  # Run tests for the build host even when called from a cross-compile shell.
  Remove-Item Env:GOOS, Env:GOARCH -ErrorAction SilentlyContinue
  $env:CGO_ENABLED = '0'
  & go test ./...
  if ($LASTEXITCODE -ne 0) { throw '核心测试失败' }
  $platforms = [ordered]@{}
  $targets = @(
    @{os='windows'; arch='amd64'; dir='windows-x64'; name='虾盘.exe'},
    @{os='darwin'; arch='arm64'; dir='macos-arm64'; name='xiapan'},
    @{os='darwin'; arch='amd64'; dir='macos-x64'; name='xiapan'},
    @{os='linux'; arch='amd64'; dir='linux-x64'; name='xiapan'}
  )
  foreach ($target in $targets) {
    $env:GOOS = $target.os
    $env:GOARCH = $target.arch
    $targetDir = Join-Path $projectRoot ('dist/' + $target.dir)
    New-Item -ItemType Directory -Path $targetDir -Force | Out-Null
    $ldflags = '-s -w'
    if ($target.os -eq 'windows') { $ldflags += ' -H windowsgui' }
    $destination = Join-Path $targetDir $target.name
    # Publish complete files, with the build manifest only after every platform.
    $temporary = $destination + '.building-' + [Guid]::NewGuid().ToString('N')
    try {
      & go build -buildvcs=false -trimpath "-ldflags=$ldflags" -o $temporary .
      if ($LASTEXITCODE -ne 0) { throw "构建失败：$($target.dir)" }
      Move-Item -LiteralPath $temporary -Destination $destination -Force
    } finally {
      if (Test-Path -LiteralPath $temporary) { Remove-Item -LiteralPath $temporary -Force }
    }
    $binary = Get-Item -LiteralPath $destination
    $platforms[$target.os + '/' + $target.arch] = [ordered]@{
      path=('dist/' + $target.dir + '/' + $target.name)
      sha256=(Get-FileHash -LiteralPath $destination -Algorithm SHA256).Hash.ToLowerInvariant()
      bytes=$binary.Length
      version=$buildVersion
    }
    Write-Output "$($target.dir): $($binary.Length) bytes"
  }
  $finalSource = Get-SourceEvidence
  if ($finalSource.source_fingerprint -ne $source.source_fingerprint) {
    throw 'Source changed while building; rebuild after edits finish. No new build manifest was published.'
  }
  $finalCommit = (& git rev-parse HEAD).Trim()
  if ($LASTEXITCODE -ne 0 -or $finalCommit -ne $sourceCommit) {
    throw 'Source commit changed while building; rebuild before packaging.'
  }
  $manifest = [ordered]@{
    schema=1
    version=$buildVersion
    built_at=[DateTime]::UtcNow.ToString('o')
    source_commit=$sourceCommit
    fingerprint_algorithm=$source.fingerprint_algorithm
    source_fingerprint=$source.source_fingerprint
    source_files=$source.source_files
    go_version=$goVersion
    platforms=$platforms
  }
  $manifestPath = Join-Path $projectRoot 'dist/build-manifest.json'
  $manifestTemp = $manifestPath + '.tmp-' + [Guid]::NewGuid().ToString('N')
  try {
    [IO.File]::WriteAllText($manifestTemp, ($manifest | ConvertTo-Json -Depth 8) + "`n", [Text.UTF8Encoding]::new($false))
    Move-Item -LiteralPath $manifestTemp -Destination $manifestPath -Force
  } finally {
    if (Test-Path -LiteralPath $manifestTemp) { Remove-Item -LiteralPath $manifestTemp -Force }
  }
  Write-Output "Build manifest: $manifestPath"
} finally {
  if ($pushed) { Pop-Location }
  foreach ($name in $originalEnvironment.Keys) {
    [Environment]::SetEnvironmentVariable($name, $originalEnvironment[$name], 'Process')
  }
}
