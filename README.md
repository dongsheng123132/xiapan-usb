# 虾盘 · AI 多功能维护盘

0.2.0-preview。一个轻量、开源的便携 AI 维护台：默认虾盘云 Agent，免注册设备钱包与一键打开充值页；本地小模型作为断网备选。本机浏览器呈现持续对话，Go 标准库动作核心负责真实检测和受控执行。

## 使用

Windows：双击 U 盘目录里的 `虾盘.exe`。输入问题继续对话，或使用 `/体检`、`/进程`、`/网络`、`/资源`、`/模型`、`/安装`、`/报告`、`/救援`。只读检测可直接执行；对话中的安装和保存报告要点击对应确认卡片。

默认云端问答需要钱包额度。第一次联网自动创建钱包，也可在左下角钱包面板填入已有凭证。凭证就是钱包，请自行备份；充值按钮在系统浏览器打开官方页面，由用户选择并付款。查询失败显示未知，不显示假零余额。钱包轮换支持中断续接，移除本盘引用不会删除服务端余额。用户凭证和运行数据不进源码或发行模板。

PicoClaw 是可选的原生 Agent，接同一套钱包、便携工作区和六个维护技能。安装等宿主维护操作仍由维护核心处理；本轮禁用了 PicoClaw 的任意命令、文件写入、技能安装和子代理工具。默认云端 Agent 则使用直接受控工具调用，能够在持续对话里读取实际证据并提出下一步。

断网时可直接用本地工具或维护向导。已带齐 Windows 模型资源的试用包还可选择「本盘离线 AI」，首次提问时启动；小模型说明需要结合实际检测，不能凭它的回答认定电脑故障。界面退出会停止本程序启动的模型；之后按系统方式安全移除 U 盘。

想用终端菜单：双击 `终端维护菜单.cmd`。专业模式展示真实 JSON；Ctrl K 打开动作菜单。两种入口调用同一动作核心。

## 当前能力和边界

| 能力 | 本轮状态 |
| --- | --- |
| 系统体检 | 系统、架构、逻辑处理器、总内存、可用内存；Windows 磁盘空间 |
| 进程检查 | 读取工作集内存，按占用排序；不结束进程、不读取命令行 |
| 网络诊断 | 网卡、虾盘云 DNS 和 HTTPS；可达不等于已获得模型调用权限 |
| 盘内资源 | 扫描 models、packages、images、drivers、runtime，不执行发现的未知文件 |
| 工具安装 | 仅内置便携体检工具；复制到本盘新目录、校验文件并实际启动验证 |
| 维护记录 | 报告、对话和确认结果保存在本盘；保留先前记录，拒绝陈旧版本重复确认 |
| 云端 AI | 真实钱包、余额、模型目录及充值页面已验证；有额度的真实 AI 工具调用尚待验收 |
| 本地 AI | Windows CPU 引擎和 Qwen3.5-0.8B 可选包实际启动、问答和停止已验证 |
| PicoClaw | v0.3.1 原生程序已通过本地接口回放；真实付费云端对话尚待验收 |
| 启动救援 | 有准备流程与技能；尚未制作或验证 Ventoy、WinPE、Linux 救援启动环境 |

这次真盘验证使用 E: ORICO exFAT，同一台 Windows 电脑。Linux 核心另外在 WSL Ubuntu 跑通体检和报告；不是 Linux 从这块 U 盘启动的验证。macOS ARM64/x64 有构建产物，实机连接未成功。没有据此宣称三系统、旧电脑性能或跨电脑接续已全部验证。

exFAT 适合共享大模型和镜像文件；格式化成 exFAT 本身不会使 U 盘可启动。重装、分区、克隆、引导修复、实际数据恢复、驱动安装以及 Codex/Node/Claude Code 等通用软件安装，仍需要独立工具适配和验收。

## 标准目录

```text
虾盘.exe / 终端维护菜单.cmd
macos-arm64/  macos-x64/  linux-x64/    平台构建
models/      可选模型权重
runtime/     对应系统的推理引擎、可选 PicoClaw
packages/    待适配的软件包
images/      待验证的系统或救援镜像
drivers/     待适配的驱动包
skills/      六个维护技能
licenses/    第三方许可证
data/        钱包、对话、报告、便携工作区、备份
```

核心 Windows 程序约 8 MB；PicoClaw、运行时和 563 MB GGUF 是可选资源，不应把完整离线包大小说成核心程序大小。大文件不进 Git。

## 构建与验证

```powershell
go test ./...
.\scripts\build.ps1
.\scripts\prepare-picoclaw.ps1
.\scripts\prepare-local-ai.ps1
```

准备脚本使用固定版本与 SHA-256；准备资源后重新构建，把可信清单嵌入程序。默认云端地址为 `catalog/cloud.json` 的公共地址；端点覆盖只接受部署者环境变量 `UCLAW_API_BASE_URL` / `UCLAW_PAY_BASE_URL`，模型文本不能更改端点。

```powershell
.\虾盘.exe action list --json
.\虾盘.exe action run system.inspect --json --no-input
.\虾盘.exe action run resources.scan --json --no-input
.\虾盘.exe action run tools.install --json --input-file install.json
```

安装参数示例：`{"package_id":"portable-check","confirmed":true}`。stdout 为结果 JSON，错误时退出码非零。钱包导入等参数走文件，不把凭证拼到命令历史里。

`scripts/stage-maintenance-preview.ps1` 只针对本次已核验的 E 盘创建新试用目录；不是通用制盘程序。它不格式化、不写引导。旧 `stage-usb-tests.ps1` 和旧测试记录属于 0.1 历史，格式化后的 E 盘不能复用旧路径。

新一轮证据与边界见 [0.2 实测记录](docs/AI维护台0.2实测-2026-10-02.md)。