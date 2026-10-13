$ErrorActionPreference = 'Stop'
$projectRoot = [IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..'))
# The source baseline and final executable hashes have one authority.
$manifest = Get-Content -LiteralPath (Join-Path $projectRoot 'catalog/picoclaw.json') -Raw | ConvertFrom-Json
if ($manifest.version -ne '0.3.1' -or $manifest.distribution -ne 'u-pico.1') { throw '不支持的发行基线；不得自动升级 PicoClaw' }
$missing = $false
foreach ($slot in $manifest.platforms.PSObject.Properties.Value) {
  foreach ($pair in @(@($slot.path,$slot.sha256), @($slot.launcher_path,$slot.launcher_sha256))) {
    $path = Join-Path $projectRoot $pair[0]
    if (!(Test-Path -LiteralPath $path) -or (Get-FileHash -LiteralPath $path -Algorithm SHA256).Hash.ToLowerInvariant() -ne $pair[1]) { $missing=$true }
  }
}
if ($missing) {
  & python (Join-Path $PSScriptRoot 'build-picoclaw-pinned.py')
  if ($LASTEXITCODE -ne 0) { throw '固定版本构建失败；不能使用未校验的运行时' }
}
Write-Output 'PicoClaw v0.3.1 / u-pico.1 核心和官方界面已按清单准备；发行版自更新已禁用。'
