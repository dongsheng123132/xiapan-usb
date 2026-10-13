<p align="center">
  <img src="banner.svg" alt="U-PicoClaw — portable AI with offline maintenance tools" width="100%">
</p>

<p align="center"><a href="README.md">English</a> · <a href="README.zh-CN.md">简体中文</a> · <a href="README.ja.md">日本語</a></p>

# 虾盘 · U-PicoClaw 随身 AI 优盘

**轻量启动、钱包与 Key 随盘、本机程序缓存，再带一套断网应急维护工具。**

![Preview](https://img.shields.io/badge/version-0.5.1--portable--preview-8b6b33)
![Go](https://img.shields.io/badge/Go-1.24%2B-367a8d)
![License](https://img.shields.io/badge/license-MIT-527c46)
![AI optional](https://img.shields.io/badge/AI-optional-4c7770)

[快速开始](#快速开始) · [平台支持](#平台支持) · [制作便携包](#制作多平台便携包) · [参与贡献](CONTRIBUTING.md)

> [!NOTE]
> **当前为预览版。** 发行配方包含 Windows x64、Mac Apple Silicon 和 Mac Intel 三个平台的程序。新版 Mac 包尚未完成实机验收，本轮远程测试机不可达；Windows 工具不算 Mac 工具。产品范围以 [产品设计](docs/PRODUCT.md) 为准。

## 把 AI 和应急工具带在身边

双击进入原有的轻量自建界面，需要长任务、技能、渠道或完整配置时，再点 **完整 PicoClaw**。两个入口使用同一个受管理的持久 Gateway。关闭网页不会停止 Gateway；完成后点 **退出**，再安全移除 U 盘。

| 特性 | 说明 |
| --- | --- |
| 钱包与 API Key 随盘 | 使用虾盘云或自带兼容 API，设置与对话随盘带走。仅打开首页不会自动创建钱包。 |
| 按需充值 | 钱包内查看余额并打开充值页，支付由用户完成。 |
| 本机程序缓存 | 已校验的 PicoClaw 程序可缓存在当前电脑，减少重复读取 U 盘；Key、钱包、对话与工作文件继续随盘保存。 |
| 断网应急 | 无需模型、API Key 或钱包额度，直接检测电脑并打开兼容工具。 |
| 中英双语 | 整个界面可切换中英文，选择随盘保存，AI 按所选语言回答。 |

首次填充缓存仍需读取 U 盘。首屏可用、Gateway 就绪与首次 AI 回答分别计时，**没有把“0.3 秒启动”作为已经验证的承诺**。成品体积取决于平台程序和工具资源。本产品不再携带本地模型权重与推理运行时，也不加入 OpenClaw。

## 界面语言

顶栏的语言选择器可在 **简体中文** 与 **English** 之间切换所有程序自带文案，选择保存在盘内 `data/settings/ui.json`，换台电脑依然生效，同时决定 AI 用什么语言回答。

对话内容、文件路径、真实 JSON 证据、以及你自己起的会话标题都不翻译，只翻译界面本身。语言资源加载失败时界面保持中文，不会因此打不开。

## 左侧栏目

左侧导航默认显示，可通过顶部按钮收起为窄条图标栏，收起状态会记住。窗口宽度小于 780px 时，左侧栏改为浮层抽屉，由「对话记录」按钮打开，避免小屏被侧栏挤占。

## 快速开始

准备 **Git 与 Go 1.24+**。Windows、Mac、Linux 都从同一个仓库开发：

```sh
git clone --branch codex/u-picoclaw-portable https://github.com/dongsheng123132/xiapan-usb.git
cd xiapan-usb
go test ./...
go build -o dist/xiapan .
./dist/xiapan action run system.inspect --json --no-input
```

Windows 改为 `go build -o dist/xiapan.exe .`，再运行 `./dist/xiapan.exe`。这样构建的是动作核心，不会自动带上 PicoClaw 和第三方程序。平台适配与分支协作见 [开发指南](docs/DEVELOPMENT.md)。

## 工具箱里有什么

Windows 发行配方包含 **9 款便携工具和 9 项系统工具入口**，另有明确标注的选装下载资源。目录按当前平台显示真实可用状态；目录中有名字，不代表已经安装。

| 已适配工具 | 用途 |
| --- | --- |
| WinDirStat | 查看磁盘空间被谁占用 |
| PeaZip | 压缩、解压文件 |
| Everything | 快速搜索文件 |
| Explorer++ | 浏览和管理文件 |
| Notepad++ | 编辑文本和配置 |
| SumatraPDF | 阅读 PDF 文档 |
| CrystalDiskInfo | 读取磁盘健康与 SMART 信息 |
| PhotoRec | 将找回的文件输出到其他健康介质 |
| TestDisk | 查看分区与恢复选项 |

工具的版本、文件、SHA-256 与许可证记录在固定清单中，制盘只复用资源源目录中的已核验文件。驱动资源和硬件识别单独提供；自动匹配、安装与回滚尚未实现。系统工具入口包括任务管理器、设备管理器等 Windows 自带程序。

## 一套 PicoClaw，两个界面入口

| 入口 | 用途 |
| --- | --- |
| 自建轻量首页 | 快速对话、模型设置、钱包充值与维护入口。 |
| 完整 PicoClaw | 按需启动官方工作台，完成复杂交互与详细配置。 |
| 维护向导 | 不用 AI、不用云端额度，执行确定性的检测和工具启动。 |

核心与官方 launcher 一起冻结在 **PicoClaw v0.3.1**，发行补丁禁用自升级，清单同时锁定两者 SHA-256。客户盘不跟随上游 latest/nightly 自动更新。

PicoClaw 工具功能按当前系统用户权限运行。维护动作由界面、CLI、MCP 复用同一动作核心；维护动作的确认规则不等于限制了 PicoClaw 的任意 shell 命令。

![界面、CLI 与 AI 共用维护核心](diagrams/action-core.svg)

## 平台支持

| 平台 | 实际验证范围 |
| --- | --- |
| **Windows x64** | 历史维护流程已有实物 U 盘测试；0.5 Gateway/缓存成品以本轮发行验收记录为准，旧测试不代替新链路验证。 |
| macOS · Apple Silicon / Intel | 两个平台的程序均进入制盘配方。首次信任、权限、实体 U 盘与跨电脑使用尚未实机验收，本轮远程 Mac 测试机不可达。 |
| Linux x64 | 保留核心源码与构建目标，本次成品配方不包含 Linux。历史 WSL 体检/报告测试不等于 Linux 桌面发行验收。 |

一只盘可以保存多平台程序，但各系统运行自己的可执行文件。系统无法启动时，目前不能靠本工具代替启动救援环境。启动项检查覆盖常见 Run 项与启动文件夹，不包含所有服务/计划任务，也不自动关闭启动项。

## 制作多平台便携包

维护者需要 Go、PowerShell、Python 3.11+；重建固定版官方工作台另外需要 Node.js 与 pnpm。最终用户无需这些构建工具。

```powershell
python scripts/build-picoclaw-pinned.py
.\scripts\build.ps1
python scripts/package-portable.py --resources "D:\已验证工具资源" --output "D:\发盘\U-PicoClaw-0.5.1-portable-preview"
```

资源目录需有清单要求的工具文件，输出必须是尚不存在的新目录。脚本校验资源、创建空白 data 并生成成品校验清单，不复制来源盘的个人数据。新母盘只使用这条配方，历史 package-genie 不再作为本产品制盘入口。准备要求与验收见 [开发指南](docs/DEVELOPMENT.md)。

```text
虾盘.exe   Windows 启动入口
app/       各平台程序、固定 PicoClaw、工具与许可证
data/      个人设置、钱包、对话与工作内容
drivers/   提前准备的驱动资源
```

> [!IMPORTANT]
> API Key 及历史备份目前以**明文随盘保存**，请保管好整只盘。换电脑仍需服务可达、凭证有效；localhost 模型服务要在当前电脑启动。客户盘不自升级；将来经过验证的新发行版仍需保留 `data/` 和各工具个人配置。发货模板不能复制已经使用过的数据目录。

## 开源，也可以购买预装盘

自有源码采用 [MIT](LICENSE)。你可以免费下载、自行制盘，也可以在有成品供应时选择预装实体盘。实体盘提供硬件、资源预下载、配置验证和支持服务。云端额度可选，本地维护功能不以充值解锁。

第三方工具保留各自许可证，不能统一改称 MIT。再分发前请阅读 [第三方说明](THIRD_PARTY.md) 和 [商业交付说明](docs/DISTRIBUTION.md)。

<details>
<summary>项目主题配图</summary>

![虾盘 U 盘精灵概念插画](assets/xiapan-usb-concept.png)

这是项目概念插画，不代表某款正在发货的硬件外观。[设计与工具说明](docs/README-DESIGN.md)。

</details>

## 一起完善

欢迎贡献平台适配、可复现的错误报告和绿色工具清单。从 [贡献指南](CONTRIBUTING.md)、[安全说明](SECURITY.md) 或 [Issue](https://github.com/dongsheng123132/xiapan-usb/issues) 开始。不要上传钱包链接、API Key 或私人诊断数据。

[虾盘云](https://cloud.u-claw.org/) · [U-King](https://www.u-king.org/) · [虾盘云画板](https://tu.u-claw.org.cn/)
