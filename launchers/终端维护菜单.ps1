$ErrorActionPreference='Stop'
$binary=Join-Path $PSScriptRoot '虾盘.exe'
if(!(Test-Path -LiteralPath $binary)){throw '请从完整的维护盘目录启动'}
do {
  Clear-Host
  Write-Host '  XIAPAN / AI MAINTENANCE KIT' -ForegroundColor Green
  Write-Host "  本盘：$PSScriptRoot"
  Write-Host '  1 系统体检    2 进程内存    3 网络诊断'
  Write-Host '  4 盘内资源    5 本地模型    6 保存报告'
  Write-Host '  7 查看报告    8 打开 AI 对话界面    0 退出'
  $choice=Read-Host '选择动作'
  if($choice -eq '0'){break}
  if($choice -eq '8'){Start-Process -FilePath $binary -WindowStyle Hidden;break}
  $action=@{'1'='system.inspect';'2'='processes.inspect';'3'='network.diagnose';'4'='resources.scan';'5'='models.inspect';'6'='report.create';'7'='reports.list'}[$choice]
  if(!$action){continue}
  if($choice -eq '6' -and (Read-Host '保存新的报告到本盘？输入 y 确认') -ne 'y'){continue}
  try {
    $raw=& $binary action run $action --json --no-input | Out-String
    $result=$raw|ConvertFrom-Json
    if(!$result.ok){Write-Host $result.error -ForegroundColor Yellow}else{$result.data|ConvertTo-Json -Depth 12|Write-Host}
  }catch{Write-Host $_.Exception.Message -ForegroundColor Yellow}
  Read-Host '回车返回菜单'|Out-Null
}while($true)
