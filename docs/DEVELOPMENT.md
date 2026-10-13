# 同仓库、多平台开发

源码唯一入口：https://github.com/dongsheng123132/xiapan-usb ，本次便携产品维护分支为 `codex/u-picoclaw-portable`。远端 `main` 已转向另一条公司 IT 排障产品线；本分支保留钱包与传统便携工具，不自动覆盖 `main`。不要为 Mac/Linux 另建一份互不同步的项目，也不要直接在出售的 U 盘上开发。

## 当前产品与启动链路

产品范围以 [PRODUCT.md](PRODUCT.md) 为准。`0.5.1-portable-preview` 保留自建轻量首页，并在用户点击“完整 PicoClaw”时打开固定版官方工作台；两个界面接入同一受管理的持久 Gateway。

- 首页立即可用，语言、设置、历史与只读状态在后台加载。不在首页自动开户、查余额、检查升级、扫描硬件或启动完整工作台。
- `pico.ui.open` 按需打开官方界面，`pico.status` 只读查询；关闭浏览器页面不结束 Gateway，明确“退出”才停止本次启动的进程。
- `runtime.cache.status` 只读查询电脑缓存状态。缓存只包含可再生成的已校验程序，首次 AI 使用才复制；Key、钱包、会话、工作文件继续写入本盘。当前不声称所有第三方工具的 I/O 都已转移到宿主电脑。
- 维护工具和确定性诊断在没有模型、网络或余额时仍可调用。本仓不维护本地模型下载/推理功能，也不引入 OpenClaw。

## Mac / Linux

```sh
git clone --branch codex/u-picoclaw-portable https://github.com/dongsheng123132/xiapan-usb.git
cd xiapan-usb
git switch -c codex/macos-portable
go test ./...
go build -o dist/xiapan .
./dist/xiapan action run system.inspect --json --no-input
# 编辑、测试完成后
git add <本次修改的文件>
git commit -m "feat: adapt macOS portable runtime"
git push -u origin codex/macos-portable
```

到 GitHub 发起 PR，目标选择 `codex/u-picoclaw-portable`，检查通过后合入便携产品分支。另一台电脑先保存自己的修改，再 `git pull --rebase`。Linux 分支可使用 `codex/linux-portable`，但 Linux 不属于本次三平台成品配方。不要提交 `data/`、工具二进制或 API Key。

## 实现入口

| 文件 | 职责 |
| --- | --- |
| `core.go` / `main.go` | 无界面动作、CLI、HTTP 接口 |
| `platform_windows.go` / `platform_unix.go` | 系统差异 |
| `maintenance_tools.go` / `portable_tools.go` | 系统及便携工具注册、验证与启动 |
| `pico.go` / `pico_mcp.go` | PicoClaw 适配与 MCP 动作边界 |
| `runtime_cache.go` / `resource_runtime.go` | 已校验程序的宿主缓存、资源路径和校验 |
| `settings.go` | 随盘设置、备份与并发修改保护 |
| `cloud.go` | 随盘钱包、余额、充值和凭证接入 |
| `ui_settings.go` | 界面语言随盘保存与请求语言解析 |
| `web/` | 浏览器界面，只调用动作核心 |
| `web/locales.json` | 中→英词典；界面文案的唯一翻译来源 |
| `catalog/tool-packages.json` | 工具官方来源、版本、许可证、启动参数与便携配置 |
| `catalog/picoclaw.json` | 固定上游版本、发行变体、三平台核心与 launcher 校验值 |
| `catalog/portable-tools.json` / `catalog/emergency-tools.json` | 随盘工具文件与校验清单 |
| `scripts/package-portable.py` | 从构建产物与已验证资源组装新的母盘 |

## 界面文案与多语言

界面文案以**中文原文为 key**，英文写在 `web/locales.json`。新增或修改界面文字时必须同步补词典，否则英文界面会回落成中文。

- 词典放在 `web/locales.json` 顶层，不能放 `web/locales/` 子目录：`main.go` 的 `//go:embed web/*` 不递归，子目录不会进二进制。
- 静态标签由 `i18n.js` 扫描中文文本节点自动绑定；动态文案用 `ui(node, 源文)`、`msg(源文, 变量)`，不要用 `t()` 拼好再当 `raw` 塞进去，否则切换语言不会重译。
- `data-command`、`data-prompt` 是发给后端的原值，**不参与翻译**；`/启动项` 这类斜杠命令始终用中文，后端按字面匹配。
- 对话内容、文件路径、真实 JSON 证据、用户自己起的会话标题都不翻译。
- Go 侧的用户可见文案目前保持中文，由前端词典负责显示层翻译；AI 回答语言由 `requestLanguage` → `responseLanguage` 决定。
- 改了词典或 `web/` 下任何文件都要重新构建：资源是编译期嵌入的，不重新构建不会生效。

`prepare-portable-tools.py` 生成 `catalog/portable-tools.json` 的文件校验清单。修改清单后需要重新构建，不能让模型临时写一个任意可执行路径。

## 固定版本与制盘

本仓程序使用 Go 1.24+。重建固定的 PicoClaw v0.3.1 核心和官方 Web UI 需要上游要求的 Go 1.25.11、Node.js `^20.19.0 || ^22.13.0 || >=24`、pnpm 10.33.0，以及 Python 3.11+。这些是维护者的构建依赖，成品用户无需安装。

```powershell
python scripts/build-picoclaw-pinned.py
.\scripts\build.ps1
python scripts/package-portable.py --resources "D:\已验证工具资源" --output "D:\发盘\U-PicoClaw-0.5.1-portable-preview"
```

1. 固定上游构建脚本校验源码归档，安装锁定前端依赖，应用禁用自升级的发行补丁，构建核心及 launcher 并更新同一平台清单。不在客户 U 盘执行此脚本，不改为下载 latest/nightly。
2. 先提交审核后的源码，再运行 `build.ps1`：它先运行核心测试，再生成各平台主程序和源码指纹。打包要求 Git 工作树干净且构建指纹对应同一提交。改动嵌入的 UI、工具清单或 PicoClaw hash 后都要重新构建。
3. `package-portable.py` 是新成品的唯一组装入口。`--resources` 指向已有的工具资源源目录，`--output` 必须是不存在的新目录；只拷贝已批准资源，创建空白 `data/`，生成发行与校验信息。历史 `package-genie.py` 仅转发到同一配方，不再维护第二套组装逻辑。
4. 复用旧母盘作为工具资源源时，不复制其个人配置、钱包或工作文件；保留旧母盘供回退。9 款便携工具与 9 项系统入口属于 Windows 工具集，不能把这些数量套用于 Mac。

## 验收边界

已产生的平台二进制不等于完成对应系统验收。本轮 Mac 远程测试机不可达，Apple Silicon 和 Intel 包的实体使用仍未验证。

1. 在隔离空白数据根验证首页、原有聊天、完整官方 UI、共享 Gateway、钱包失败分支和退出清理；测试默认不调用真实收费模型、不创建客户钱包。
2. 分别记录冷启动首页、首次程序缓存、缓存命中、Gateway 就绪和模型首个回答时间。真实 U 盘、不同电脑和缓存冷/热条件要写入记录，不把本机服务就绪时间等同于“0.3 秒即用”。
3. 验证中文/空格路径、盘符变化、只读目录、缓存不可写、断网、钱包不可达和拔盘前退出；API Key、会话和工作内容始终以盘内副本为准。
4. Mac 单独验证 Apple Silicon / Intel、首次信任、Unix 执行权限和实体盘路径。Windows EXE 不能作为 Mac 工具；签名、公证与系统授权没有验收前，不宣称所有机器双击即用。
5. 驱动硬件识别、离线资源准备与真正安装/回滚分别验收；目前没有自动匹配、安装和回滚链路，工具成功打开也不等于电脑已经修复。

`docs/ci.yml.example` 提供 Windows、macOS 和 Linux 核心测试与构建模板；当前仓库没有启用中的 workflow。配置 CI 后仍需保留实体 U 盘和桌面交互的独立验收记录。
