# 网管精简版开发

唯一源码仓库：`dongsheng123132/xiapan-usb`。当前开发版本 `0.5.0-dev`，交付平台仅 Windows x64；历史 `v0.4.4-preview` Release 保持原样。

```powershell
go test ./...
go vet ./...
.\scripts\build.ps1
.\scripts\prepare-picoclaw.ps1
python scripts/package-genie.py
```

开发依赖 Go、PowerShell 和 Python；成品只需解压后运行。准备脚本固定 PicoClaw 版本与 SHA-256。打包脚本只复制当前构建、已校验 Agent 和许可证，生成新目录、源码提交标识及 ZIP 校验，不复制用户数据。

动作只在 Go 核心实现，GUI、CLI 和 AI 共用。新增动作前必须能说明公司网管的具体故障、现有验证证据及长期维护人；一个工作日不能交付可验证结果的功能先拆小或不做。

`web/locales.json` 为中英界面词典。修改 `web/`、技能或内嵌网络脚本后重新构建。业务测动作核心，权限测本机 HTTP 与 MCP，界面仅做必要交互验证。

不恢复云钱包、离线模型、软件下载目录、跨平台完整包或启动救援入口。先验证真实客户的复用频率，再决定是否另开项目。
