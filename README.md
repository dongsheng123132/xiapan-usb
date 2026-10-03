# 虾盘 · U 盘精灵

0.4.1-preview。沿用 U-Claw / ClawX 的便携逻辑：一个启动入口，模型设置、密钥与对话随盘保存。PicoClaw 负责一般问答和维护工具调用；可以使用虾盘云，也可以在页面中设置自己的 OpenAI 兼容 API。浏览器界面默认只显示对话，历史按需展开。Go 动作核心同时供 GUI、CLI 与 PicoClaw MCP 调用。

## 使用

Windows：双击 U 盘目录里的 `虾盘.exe`。默认 PicoClaw，右上角「模型设置」可切换虾盘云或自带 API，可输入问题继续对话，或使用 `/启动项`、`/体检`、`/进程`、`/网络`、`/驱动`、`/工具`、`/资源`、`/模型`、`/安装`、`/报告`、`/救援`。只读检测可直接执行；安装、保存报告及 AI 申请打开工具需要确认。用户直接点击工具的「打开」按钮时不再重复确认；需要管理员权限的原生工具由 Windows 提示授权。

使用虾盘云需要钱包额度，可从模型设置打开钱包并启用，也可导入已有凭证；启动页面不自动创建钱包。自带 API 不需要虾盘钱包。凭证就是钱包，请自行备份；充值按钮在系统浏览器打开官方页面，由用户选择并付款。查询失败显示未知，不显示假零余额。钱包轮换支持中断续接，移除本盘引用不会删除服务端余额。用户凭证和运行数据不进源码或发行模板。

PicoClaw 是默认原生 Agent，使用已保存的模型选择、便携工作区和六个维护技能。通过 MCP 主动调用与 GUI/CLI 相同的维护核心，读取证据、申请下一步；没有关键词预先替它检测的逻辑。其任意命令、文件写入、技能安装和子代理工具保持禁用。模型的“已确认”参数不能绕过页面确认。虾盘云直连是备用引擎。

工具集合可打开当前 Windows 自带的任务管理器、磁盘清理、设备管理器、系统信息、资源监视器和事件查看器。常用工具包另带 WinDirStat 和 PeaZip，后续操作由用户在工具里完成。Ventoy、SystemRescue、Hiren’s BootCD PE 列为官方资源入口，尚未打包或制作启动层。网络驱动检测无需联网，读取硬件 ID 和故障码，列出盘内驱动包；不会根据文件名猜匹配或自动安装。维护 skills 规定步骤和成功判据，不能代替实际工具。

断网时可直接用本地工具或维护向导。已带齐 Windows 模型资源的试用包还可选择「本盘离线 AI」，首次提问时启动；小模型说明需要结合实际检测，不能凭它的回答认定电脑故障。界面退出会停止本程序启动的模型；之后按系统方式安全移除 U 盘。

旧维护盘可继续双击 `终端维护菜单.cmd`；精简包通过 `虾盘.exe action run <动作> --json --no-input` 使用同一核心。页面专业模式展示真实 JSON；Ctrl K 打开动作菜单。

## 当前能力和边界

0.4.1 增加可选常用工具包：WinDirStat 2.9.0、PeaZip Portable 11.3.0，以及 Windows 资源监视器、事件查看器入口。界面、CLI 和 AI 确认卡片共用 `tools.launch`；本盘工具执行前按固定清单校验，模型不能指定任意程序路径或命令参数。

目前完整包支持 Windows x64。macOS ARM64/x64、Linux x64 仅有主程序构建目标，PicoClaw 运行时清单及新增便携工具仍限定 Windows x64，尚不能宣称三系统完整可用。自带 API 设置不绑定电脑；密钥及历史备份以明文随盘保存，不自动过期。公网服务仍需可达且凭证有效，localhost 服务需要在当前电脑另行启动。

| 能力 | 本轮状态 |
| --- | --- |
| 模型设置 | 自带 OpenAI 兼容服务 / 虾盘云；连接与工具调用测试；原子保存、备份与版本冲突保护 |
| 启动项读取 | Windows Run / Run32、当前用户和公共启动文件夹；状态缺失标为未知，不包含全部服务或计划任务，不关闭启动项 |
| 系统体检 | 系统、架构、逻辑处理器、总内存、可用内存；Windows 磁盘空间 |
| 进程检查 | 读取工作集内存，按占用排序；不结束进程、不读取命令行 |
| 网络诊断 | 网卡、虾盘云 DNS 和 HTTPS；可达不等于已获得模型调用权限 |
| 离线网络驱动 | Windows 网络设备、未知缺驱动设备、硬件 ID、驱动版本、INF 和故障码；盘内驱动库存 |
| 工具集合 | 6 个 Windows 系统工具入口；可选 WinDirStat / PeaZip 便携包；传统救援资源的官方入口 |
| 盘内资源 | 扫描 models、packages、images、drivers、runtime，不执行发现的未知文件 |
| 工具安装 | 仅内置便携体检工具；复制到本盘新目录、校验文件并实际启动验证 |
| 维护记录 | 报告、对话和确认结果保存在本盘；保留先前记录，拒绝陈旧版本重复确认 |
| 云端 AI | 真实充值到账后，PicoClaw 已调用本机体检、根据结果回答；用户确认后实际保存报告 |
| 本地 AI | Windows CPU 引擎和 Qwen3.5-0.8B 可选包实际启动、问答和停止已验证 |
| PicoClaw | v0.3.1 原生程序通过本地接口回放及真实虾盘云调用，维护动作与确认结果可核验 |
| 启动救援 | 有准备流程与技能；尚未制作或验证 Ventoy、WinPE、Linux 救援启动环境 |

0.4 精简包已在 E: ORICO exFAT、F: SanDisk NTFS 两块实盘验证，使用同一台 Windows 电脑，详见 `docs/双U盘实测-20261003.md`。Linux 核心另在 WSL Ubuntu 跑通体检和报告；不是 Linux 从 U 盘启动的验证。macOS ARM64/x64 有构建产物，实机连接未成功。没有据此宣称三系统、旧电脑性能或跨电脑接续已全部验证。

exFAT 适合共享大模型和镜像文件；格式化成 exFAT 本身不会使 U 盘可启动。重装、分区、克隆、引导修复、实际数据恢复、驱动安装以及 Codex/Node/Claude Code 等通用软件安装，仍需要独立工具适配和验收。

## 精简便携包

运行 `python scripts/package-genie.py`，从当前代码构建的 Windows 程序和已校验 PicoClaw 生成新目录及带 SHA-256 的 ZIP：

```text
虾盘.exe     双击启动
app/         PicoClaw 程序、许可证、说明及文件校验清单
data/        首次使用后保存模型设置、凭据与备份、对话和报告
```

约 44 MiB 的程序文件，不含离线模型、官方 PicoClaw Launcher、其他平台程序、驱动或救援镜像。无需安装 Node/Python。现有维护盘的 `runtime/`、`models/`、`drivers/` 等布局继续兼容；离线资源另行准备。新包永远不复制开发机或旧盘的用户数据。自带密钥保存在 `data/settings/model.json`，历史备份也可能含密钥，应一并保管。

设置页的地址填写服务商的 API 基础地址（通常以 `/v1` 结尾），模型 ID 原样保存。API Key 留空保留同一地址下的旧密钥；更改地址后需重新输入，不把旧密钥发往新端点。设置动作仅用户界面/CLI 可调用，模型不能修改自身服务配置。测试连接发送一次真实模型请求但不保存设置。旧设置与损坏文件均不会被静默覆盖。

退出后安全移除 U 盘。换目录接续已使用本机独立测试目录验证；不同电脑、真实换盘符、裸网及 UAC 交互仍需单独验收。系统和系统浏览器可能留下自己的运行痕迹。

## 构建与验证

```powershell
go test ./...
.\scripts\build.ps1
.\scripts\prepare-picoclaw.ps1
.\scripts\prepare-local-ai.ps1  # 仅离线选装包需要
python scripts/verify-genie.py
python scripts/package-genie.py
```

常用工具包先运行 `python scripts/prepare-portable-tools.py`，再重新构建，最后运行 `python scripts/package-genie.py --with-tools`。准备脚本拒绝覆盖已经存在的工具目录；运行过的工具配置不能当发行模板。工具包包含上游许可证和项目源码归档，维护逻辑仍只有一份。

准备脚本使用固定版本与 SHA-256；准备资源后重新构建，把可信清单嵌入程序。默认云端地址为 `catalog/cloud.json` 的公共地址；虾盘云端点覆盖接受部署者环境变量 `UCLAW_API_BASE_URL` / `UCLAW_PAY_BASE_URL`；自带 API 使用用户设置，模型文本不能更改端点。

```powershell
.\虾盘.exe action list --json
.\虾盘.exe action run system.inspect --json --no-input
.\虾盘.exe action run resources.scan --json --no-input
.\虾盘.exe action run tools.install --json --input-file install.json
```

安装参数示例：`{"package_id":"portable-check","confirmed":true}`。stdout 为结果 JSON，错误时退出码非零。钱包导入等参数走文件，不把凭证拼到命令历史里。

`scripts/stage-maintenance-preview.ps1` 只针对本次已核验的 E 盘创建新试用目录；不是通用制盘程序。它不格式化、不写引导。旧 `stage-usb-tests.ps1` 和旧测试记录属于 0.1 历史，格式化后的 E 盘不能复用旧路径。

新一轮证据与边界见 [PicoClaw 0.3 实测](docs/PicoClaw维护盘0.3实测-2026-10-02.md)；[0.2 实测记录](docs/AI维护台0.2实测-2026-10-02.md)保留为历史。

0.4 的旧版参考与验证说明见 [便携精灵 0.4](docs/便携精灵0.4-实现与验证.md)。
