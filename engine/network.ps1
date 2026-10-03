# Vendored from Open365 (Apache-2.0) commit ab6d25d125a0bdcdbbff1632cf87bf5b5f6b11d9.
# Xiapan modifications: prefer a gateway-bearing adapter; retain read-only probes only.
# Repair commands, administrator escalation and speculative repair advice are removed.
param(
    [Parameter(Position=0)][ValidateSet('diagnose')][string]$Action='diagnose',
    [switch]$Json
)
$ErrorActionPreference='Stop'
try { [Console]::OutputEncoding=[Text.UTF8Encoding]::new($false) } catch { }




# ---------- 诊断（只读，不改任何设置） ----------

# hosts 文件只读体检：数「自定义映射」条数，挑出对主流网站的指向（流氓软件常见劫持面：
# 屏蔽 Windows 更新、劫持首页/激活）。只读、绝不改 hosts；路径作参数便于自测。
function Get-HostsFindings([string]$path) {
    $r = [ordered]@{ entries = 0; suspicious = $false; hits = @() }
    if (-not (Test-Path -LiteralPath $path)) { return $r }
    $major = @('microsoft', 'windowsupdate', 'apple', 'icloud', 'google', 'baidu',
               'qq.com', 'tencent', 'taobao', 'alipay', 'weixin')
    foreach ($line in (Get-Content -LiteralPath $path -ErrorAction SilentlyContinue)) {
        $t = "$line".Trim()
        if (-not $t -or $t.StartsWith('#')) { continue }
        $parts = $t -split '\s+'
        if ($parts.Count -lt 2) { continue }
        $r.entries++
        $ip = $parts[0]
        $hn = ($parts[1..($parts.Count - 1)] -join ' ').ToLower()
        foreach ($m in $major) {
            if ($hn.Contains($m)) { $r.suspicious = $true; $r.hits += "$ip -> $hn"; break }
        }
    }
    return $r
}

function Invoke-Diagnose {
    $result = [ordered]@{
        timestamp   = (Get-Date).ToString('s')
        adapters    = @()
        gateway     = $null
        dns_servers = @()
        proxy       = $null
        tests       = [ordered]@{}
        verdict     = $null      # 给出"病因"判断
    }

    # 1) 活动网卡 + IP + 网关
    $adapters = Get-NetIPConfiguration | Where-Object { $_.IPv4Address -and $_.NetAdapter.Status -eq 'Up' }
    foreach ($a in $adapters) {
        $result.adapters += [ordered]@{
            name    = $a.InterfaceAlias
            ipv4    = ($a.IPv4Address.IPAddress -join ', ')
            gateway = $a.IPv4DefaultGateway.NextHop
            dns     = ($a.DNSServer | Where-Object { $_.AddressFamily -eq 2 } | ForEach-Object { $_.ServerAddresses } ) -join ', '
        }
    }
    # Xiapan patch: prefer the adapter that has a default gateway (VPN/virtual adapters such as Tailscale often come first).
    $primary = $adapters | Where-Object { $_.IPv4DefaultGateway } | Select-Object -First 1
    if (-not $primary) { $primary = $adapters | Select-Object -First 1 }
    $gw = $primary.IPv4DefaultGateway.NextHop
    $result.gateway = $gw
    $result.dns_servers = @($primary.DNSServer | Where-Object { $_.AddressFamily -eq 2 } | ForEach-Object { $_.ServerAddresses })

    # 2) 系统代理状态（流氓软件常在这里下黑手）
    $reg = 'HKCU:\Software\Microsoft\Windows\CurrentVersion\Internet Settings'
    $proxyEnable = (Get-ItemProperty -Path $reg -Name ProxyEnable -ErrorAction SilentlyContinue).ProxyEnable
    $proxyServer = (Get-ItemProperty -Path $reg -Name ProxyServer -ErrorAction SilentlyContinue).ProxyServer
    $result.proxy = [ordered]@{
        enabled = [bool]$proxyEnable
        server  = $proxyServer
    }

    # 3) 三级连通性测试：网关 -> 公共IP -> 域名解析
    function Test-Ping([string]$target) {
        try { return [bool](Test-Connection -ComputerName $target -Count 2 -Quiet -ErrorAction Stop) }
        catch { return $false }
    }
    $t_gateway = if ($gw) { Test-Ping $gw } else { $false }

    # 真实"能否打开网页"测试：用 HTTP 抓微软官方网络探测端点。
    # 比 ping 可靠得多 —— 很多环境/代理拦 ICMP，ping 不通≠没网。
    #
    # 关键：测两次 —— 一次直连(绕过代理)、一次走系统代理。
    # 两者对比才能区分"底层链路坏"和"代理坏"：
    #   直连通 + 走代理不通  => 代理坏了（流氓软件偷设/梯子挂了）
    #   直连不通             => 底层(Winsock/TCPIP/DNS)坏了
    function Test-Web([string]$url, [bool]$useSystemProxy) {
        try {
            $req = [System.Net.HttpWebRequest]::Create($url)
            if ($useSystemProxy) {
                # 用系统默认代理（即注册表里那个）
                $req.Proxy = [System.Net.WebRequest]::GetSystemWebProxy()
            } else {
                $req.Proxy = $null        # 绕过代理，测裸连通
            }
            $req.Timeout = 4000
            $req.Method = 'GET'
            $resp = $req.GetResponse()
            $code = [int]$resp.StatusCode
            $resp.Close()
            return ($code -ge 200 -and $code -lt 400)
        } catch { return $false }
    }
    # DNS 解析单独测（即使 HTTP 失败也能区分是不是 DNS 问题）
    $t_dns = $false
    try {
        $rr = Resolve-DnsName -Name 'www.msftconnecttest.com' -Type A -ErrorAction Stop -QuickTimeout
        $t_dns = [bool]($rr | Where-Object { $_.IPAddress })
    } catch { $t_dns = $false }

    # IP 层：直接连一个公网 IP 的 80 端口（绕过 DNS）
    $t_internet = $false
    try {
        $tcp = New-Object System.Net.Sockets.TcpClient
        $iar = $tcp.BeginConnect('114.114.114.114', 53, $null, $null)
        $t_internet = $iar.AsyncWaitHandle.WaitOne(3000) -and $tcp.Connected
        $tcp.Close()
    } catch { $t_internet = $false }

    # 整条链路测两次：直连 vs 走系统代理
    $url = 'http://www.msftconnecttest.com/connecttest.txt'
    $t_web_direct = Test-Web $url $false           # 绕过代理，测底层链路
    # 只有系统代理开着时才需要测"走代理"；没开代理时两者等价
    if ($result.proxy.enabled -and $result.proxy.server) {
        $t_web_proxy = Test-Web $url $true         # 走系统代理
    } else {
        $t_web_proxy = $t_web_direct
    }
    # "用户实际能否上网" = 系统当前生效路径的结果
    $t_web = if ($result.proxy.enabled -and $result.proxy.server) { $t_web_proxy } else { $t_web_direct }

    $result.tests.gateway_reachable  = $t_gateway
    $result.tests.internet_reachable = $t_internet      # IP 层(TCP)通不通
    $result.tests.dns_works          = $t_dns            # DNS 解析行不行
    $result.tests.web_direct         = $t_web_direct     # 直连能否打开网页
    $result.tests.web_via_proxy      = $t_web_proxy      # 走系统代理能否打开
    $result.tests.web_works          = $t_web            # 用户当前实际能否上网

    $result.verdict = '公网探测结果需结合公司代理、DNS、VPN 与准入策略复核。'
    $result.hosts = Get-HostsFindings "$env:WINDIR\System32\drivers\etc\hosts"

    return $result
}


$result = Invoke-Diagnose
$result | ConvertTo-Json -Depth 6 -Compress
