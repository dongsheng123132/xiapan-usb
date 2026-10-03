# Xiapan USB Toolkit · 虾盘 U 盘精灵

**Portable PC maintenance tools, with optional AI.**

Carry system diagnostics, portable utilities and your AI settings on a USB drive. Use the maintenance tools without AI, or connect your own OpenAI-compatible API. Windows x64 is the tested platform; full macOS and Linux support is under development.

开源、便携、可接 AI 的电脑维护工具箱。自己的维护核心，按需接入第三方绿色工具和 AI。

**0.4.2 Preview：Windows x64 已实机验证；macOS / Linux 正在适配。当前仓库提供源码，尚未发布面向所有平台的完整下载包。**

## 能做什么

- 搜索、分类筛选 35 项工具目录：6 款已适配便携工具、6 个 Windows 系统工具、19 项选装软件和 4 项救援/驱动资源。
- 读取系统信息、进程、常见启动项和网络设备；保存维护报告。当前不支持自动禁用启动项或自动修复所有故障。
- 不使用 AI 也能检测和打开工具。AI 可选 PicoClaw Agent、自带 OpenAI 兼容 API、虾盘云，或另行准备的本地模型。
- 设置、API Key、对话和报告随盘保存；GUI、CLI 和 MCP 共用同一套 Go 动作核心。
- AI 只能申请受支持的维护动作。任意 Shell、文件修改和子代理工具默认禁用，确认在动作核心校验。

六款已适配工具：WinDirStat、PeaZip、Everything、Explorer++、Notepad++、SumatraPDF。**源码仓库不带这些程序**；按官方固定版本下载并校验后，才能生成含工具的便携包。选装目录不等于已内置。

## 在 Windows、Mac 或 Linux 开发

三种系统使用**同一个仓库**。准备 Git 和 Go 1.24 或更新版本：

```sh
git clone https://github.com/dongsheng123132/xiapan-usb.git
cd xiapan-usb
go test ./...
go build -o dist/xiapan .
```

Windows 可以改成 `go build -o dist/xiapan.exe .`。运行只读体检：

```sh
./dist/xiapan action run system.inspect --json --no-input
```

开发机上的数据不要作为发货模板。详细流程、分支协作和平台验收见 [开发指南](docs/DEVELOPMENT.md)。

| 平台 | 当前状态 |
| --- | --- |
| Windows x64 | 本机两块 U 盘基础流程、其中一块完整六工具包已验证；不是所有电脑兼容性保证 |
| macOS Apple Silicon / Intel | 主程序可交叉构建；Agent、离线模型运行时及维护工具需要原生适配和实机验证 |
| Linux x64 | 主程序构建及 WSL 核心动作已验证；真实 Linux 便携盘、桌面集成与完整工具包待验证 |

同一只盘可保存多平台程序和共享模型，但各平台运行各自的可执行文件。此项目目前**不是可启动救援系统**，不能替代已经无法启动的操作系统。

## 生成 Windows 便携包

需要 Go、PowerShell 和 Python 3；最终用户运行便携包不需要 Go/Python/Node。

```powershell
.\scripts\prepare-picoclaw.ps1
.\scripts\build.ps1
python scripts/package-genie.py
```

添加六款工具：先运行 `python scripts/prepare-portable-tools.py`，重新构建，再运行 `python scripts/package-genie.py --with-tools`。下载使用固定版本及 SHA-256；准备脚本拒绝覆盖已有工具目录。离线模型另见 `scripts/prepare-local-ai.ps1`，不包含在默认精简包里。

```text
虾盘.exe    启动入口
app/        引擎、可选工具、许可证和校验清单
data/       用户首次运行后生成的设置、对话和报告
```

API Key 和历史备份目前以**明文**存放在 `data/`，不绑定电脑；请保管整只盘。换电脑仍需要服务可达、Key 有效；本机 localhost 服务需要重新启动。更新时保留 `data/` 和工具目录内的个人配置。

## 开源与实体盘

本项目自有源码采用 [MIT](LICENSE)。你可以免费下载、自行制作 U 盘，也可以购买预装好、测试过的实体盘。实体盘的价值来自硬件、下载与配置时间、兼容性验证和支持服务。

第三方工具和模型保留各自许可证，不能统一改称 MIT；见 [第三方说明](THIRD_PARTY.md)。实体盘与离线模型交付建议见 [商业交付说明](docs/DISTRIBUTION.md)。云端额度是可选服务，不是本地维护功能的解锁条件。

- [虾盘云](https://cloud.u-claw.org/) · 可选模型服务
- [U-King](https://www.u-king.org/) · AI 装机工具
- [虾盘云画板](https://tu.u-claw.org.cn/) · 网页创作工具

欢迎通过 Issue / Pull Request 提交平台适配和工具清单；见 [贡献指南](CONTRIBUTING.md)。请勿上传钱包、API Key、私人诊断数据或用户文件。
