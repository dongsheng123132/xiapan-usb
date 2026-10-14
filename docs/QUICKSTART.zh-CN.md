# 虾盘随身 AI 盘 · 快速入门

适用版本：0.5.4。**随身 AI、钱包随盘、程序缓存、断网应急、电脑维护工具包。**

## 复制到 U 盘

把全新发货母盘文件夹里的**全部内容**复制到 U 盘，保持目录结构即可。可以用文件管理器，也可以用批量分发工具的普通文件复制模式；不需要格式化，也不需要制作启动盘。等复制完成后，再打开 U 盘上的启动入口。

每个客户应从干净母盘复制，母盘的 `data/` 应为空。不要把自己已经使用过的 `data/` 发给客户，其中可能有 API Key、钱包、聊天记录和工作文件。需要核对复制完整性时，Windows PowerShell 运行盘根的 `校验母盘.ps1`；Mac 打开 `校验母盘 Mac.command`（或在终端用 `sh` 运行）。校验只读取固定文件，不校验或改动个人 `data/`。

## 开始使用

| 电脑 | 打开 U 盘上的入口 |
| --- | --- |
| Windows x64 | `Start Windows.cmd` 或 `开始使用.cmd` |
| Mac Apple Silicon / Intel | `Start Mac.command` 或 `启动 Mac.command` |

首页使用轻量自建界面，可切换中文或 English。日常直接聊天；复杂任务可打开“完整 PicoClaw”，使用官方工作台。两个入口共用持久 AI 服务。第三方工具与官方工作台的语言取决于各自的支持情况。

Mac 首次可能出现系统信任或文件访问提示。如双击脚本未能运行，可打开“终端”，输入 `sh` 加一个空格，将 `.command` 文件拖入终端，再按回车。脚本会把程序复制到电脑缓存并赋予执行权限；它不会关闭 Gatekeeper。Apple Silicon 已通过实体 ExFAT 盘的启动、文件工具、凭证重启与退出验收；Intel 已构建，首次 Finder 信任和所有系统权限仍须按目标 Mac 验证。

## 选择 AI 服务

**用虾盘云：**打开钱包，按提示绑定或创建本盘钱包；通过充值入口打开付款页面，完成后刷新余额。钱包和凭证保存在本盘，换电脑可以继续使用。

**用自己的 API：**打开“模型设置”，选择“自带 API · OpenAI 兼容”，填写 API 基础地址、API Key、模型 ID，无需开通虾盘云钱包。

| 服务商 | API 基础地址 | 官方说明 |
| --- | --- | --- |
| Kimi | `https://api.moonshot.cn/v1` | [Kimi 接口说明](https://platform.kimi.com/docs/guide/migrating-from-openai-to-kimi) |
| 智谱 | `https://open.bigmodel.cn/api/paas/v4` | [智谱开发文档](https://docs.bigmodel.cn) |
| Gemini | `https://generativelanguage.googleapis.com/v1beta/openai` | [Gemini 兼容接口](https://ai.google.dev/gemini-api/docs/openai) |

模型 ID 按服务商账户中实际可用的模型填写；需要 AI 操作工具时，选择支持工具调用的模型。**保存设置不会调用模型；“测试连接”会发送一次模型请求，可能计费。** 使用自带 API 时，费用由相应服务商账户承担。

Gemini 兼容模式不保证复杂的多轮工具交互；这类任务请在完整 PicoClaw 中配置原生 Gemini。在线 AI 需要目标电脑能连接所选服务，网络可达性随地区与网络环境变化。

## 断网时打开电脑维护工具包

工具入口和本地诊断无需 AI、余额或联网。先查看诊断结果，再选择工具处理；打开工具不会自动完成修复。

- **Windows：9 款随盘便携工具。** WinDirStat、Everything、PeaZip、Notepad++、SumatraPDF、Explorer++、CrystalDiskInfo、PhotoRec、TestDisk，覆盖空间查看、文件查找、解压、文本/PDF 查看、磁盘健康和数据恢复。
- **Windows：9 项系统工具入口。** 内存诊断、网络适配器、网络设置、任务管理器、磁盘清理、设备管理器、系统信息、资源监视器、事件查看器。
- **Mac：5 项系统自带工具入口。** 活动监视器、磁盘工具、控制台、系统信息、无线诊断。它们调用当前 Mac 已有的工具，不是随盘打包的第三方 Mac 软件；系统未提供时会显示不可用。

盘内附有 **3DP Net 21.01（2021 年旧网卡驱动库）**及说明，仅作为 Windows 应急资源，不是通用驱动包，不自动匹配或安装。先确认系统和硬件型号。本盘是操作系统内运行的便携应用，不是 WinPE 或可引导救援盘。

## Mac 网络与启动救援

Mac 的网络入口显示真实接口、启用状态与地址，可打开无线诊断。macOS 自带驱动；外接网卡的软件需匹配型号与 macOS 版本，盘内 Windows 驱动库不用于 Mac。启动项清单目前仅支持 Windows，Mac 首页使用系统信息入口。

无法进入 macOS 时，按 [Apple 恢复说明](https://support.apple.com/zh-cn/102518)进入内建恢复环境：Apple Silicon 关机后按住电源键，进入“选项”；Intel 开机后按住 Command-R。先确认备份和目标磁盘，再按 [Apple 磁盘急救说明](https://support.apple.com/zh-cn/102611)决定操作。本程序不重启进入恢复、不抹盘或重装系统。

## 带走前

首次使用 AI 时，固定程序会复制到当前电脑缓存；后续启动优先使用该缓存，减少 U 盘大文件读取。Key、钱包、会话和工作文件仍保存在 U 盘。首次缓存速度受 U 盘和电脑影响。

内置 PicoClaw 版本冻结，不自动升级；盘内不含本地大模型。请保管好 U 盘，盘内 API Key 是敏感凭证，不应当作已加密的密码保险箱。

关闭已打开的维护工具，在页面点击“退出”，再使用操作系统的安全弹出功能拔盘。只关闭浏览器标签页不会停止后台 AI 服务。
