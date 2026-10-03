# macOS 版

与 Windows 同一仓库、同一套 Go 动作核心。Mac 差异集中在 `macos.go`（跨平台可测的解析与工具清单）和 `macos_darwin.go`（只在 macOS 编译的系统调用与启动器）。

## 能力与验证状态

| 能力 | macOS 实现 | 状态 |
| --- | --- | --- |
| 启动入口 | `虾盘.app`（arm64 + x64 通用二进制）。双击后启动独立的本机服务并打开浏览器，启动器随即退出，所以关掉页面后再次双击仍可打开 | Apple Silicon 实测，含从 exFAT U 盘启动 |
| 电脑体检 | 系统版本、机型、CPU、总内存、可用内存（`kern.memorystatus_level`）、本机与外接卷（`getfsstat` + `diskutil`）；只读卷和 NTFS 移动盘给出提示 | 实测 |
| 启动项 | `~/Library/LaunchAgents`、`/Library/LaunchAgents`、`/Library/LaunchDaemons`，状态取自 `launchctl print-disabled`；不返回程序路径和参数。不含系统设置「登录项」中的 App（读取需管理员） | 实测 |
| 进程、网络 | `ps`、网卡地址、虾盘云 DNS/HTTPS | 实测 |
| 网卡与驱动 | `networksetup` 网络端口；不读取 MAC 地址。macOS 无 Windows 式驱动故障码 | 实测 |
| 系统工具 | 活动监视器、登录项设置、存储空间设置、磁盘工具、系统信息、控制台、无线诊断，经 LaunchServices 打开固定路径或系统设置面板 | 存在性检测与申请确认实测；未实际点开各面板 |
| AI 助手（PicoClaw） | 官方 v0.3.1 Darwin 包，固定 SHA-256，清单 `catalog/picoclaw.darwin-*.json` | arm64 实测：自带 API 与虾盘云钱包对话、经临时 MCP 调用体检；写操作只生成确认卡片 |
| 离线 AI（llama.cpp） | 官方 b11146 macOS 包，只保留 `llama-server` 实际链接的库；Apple 芯片用 Metal（`gpu_layers: 99`），Intel 保持 CPU | M4 实测：提示词约 1400 token/s，生成约 110 token/s；退出时随服务停止。从本机磁盘启动不到 1 秒，从 U 盘启动约 29 秒（每次启动都完整校验 563 MB 模型） |
| 第三方便携工具 | 未适配。Windows EXE 不算 Mac 工具，工具箱中显示为未准备 | 待做 |

## 开发

```sh
go test ./...                     # macOS 上会额外跑 *_darwin_test.go 实机测试
./scripts/build.sh                # 四个平台 + dist/macos-universal/xiapan
./dist/macos-arm64/xiapan action run system.inspect --json --no-input
```

## 运行时（AI 助手 / 离线 AI）

```sh
python3 scripts/prepare-macos-runtime.py --arch arm64 --with-model   # 或 --arch all；--skip-llama 只要 PicoClaw
./scripts/build.sh                                                    # 重新构建，嵌入新清单
```

脚本只下载其中钉死版本与 SHA-256 的官方归档，校验后才解包；归档中的链接会写成普通文件（exFAT 不支持链接，便携路径检查也拒绝链接）。模型与 Windows 共用 `models/Qwen3.5-0.8B-Q4_0.gguf`。

## 打包

```sh
python3 scripts/package-macos.py                   # 本机测试或直接写入 U 盘：ad-hoc 签名
python3 scripts/package-macos.py --with-local-ai   # 附带离线 AI 运行时与模型
```

```text
虾盘.app/   启动入口（通用二进制）
app/        PicoClaw、可选 llama.cpp 运行时、许可证、使用说明与校验清单
models/     仅 --with-local-ai
data/       用户首次运行后生成的设置、对话、报告和 logs/app.log
```

在 Mac 上写入 exFAT U 盘时，系统会为每个文件生成 `._` 开头的附属文件（来自 `com.apple.provenance` 属性），不影响运行，程序也不会把它们列为盘内资源。出厂盘可在写入后运行 `dot_clean -n /Volumes/<盘名>` 清理，避免在 Windows 上看到这些文件。

首次从 U 盘打开时，macOS 会询问是否允许「虾盘」访问可移除宗卷，选择允许后才能读写盘内的 `app/` 与 `data/`；在用户点击之前，程序会停在第一次访问 U 盘的那一步。若选择了不允许，启动器会提示到「系统设置 › 隐私与安全性 › 文件与文件夹」中开启。ad-hoc 签名的包每次重新构建都会被视为新程序、再次询问；Developer ID 签名后授权可保留。

与 Windows 共用一只盘：把 `虾盘.app` 和 `app/runtime/` 下的 `picoclaw/macos-*`、`macos-*` 目录复制到 Windows 包根目录即可，`data/` 与 `models/` 共用。U 盘请用 **exFAT**：Mac 不能写入 NTFS，体检会提示。

## 签名、公证与 Gatekeeper

- 直接写入 U 盘的文件没有隔离标记，ad-hoc 签名即可运行。
- 从网上下载的压缩包带隔离标记，必须用 Developer ID 签名并公证；不要把「关闭 Gatekeeper」写成使用步骤。
- 运行时的 SHA-256 会嵌入程序，签名会改变文件哈希，所以发布版要在**准备运行时这一步**签名：

```sh
python3 scripts/prepare-macos-runtime.py --arch all --sign-identity "Developer ID Application: …"
./scripts/build.sh
python3 scripts/package-macos.py --sign-identity "Developer ID Application: …" --notary-profile <notarytool 钥匙串配置>
```

  这样生成的 `catalog/*.darwin-*.json` 带有 `"locally_signed": true`，**不要提交**；仓库里保留上游原始文件的哈希。
- 带隔离标记的 App 可能被系统「隔离运行」（App Translocation）到只读临时位置。程序会从挂载表找回 U 盘上的原始位置；找不到时弹窗提示用户在访达中移动一次 `虾盘.app`。此路径需要公证包才能实测，目前只有单元测试。

## 最低系统版本

由编译所用的 Go 版本决定，打包脚本从二进制读取并写入 `Info.plist`：Go 1.24 为 macOS 11，Go 1.25 起为 macOS 12，当前 Go 1.27 构建为 macOS 13。面向旧 Mac 的发布版建议用 Go 1.24 构建，例如 `GOTOOLCHAIN=go1.24.0 ./scripts/build.sh`。

## 尚待实机验收

Intel Mac 与 x64 运行时（仓库尚无 `*.darwin-amd64.json` 清单）、macOS 11–14、U 盘拔插后再用与换电脑、同一只盘在 Windows 上使用、断网启动、公证包的隔离运行。慢速 U 盘上离线 AI 的启动校验可能接近 60 秒的动作超时。CI 通过不等于这些已验收。
