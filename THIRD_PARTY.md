# 第三方资源说明

本项目自有源码采用 MIT 许可证，见 LICENSE。可选本地 AI 包独立保留上游许可证；不把第三方模型或工具变成本项目自有作品。

| 资源 | 本轮版本与来源 | 许可证 |
| --- | --- | --- |
| llama.cpp 推理运行时 | [官方 b11146 Windows CPU 包与 macOS 包](https://github.com/ggml-org/llama.cpp/releases/tag/b11146) | MIT，全文见 licenses/llama.cpp-MIT.txt |
| Qwen3.5-0.8B 模型 | [Qwen 原模型](https://huggingface.co/Qwen/Qwen3.5-0.8B)，[ggml-org 量化文件](https://huggingface.co/ggml-org/Qwen3.5-0.8B-GGUF/tree/8fea620) | Apache 2.0，全文见 licenses/Qwen3.5-Apache-2.0.txt |
| PicoClaw 可选 Agent | [官方 v0.3.1 Windows 与 Darwin 包](https://github.com/sipeed/picoclaw/releases/tag/v0.3.1) | MIT，全文见 licenses/PicoClaw-MIT.txt |
| Open365 网络引擎（断网急救） | [Open365](https://github.com/dongsheng123132/Open365) `engine/network.ps1`，commit `ab6d25d125a0bdcdbbff1632cf87bf5b5f6b11d9`；版权 Open365 Contributors | Apache-2.0，全文见 licenses/Open365-Apache-2.0.txt |
| WinDirStat 可选便携工具 | [官方 2.9.0 便携包](https://github.com/windirstat/windirstat/releases/tag/release/v2.9.0) | GPL-2.0；工具目录保留 LICENSE.md，工具包附对应源码 |
| PeaZip 可选便携工具 | [官方 11.3.0 WIN64 Portable](https://github.com/peazip/PeaZip/releases/tag/11.3.0) | LGPL-3.0；依赖另按 res/share/copying/third-parties 保留各自声明，工具包附 PeaZip 主项目源码 |
| Notepad++ | [官方 8.9.8.1 x64 Portable](https://github.com/notepad-plus-plus/notepad-plus-plus/releases/tag/v8.9.8.1) | GPL-3.0，保留官方 license.txt，附对应主项目源码 |
| SumatraPDF | [官方 3.6.1 Portable](https://www.sumatrapdfreader.org/download-free-pdf-viewer) | GPL-3.0 及组件声明，保留 COPYING / COPYING.BSD / AUTHORS，附对应项目源码 |
| Explorer++ | [官方 1.4.0 x64](https://github.com/derceg/explorerplusplus/releases/tag/version-1.4.0) | GPL-3.0，保留 License.txt，附对应项目源码 |
| Everything | [官方 1.4.1.1032 x64 ZIP](https://www.voidtools.com/downloads/) | MIT 风格许可及 PCRE 声明，完整保留官网 License.txt |

Open365 网络引擎原样放在 `engine/network.ps1`（文件头注明来源；唯一修改：诊断时优先选择有默认网关的网卡作为主网卡，避免 Tailscale 等虚拟网卡排在前面导致误判）并内嵌进程序；用途是「断网急救」的只读分层诊断；只运行其 `diagnose` 动作，引擎里的修复动作不会被调用。`licenses/Open365-Apache-2.0.txt` 取自 Open365 仓库的 LICENSE，仅把附录样板里误写的第三方版权行改回标准占位符 `Copyright [yyyy] [name of copyright owner]`，其余条款未动。

试用包使用 Q4_0 GGUF，属于上游模型的量化形式。未修改下载的模型和运行时文件；执行前校验的具体文件与 SHA-256 由 catalog/local-ai.json 记录，并嵌入对应构建。

本轮未打包 Ventoy、Hiren’s BootCD PE、SystemRescue 或其他候选安装软件；功能研究和未来资源适配不等于已分发这些项目。

`package-genie.py --with-tools` 包含上述六款传统工具；不带该参数的精简包不包含它们。完整上游便携资源保留，未修改第三方二进制。`catalog/tool-packages.json` 是来源和便携设置的维护入口，准备脚本生成 `catalog/portable-tools.json` 固定资源哈希；启动前验证。源码及上游下载记录位于工具包的 `app/tool-sources/`。更新时保留各工具的用户配置。本轮交付用户 F 盘本地试用，未对外发布。

`catalog/tool-library.json` 仅为选装目录；尚未适配或未获再分发许可的项目不打入 ZIP。特别是 [Microsoft Sysinternals](https://learn.microsoft.com/en-us/sysinternals/license-faq) 明确不提供第三方再分发许可，Autoruns、Process Explorer、Process Monitor、RAMMap、TCPView 只提供官方下载入口。

KeePass 与 CrystalDiskInfo 的官方便携包已调研，但本机窗口启动验收未完成，改列选装目录；发行包不包含它们的程序。
