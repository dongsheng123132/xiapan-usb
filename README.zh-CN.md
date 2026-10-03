# 虾盘 · 公司网管工具箱

**给 Windows 公司网管：选故障 → 检测 → 看证据 → 留报告。**

[English](README.md) · [日本語](README.ja.md)

当前源码为 **0.5.0-dev 网管精简开发版**，尚未公开发行。[产品页](https://usb.u-claw.org.cn/) 和 [v0.4.4-preview Release](https://github.com/dongsheng123132/xiapan-usb/releases/tag/v0.4.4-preview) 仍提供历史版本，功能范围与本分支不同。

## 只做现场排障

- 电脑卡顿、空间不足：系统、内存、磁盘、进程和启动项只读检查。
- 网页打不开、网络异常：网卡、网关、DNS、网页与代理分层诊断。
- 网卡缺失、驱动异常：硬件 ID、故障码、驱动版本与 INF 信息。
- 打开 Windows 自带的任务管理器、设备管理器、资源监视器、事件查看器、系统信息、磁盘清理或网络设置。
- 保存当前会话的检测证据和新的体检报告，记录随盘移动。

检测按钮不需要 AI、账号或充值。AI 只有一种可选接入：公司提供的 OpenAI 兼容 API，通过 PicoClaw 解释证据并申请维护动作。只有发送 AI 问题时才发送会话与相关证据；测试连接单独发送测试请求。

## 使用与构建

完整包解压到可写目录或 U 盘，双击 `虾盘.exe`。公司电脑需能进入 Windows，当前交付目标仅 Windows x64。源码构建见 [开发说明](docs/DEVELOPMENT.md)。

```powershell
go test ./...
.\scripts\build.ps1
.\scripts\prepare-picoclaw.ps1
python scripts/package-genie.py
```

动作核心也可直接调用：

```powershell
.\虾盘.exe action list --json
.\虾盘.exe action run system.inspect --json --no-input
.\虾盘.exe action run network.rescue --json --no-input
```

## 边界

本程序不自动修改网络、结束进程、禁用启动项或安装驱动。公网探测需结合公司代理、DNS、VPN、准入策略及实际业务判断；不建议绕过公司管控。系统工具里的操作由网管负责，打开工具不代表已修复。

已删除云钱包、离线模型、软件推荐与打包库、资源扫描、通用写作入口、Mac 完整包链路及启动救援入口。已有 `data/`、钱包、模型与工具文件不被清理。旧云设置需在 AI 设置中改为公司 API，原文件保留到用户主动保存，并留备份。

密钥、历史备份和报告目前以明文随盘保存。更新保留 `data/`；制包不得复制用过的数据。见 [交付说明](docs/DISTRIBUTION.md)、[删减取舍](docs/SCOPE.md)、[第三方组件](THIRD_PARTY.md)。

自有代码采用 [MIT](LICENSE)。
