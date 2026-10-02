# 虾盘 · AI 多功能维护盘

一个独立、轻量的便携电脑维护工具。Go 标准库编写，无 npm 依赖；界面资源内嵌在程序中，本机浏览器显示。

当前版本：0.1.0-preview。

## 使用

Windows：在个人 U 盘试用目录中双击 `虾盘.exe`。进入维护工作台后可以体检、扫描资源、保存报告、安装内置便携体检工具、启动已备齐的本地模型。

程序默认以自身所在目录为便携根目录，数据保存到 `data/`。点右上角电源按钮退出，待系统安全移除后拔盘。

Mac/Linux 的代码提供构建目标，真实启动、浏览器打开、挂载权限及便携体验待对应实机验证；这不是已验证的三系统发行版。

## 已实现

- 原生系统/架构/内存检测；Windows 磁盘与移动盘信息。
- 只读网卡检查，不把网卡启用当成互联网可用。
- 限定资源目录的扫描，不执行扫描发现的文件。
- 内置工具的本盘安装、文件校验与真实版本验证。
- 新文件原子保存、相对目录维护报告、报告迁移读取。
- 六个可复用维护 skills。
- 明确标注的维护向导，以及已连接本地模型的真实问答。
- 固定可信清单校验后启动 CPU 推理程序，退出时停止本次进程。
- 仅本机 HTTP 服务、会话令牌和同源检查。

系统重装、分区/克隆、引导修复、实际文件恢复、通用软件安装、驱动安装和设备钱包是后续适配项。当前试用包不是可引导救援系统。

## 源码与构建

本目录为新产品代码源。现有 U-King 能力通过动作接口逐步接入，公共业务不复制分叉。

首版用本工具的受控动作核心和本地推理引擎完成维护闭环，不依赖 PicoClaw 启动。PicoClaw 等通用 Agent 可作为后续可选适配，不能替代安装与维护动作本身的验证。

```powershell
go test ./...
.\scripts\build.ps1
```

开发机准备可选的 Windows 小模型资源：

```powershell
.\scripts\prepare-local-ai.ps1
```

脚本下载固定来源资源并验证 SHA-256，更新 `catalog/local-ai.json` 后重新构建，使可信清单嵌入程序。模型和程序的大文件不进入 Git；一次下载后复制到发行目录的 `models/` 与 `runtime/`。

`scripts/stage-usb-tests.ps1` 仅用于本次已核验设备，绑定 E/F 磁盘身份并建立新目录；不要把它作为通用制盘器。它不格式化或改分区。

## 机器接口

```powershell
.\虾盘.exe action list --json
.\虾盘.exe action run system.inspect --json --no-input
.\虾盘.exe action run resources.scan --json --no-input
.\虾盘.exe action run report.create --json --no-input
.\虾盘.exe action run tools.install --json --input-file install.json
```

安装参数：`{"package_id":"portable-check","confirmed":true}`。参数走 JSON 文件，不拼接 shell 命令。

stdout 是 JSON 结果（`ok/action/data/error`），诊断日志走 stderr；成功退出码 0，执行失败 1，CLI 格式错误 2。`models.start` 会保持本进程运行以承载模型，终止后结束本次服务；GUI 提供启动、停止和退出入口。

已注册动作见 `action list`。GUI 的动作控件带 `data-action-id`，CLI 和 GUI 使用同一个 `runAction`。模型回答只作为建议，实际修改不由生成的 shell 命令执行。

## 目录

```text
虾盘.exe             离线界面与动作核心
models/              可选本地模型权重
runtime/             匹配平台的本地推理程序
packages/            后续完整软件资源包
images/              后续系统镜像
drivers/             后续匹配驱动
data/reports/        维护报告
data/tools/          本盘安装的体检工具
data/logs/           本次模型启动日志
data/tmp/            临时目录
```

研究与交互决定在 `docs/传统维护盘与交互优化.md`。本次 USB 证据生成到 `evidence/`；这些包含本机路径和设备信息的原始输出默认不提交。

已完成与待验证范围见 `docs/USB实测记录-2026-10-02.md`。许可证与可选资源来源见 LICENSE、THIRD_PARTY.md 和 licenses/。
