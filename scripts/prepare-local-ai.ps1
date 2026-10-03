param([string]$ProxyUrl='http://127.0.0.1:7897')
$ErrorActionPreference='Stop'
$projectRoot=[IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..'))
$downloadDir=Join-Path $projectRoot '.cache\downloads'
New-Item -ItemType Directory -Path $downloadDir -Force | Out-Null
function Get-VerifiedFile([string]$Url,[string]$Path,[string]$ExpectedHash){
  if(Test-Path -LiteralPath $Path){if((Get-FileHash -LiteralPath $Path -Algorithm SHA256).Hash.ToLowerInvariant() -eq $ExpectedHash){Write-Output "已验证，使用本地文件：$([IO.Path]::GetFileName($Path))";return};throw '已有文件校验不符，保留原文件并停止。'}
  $partPath="$Path.part"
  $curlArgs=@('--fail','--location','--connect-timeout','15','--max-time','600','--retry','2','--continue-at','-','--output',$partPath)
  if($ProxyUrl){$curlArgs+=@('--proxy',$ProxyUrl)}
  $curlArgs+=$Url
  & curl.exe @curlArgs
  if($LASTEXITCODE -ne 0){throw "下载未完成，保留断点文件：$([IO.Path]::GetFileName($partPath))"}
  if((Get-FileHash -LiteralPath $partPath -Algorithm SHA256).Hash.ToLowerInvariant() -ne $ExpectedHash){throw '下载文件校验失败，未将其作为可执行资源。'}
  Move-Item -LiteralPath $partPath -Destination $Path
}
$engineZip=Join-Path $downloadDir 'llama-b11146-bin-win-cpu-x64.zip'
Get-VerifiedFile 'https://github.com/ggml-org/llama.cpp/releases/download/b11146/llama-b11146-bin-win-cpu-x64.zip' $engineZip '14cf1303ca9ac3abd94816850532f9f9a69ac66fbaca3776fc6f9061c2fac1d1'
$unpackPath=Join-Path $downloadDir ('unpack-'+[guid]::NewGuid().ToString('N'))
Expand-Archive -LiteralPath $engineZip -DestinationPath $unpackPath
$server=Get-ChildItem -LiteralPath $unpackPath -Recurse -File -Filter 'llama-server.exe' | Select-Object -First 1
if(!$server){throw '官方资源包中没有找到 llama-server.exe'}
$runtimeDir=Join-Path $projectRoot 'runtime\windows-x64'
New-Item -ItemType Directory -Path $runtimeDir -Force | Out-Null
$engineFiles=@()
foreach($file in (Get-ChildItem -LiteralPath $server.DirectoryName -File | Where-Object {$_.Name -eq 'llama-server.exe' -or $_.Extension -eq '.dll'})){
  $destination=Join-Path $runtimeDir $file.Name
  $hash=(Get-FileHash -LiteralPath $file.FullName -Algorithm SHA256).Hash.ToLowerInvariant()
  if(Test-Path -LiteralPath $destination){if((Get-FileHash -LiteralPath $destination -Algorithm SHA256).Hash.ToLowerInvariant() -ne $hash){throw "运行时已有不同文件，保留并停止：$($file.Name)"}}else{Copy-Item -LiteralPath $file.FullName -Destination $destination}
  $engineFiles+=@{path=('runtime/windows-x64/'+$file.Name);sha256=$hash}
}
$modelDir=Join-Path $projectRoot 'models'
New-Item -ItemType Directory -Path $modelDir -Force | Out-Null
$modelPath=Join-Path $modelDir 'Qwen3.5-0.8B-Q4_0.gguf'
Get-VerifiedFile 'https://huggingface.co/ggml-org/Qwen3.5-0.8B-GGUF/resolve/8fea620/Qwen3.5-0.8B-Q4_0.gguf?download=true' $modelPath '57d1997790d1744fba5b40a7317df71ea5e2acee28c47e78f0cce39c0703f8cf'
$manifestPath=Join-Path $projectRoot 'catalog\local-ai.json'
$manifest=Get-Content -LiteralPath $manifestPath -Raw | ConvertFrom-Json
$manifest.ready=$true
$manifest.engine_files=$engineFiles
$temporaryManifest="$manifestPath.tmp"
[IO.File]::WriteAllText($temporaryManifest, ($manifest | ConvertTo-Json -Depth 6) + [Environment]::NewLine, [Text.UTF8Encoding]::new($false))
Move-Item -LiteralPath $temporaryManifest -Destination $manifestPath -Force
Write-Output '本地模型资源已准备并校验。请重新构建，以将可信运行时清单嵌入程序。'
