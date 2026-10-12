# 第三方资源说明

本项目自有源码采用 MIT 许可证，见 LICENSE。可选本地 AI 包独立保留上游许可证；不把第三方模型或工具变成本项目自有作品。

| 资源 | 本轮版本与来源 | 许可证 |
| --- | --- | --- |
| llama.cpp 推理运行时 | [官方 b11146 Windows CPU 包与 macOS 包](https://github.com/ggml-org/llama.cpp/releases/tag/b11146) | MIT，全文见 licenses/llama.cpp-MIT.txt |
| Qwen3.5-2B 模型 | [Qwen 原模型](https://huggingface.co/Qwen/Qwen3.5-2B)，[Unsloth 固定版本量化文件](https://huggingface.co/unsloth/Qwen3.5-2B-GGUF/tree/f6d5376be1edb4d416d56da11e5397a961aca8ae) | Apache 2.0，全文见 licenses/Qwen3.5-Apache-2.0.txt |
| PicoClaw 可选 Agent | [官方 v0.3.1 Windows 与 Darwin 包](https://github.com/sipeed/picoclaw/releases/tag/v0.3.1) | MIT，全文见 licenses/PicoClaw-MIT.txt |
| Open365 网络引擎（断网急救） | [Open365](https://github.com/dongsheng123132/Open365) `engine/network.ps1`，commit `ab6d25d125a0bdcdbbff1632cf87bf5b5f6b11d9`；版权 Open365 Contributors | Apache-2.0，全文见 licenses/Open365-Apache-2.0.txt |
| WinDirStat 可选便携工具 | [官方 2.9.0 便携包](https://github.com/windirstat/windirstat/releases/tag/release/v2.9.0) | GPL-2.0；工具目录保留 LICENSE.md，工具包附对应源码 |
| PeaZip 可选便携工具 | [官方 11.3.0 WIN64 Portable](https://github.com/peazip/PeaZip/releases/tag/11.3.0) | LGPL-3.0；依赖另按 res/share/copying/third-parties 保留各自声明，工具包附 PeaZip 主项目源码 |
| Notepad++ | [官方 8.9.8.1 x64 Portable](https://github.com/notepad-plus-plus/notepad-plus-plus/releases/tag/v8.9.8.1) | GPL-3.0，保留官方 license.txt，附对应主项目源码 |
| SumatraPDF | [官方 3.6.1 Portable](https://www.sumatrapdfreader.org/download-free-pdf-viewer) | GPL-3.0 及组件声明，保留 COPYING / COPYING.BSD / AUTHORS，附对应项目源码 |
| Explorer++ | [官方 1.4.0 x64](https://github.com/derceg/explorerplusplus/releases/tag/version-1.4.0) | GPL-3.0，保留 License.txt，附对应项目源码 |
| Everything | [官方 1.4.1.1032 x64 ZIP](https://www.voidtools.com/downloads/) | MIT 风格许可及 PCRE 声明，完整保留官网 License.txt |
| CrystalDiskInfo | [官方 9.9.3](https://sourceforge.net/projects/crystaldiskinfo/files/9.9.3/) | MIT 及附带组件声明，便携目录保留声明，app/extras/sources 附对应源码 |
| TestDisk / PhotoRec | [官方 7.2](https://www.cgsecurity.org/wiki/TestDisk_Download) | GPL-2.0-or-later 及附带组件许可；CLI 子集、声明、源码及依赖源码见 app/extras |

Open365 网络引擎原样放在 `engine/network.ps1`（文件头注明来源；唯一修改：诊断时优先选择有默认网关的网卡作为主网卡，避免 Tailscale 等虚拟网卡排在前面导致误判）并内嵌进程序；用途是「断网急救」的只读分层诊断；只运行其 `diagnose` 动作，引擎里的修复动作不会被调用。`licenses/Open365-Apache-2.0.txt` 取自 Open365 仓库的 LICENSE，仅把附录样板里误写的第三方版权行改回标准占位符 `Copyright [yyyy] [name of copyright owner]`，其余条款未动。

本离线维护包使用 Q4_K_M GGUF，属于上游通用模型的量化形式，未进行维修专项微调。未修改下载的模型和运行时文件；执行前校验的具体文件与 SHA-256 由 catalog/local-ai.json 记录，并嵌入对应构建。清单同时记录本次实际使用的 ModelScope 固定版本镜像，文件大小及 SHA-256 与 Hugging Face 上游一致。

本轮未打包 Ventoy、Hiren’s BootCD PE、SystemRescue 或其他候选安装软件；功能研究和未来资源适配不等于已分发这些项目。

本版由 `scripts/package-offline.py` 生成，包含上述六款传统工具及新增硬盘工具。`catalog/tool-packages.json` 是原有工具来源和便携设置的维护入口，`catalog/portable-tools.json` 固定资源哈希，启动前验证；源码及下载记录位于 `app/tool-sources/`。新增工具由 `scripts/prepare-customer-extras.py` 准备，固定资源哈希见 `catalog/offline-tools.json`，来源、对应源码和 Cygwin/GCC 依赖源码与配方保存在 `app/extras/`。TestDisk/PhotoRec 仅附文字界面子集，不分发 QPhotoRec/Qt。

`catalog/tool-library.json` 仅为选装目录；尚未适配或未获再分发许可的项目不打入 ZIP。特别是 [Microsoft Sysinternals](https://learn.microsoft.com/en-us/sysinternals/license-faq) 明确不提供第三方再分发许可，Autoruns、Process Explorer、Process Monitor、RAMMap、TCPView 只提供官方下载入口。

KeePass 仍为选装项，本版未包含。CrystalDiskInfo 已包含；资源校验及签名检查通过，未以客户磁盘执行修复或压力测试。

可选 U-DSH 为用户本机已有的独立原始便携自解压包，不并入本项目许可证。保留包内各组件声明，包装器要求显式输入经审查的原包 SHA-256。其运行边界见 `03-大龙虾-可选/先看说明.txt`，并不作为已完成本地模型联调的离线 Agent 宣传。
