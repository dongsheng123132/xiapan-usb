<p align="center">
  <img src="banner.svg" alt="Xiapan USB Toolkit — portable PC maintenance with optional AI" width="100%">
</p>

<p align="center"><a href="README.md">English</a> · <a href="README.zh-CN.md">简体中文</a> · <a href="README.ja.md">日本語</a></p>

# 虾盘 · U 盘精灵

**开源、便携、可接 AI 的电脑维护工具箱。**

![Preview](https://img.shields.io/badge/version-0.4.2%20preview-8b6b33)
![Go](https://img.shields.io/badge/Go-1.24%2B-367a8d)
![License](https://img.shields.io/badge/license-MIT-527c46)
![AI optional](https://img.shields.io/badge/AI-optional-4c7770)

[快速开始](#快速开始) · [平台支持](#平台支持) · [制作便携包](#制作-windows-便携包) · [参与贡献](CONTRIBUTING.md)

> [!NOTE]
> **当前为预览版。** Windows x64 已在实物 U 盘测试，macOS 预览版（`虾盘.app`）已在 Apple 芯片实测，Linux 完整包仍在适配。这个仓库提供源码，不包含全部第三方程序或模型权重；目前也不是可启动救援系统。

## 把维护工具带在身边

检查电脑、找到合适的工具，把设置和维护报告留在自己的盘里。可以直接使用本地工具，也可以接入 AI 帮忙理解检测结果。

| 特性 | 说明 |
|------|------|
| 检查电脑 | 读取系统信息、进程内存、常见启动项与网络设备。 |
| 找到工具 | 可搜索分类的工具清单，明确显示已准备和待下载状态。 |
| 随盘保存 | 模型设置、对话和维护报告保存在自己的 U 盘。 |
| 自选 AI | 自带兼容 API、使用虾盘云，或另行准备可选离线模型。 |

## 快速开始

准备 **Git 与 Go 1.24+**。Windows、Mac、Linux 都从同一个仓库开发：

```sh
git clone https://github.com/dongsheng123132/xiapan-usb.git
cd xiapan-usb
go test ./...
go build -o dist/xiapan .
./dist/xiapan action run system.inspect --json --no-input
```

Windows 改为 `go build -o dist/xiapan.exe .`，再运行 `./dist/xiapan.exe`。这样构建的是动作核心，不会自动带上 PicoClaw、离线模型和第三方程序。平台适配与分支协作见 [开发指南](docs/DEVELOPMENT.md)。

## 工具箱里有什么

**35 项目录**：6 款已适配便携工具、6 项 Windows 系统工具、19 项选装软件、4 项救援/驱动资源。支持按名称和用途搜索，并按类别、准备状态筛选。

| 已适配工具 | 用途 |
| --- | --- |
| WinDirStat | 查看磁盘空间被谁占用 |
| PeaZip | 压缩、解压文件 |
| Everything | 快速搜索文件 |
| Explorer++ | 浏览和管理文件 |
| Notepad++ | 编辑文本和配置 |
| SumatraPDF | 阅读 PDF 文档 |

选装工具包从官方源下载、校验后打包这六款程序。**列入目录不等于已经安装。** 部分项目因许可或适配验收尚未完成，仅提供官方下载入口。Windows 系统工具包括任务管理器、资源监视器、事件查看器、磁盘清理、设备管理器和系统信息；macOS 为活动监视器、登录项、存储空间、磁盘工具、系统信息、控制台和无线诊断。

## AI 按需使用

| 模式 | 使用方式 |
| --- | --- |
| 不使用 AI | 本地检测与工具启动不需要云钱包。 |
| 联网 AI | 可选 PicoClaw Agent，自带 OpenAI 兼容地址、模型和 Key，或使用虾盘云。 |
| 离线 AI | 单独准备已适配的 Windows CPU 运行时与 Qwen3.5-0.8B 资源包，默认精简包不含模型。 |

界面、CLI、AI 共用同一套 Go 动作核心。PicoClaw 适配器禁用任意命令、文件修改、技能安装和子代理；模型传入“已确认”不能绕过用户确认。

![界面、CLI 与可选 AI 共用维护核心](diagrams/action-core.svg)

## 平台支持

| 平台 | 实际验证范围 |
| --- | --- |
| **Windows x64** | 两块实物 U 盘通过基础流程，其中一块通过六款工具包验证；使用同一台 Windows 电脑，不代表所有电脑均已兼容。 |
| macOS · Apple Silicon | `虾盘.app` 通用包；体检、启动项、进程、网络、系统工具、PicoClaw 与 Metal 离线模型已在 M4 / macOS 15 实测；第三方便携工具尚未适配，见 [macOS 版](docs/MACOS.md)。 |
| macOS · Intel | 程序与打包已支持 x64；x64 运行时需准备并实机验证。 |
| Linux x64 | 核心构建及 WSL 体检、报告流程通过；真实 Linux 桌面与便携盘仍需验证。 |

一只盘可以保存多平台程序，但各系统运行自己的可执行文件。系统无法启动时，目前不能靠本工具代替启动救援环境。启动项检查覆盖常见 Run 项与启动文件夹，不包含所有服务/计划任务，也不自动关闭启动项。

## 制作 Windows 便携包

构建端需要 Go、PowerShell、Python 3。最终用户运行便携包不需要 Go、Python 或 Node。

```powershell
.\scripts\prepare-picoclaw.ps1
.\scripts\build.ps1
python scripts/package-genie.py
```

<details>
<summary>加入绿色工具或准备离线模型</summary>

先运行 `python scripts/prepare-portable-tools.py`，重新构建，再运行 `python scripts/package-genie.py --with-tools`。使用固定版本及 SHA-256，准备脚本拒绝覆盖已经存在的工具目录。

离线模型资源另见 `scripts/prepare-local-ai.ps1`。默认精简包不带模型权重、驱动和救援镜像。

</details>

```text
虾盘.exe   启动入口
app/       引擎、可选工具、许可证和文件校验清单
data/      个人设置、对话和报告
```

## 制作 macOS 便携包

在 Mac 上需要 Go、Python 3 和 Xcode 命令行工具：

```sh
python3 scripts/prepare-macos-runtime.py --arch arm64   # 可选：PicoClaw 与离线 AI 运行时，加 --with-model 下载模型
./scripts/build.sh
python3 scripts/package-macos.py                         # 加 --with-local-ai 附带离线 AI
```

双击 `虾盘.app` 即在浏览器中打开。默认包为 ad-hoc 签名，适合自己直接写入 U 盘；供下载分发需要 Developer ID 签名与公证，详见 [macOS 版](docs/MACOS.md)。

> [!IMPORTANT]
> API Key 及历史备份目前以**明文随盘保存**，请保管好整只盘。换电脑仍需服务可达、凭证有效；localhost 模型服务要在当前电脑启动。更新时保留 `data/` 和各工具的个人配置。发货模板不能复制已经使用过的数据目录。

## 开源，也可以购买预装盘

自有源码采用 [MIT](LICENSE)。你可以免费下载、自行制盘，也可以在有成品供应时选择预装实体盘。实体盘提供硬件、资源预下载、配置验证和支持服务。云端额度可选，本地维护功能不以充值解锁。

第三方工具与模型保留各自许可证，不能统一改称 MIT。再分发前请阅读 [第三方说明](THIRD_PARTY.md) 和 [商业交付说明](docs/DISTRIBUTION.md)。

<details>
<summary>项目主题配图</summary>

![虾盘 U 盘精灵概念插画](assets/xiapan-usb-concept.png)

这是项目概念插画，不代表某款正在发货的硬件外观。[设计与工具说明](docs/README-DESIGN.md)。

</details>

## 一起完善

欢迎贡献平台适配、可复现的错误报告和绿色工具清单。从 [贡献指南](CONTRIBUTING.md)、[安全说明](SECURITY.md) 或 [Issue](https://github.com/dongsheng123132/xiapan-usb/issues) 开始。不要上传钱包链接、API Key 或私人诊断数据。

[虾盘云](https://cloud.u-claw.org/) · [U-King](https://www.u-king.org/) · [虾盘云画板](https://tu.u-claw.org.cn/)
