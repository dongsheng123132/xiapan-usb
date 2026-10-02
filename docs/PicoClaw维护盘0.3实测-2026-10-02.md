# 虾盘 PicoClaw 维护盘 0.3 实测

定位：PicoClaw U 盘便携版，内置虾盘云设备钱包；维护能力来自可插拔的工具和 skills。默认 PicoClaw，虾盘云直连为备用，离线小模型为断网备选。当前原生 PicoClaw 资源只备齐 Windows x64。

## 已验证

| 流程 | 可观察证据 |
| --- | --- |
| E 盘运行 | ORICO EB05-128GB、exFAT；新版从原试用目录运行，Windows 核心 8,137,216 字节 |
| 原生模型选择维护动作 | 实际 PicoClaw v0.3.1 → 本地模型接口回放 → MCP → system.inspect / tools.catalog → 真实结果回传 → 原生回答；提问不含旧诊断关键词 |
| 确认边界 | 原生模型提出 report.create 与 tools.launch，只有待确认卡片；报告数量在确认前为 0；模型额外 confirmed 参数和钱包操作不能越权 |
| 离线网络驱动读取 | 这台 Windows 读取 8 个网络设备，均有硬件 ID；包含 Intel Wi-Fi、Realtek 有线设备及虚拟接口；本轮修复后约 5.41 秒 |
| 原生工具入口 | 本机检测到 cleanmgr、设备管理器、系统信息；浏览器申请系统信息、点击确认，验证新启动 msinfo32 进程与“系统信息”窗口，再正常关闭该测试窗口 |
| 断网入口 | 本地网络、驱动、工具集合按钮；离线模型与维护向导选项，无需先获得云端回答 |
| 持续记录 | 对话、实际结果、申请状态和确认记录保存在本盘；旧版记录保留 |

设备读取改用系统 Windows PowerShell 5.1 的只读 WMI 查询。决定性测试：CIM 的 PnP 查询约 15.55 秒，WMI 同一筛选约 2.27 秒；完整驱动检测从约 19 秒降到约 5.41 秒。另测宿主临时目录和 U 盘临时目录，均约 18 秒，排除了 exFAT 临时目录是主要原因。保留异步过程与超时，不改网卡、驱动、注册表或安全设置。

## 尚未完成

本轮查询 E 盘钱包仍只有 1 个校验额度，未付款、未管理员加额度、未挪用其他钱包。真实有额度的 PicoClaw 云端问答仍待充值后验收。本地接口回放不等于真实云端调用成功。

盘内 drivers/ 当前为空。显示硬件信息不等于已经备齐驱动，驱动文件名不等于兼容性验证。尚未执行驱动安装、网络重置、自动清理、系统重装或启动救援。Ventoy、SystemRescue、Hiren’s BootCD PE 是官方资源入口，未把镜像或工具发行包直接打入盘内。

Windows 真盘与 WSL 核心测试不能证明 macOS 原生运行、第二台电脑接续、物理断网、旧电脑性能或 BIOS/UEFI 启动。macOS/Linux 核心有构建产物，原生 PicoClaw 的对应资源与实机验收应分开补齐。

## 接入更多维护工具

按工具集合接入，复用现成能力。每个资源先记录官方来源、许可证、平台/架构、版本、SHA-256、空间需求和成功判据；再注册动作适配。GUI、CLI 和 PicoClaw 共用这个动作，skill 只负责何时调用、怎样辨认影响和怎样验证。

优先增加“识别网卡 → 按硬件 ID 准备官方驱动 → 本盘包验证 → 用户确认 → 安装后复查”这一完整链路，再增加常用软件安装与更多传统工具。

参考：[PicoClaw v0.3.1 MCP 配置源码](https://github.com/sipeed/picoclaw/blob/v0.3.1/pkg/config/config.go)、[Windows WMI 查询](https://learn.microsoft.com/en-us/powershell/module/microsoft.powershell.management/get-wmiobject?view=powershell-5.1)、[Ventoy](https://www.ventoy.net/cn/)、[SystemRescue](https://www.system-rescue.org/)、[Hiren’s BootCD PE](https://www.hirensbootcd.org/)。

证据文件位于工作区忽略目录 evidence/：preview-03-results.json、pico-adapter-results.json、system-tool-ui-03.json，以及 0.3 工具集合/网络驱动/断网急救截图。运行数据与钱包不进仓库。
