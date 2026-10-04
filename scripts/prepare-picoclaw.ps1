$ErrorActionPreference='Stop'
$projectRoot=[IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..'))
$archive=Join-Path $projectRoot '.cache/picoclaw-0.3.1.zip'
New-Item -ItemType Directory -Path (Split-Path -Parent $archive) -Force|Out-Null
if(!(Test-Path -LiteralPath $archive)){
  & curl.exe -L --fail --connect-timeout 15 --max-time 180 -o $archive 'https://github.com/sipeed/picoclaw/releases/download/v0.3.1/picoclaw_Windows_x86_64.zip'
  if($LASTEXITCODE -ne 0){throw '下载未完成；请重试'}
}
if((Get-FileHash -LiteralPath $archive -Algorithm SHA256).Hash.ToLowerInvariant() -ne 'c76e4b9f3f95137b8cb58229e756538b12ed0fee0e4301a01b531626bf740af1'){throw '归档 SHA-256 不匹配'}
$dest=Join-Path $projectRoot 'runtime/picoclaw/windows-x64'
New-Item -ItemType Directory -Path $dest -Force|Out-Null
Expand-Archive -LiteralPath $archive -DestinationPath $dest -Force
if((Get-FileHash -LiteralPath (Join-Path $dest 'picoclaw.exe') -Algorithm SHA256).Hash.ToLowerInvariant() -ne 'f939c001c729e6e326a99f154f376816ac12a79cb70968f882b3ee287f45119e'){throw '程序 SHA-256 不匹配'}
Copy-Item -LiteralPath (Join-Path $dest 'LICENSE') -Destination (Join-Path $projectRoot 'licenses/PicoClaw-MIT.txt')
Write-Output 'PicoClaw v0.3.1 已准备并校验；AI 问答按需配置公司批准的 API。'
