# 同仓库、多平台开发

源码唯一入口：https://github.com/dongsheng123132/xiapan-usb 。不要为 Mac/Linux 另建一份互不同步的项目，也不要直接在出售的 U 盘上开发。

## Mac / Linux

```sh
git clone https://github.com/dongsheng123132/xiapan-usb.git
cd xiapan-usb
git switch -c codex/macos-portable
go test ./...
go build -o dist/xiapan .
./dist/xiapan action run system.inspect --json --no-input
./scripts/build.sh   # 四个平台；macOS 上另生成通用二进制
# 编辑、测试完成后
git add <本次修改的文件>
git commit -m "feat: adapt macOS portable runtime"
git push -u origin codex/macos-portable
```

到 GitHub 发起 PR，检查通过后合入 main。另一台电脑先保存自己的修改，再 `git pull --rebase`。Linux 分支可使用 `codex/linux-portable`。不要提交 `data/`、模型、工具二进制或 API Key。

## 实现入口

| 文件 | 职责 |
| --- | --- |
| `core.go` / `main.go` | 无界面动作、CLI、HTTP 接口 |
| `platform_*.go` | 系统差异 |
| `macos.go` / `macos_darwin.go` | macOS 体检、启动项、系统工具、`虾盘.app` 启动器；见 [macOS 版](MACOS.md) |
| `maintenance_tools.go` / `portable_tools.go` | 系统及便携工具注册、验证与启动 |
| `pico.go` / `pico_mcp.go` | PicoClaw 适配与 MCP 动作边界 |
| `settings.go` | 随盘设置、备份与并发修改保护 |
| `web/` | 浏览器界面，只调用动作核心 |
| `catalog/tool-packages.json` | 工具官方来源、版本、许可证、启动参数与便携配置 |

`prepare-portable-tools.py` 生成 `catalog/portable-tools.json` 的文件校验清单。修改清单后需要重新构建，不能让模型临时写一个任意可执行路径。

## Mac/Linux 完整版还需要完成

macOS 的已完成项、打包与签名流程、待验收清单见 [macOS 版](MACOS.md)。运行时清单按平台放在 `catalog/<名称>.<GOOS>-<GOARCH>.json`，不带后缀的文件仍是 Windows 清单。

1. 为目标系统准备并校验 PicoClaw、本地推理运行时，按平台选择清单（macOS：`scripts/prepare-macos-runtime.py`）。
2. 适配本机进程、磁盘和平台工具；Windows EXE 不能直接算作 Mac/Linux 工具。Mac 第三方便携工具和 Linux 仍待做。
3. 验证 Intel / Apple Silicon、权限提示、外置盘路径、只读目录及退出清理。
4. 测试拔盘再插、换电脑、断网、中文路径、干净系统启动；macOS 单独处理签名与公证，不以关闭安全功能作为默认使用步骤。

`docs/ci.yml.example` 提供 Windows、macOS 和 Linux 核心测试与构建模板。维护者将其放到 `.github/workflows/ci.yml` 即可启用；首次发布使用的 GitHub 凭证不含 workflow 权限，因此当前尚未启用自动运行。CI 通过也不等于实体 U 盘、桌面交互和本地模型已验收。
