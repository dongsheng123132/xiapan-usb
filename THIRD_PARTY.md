# 第三方资源说明

本项目自有源码采用 MIT 许可证，见 LICENSE。可选本地 AI 包独立保留上游许可证；不把第三方模型或工具变成本项目自有作品。

| 资源 | 本轮版本与来源 | 许可证 |
| --- | --- | --- |
| llama.cpp CPU 推理运行时 | [官方 b11146 Windows CPU 包](https://github.com/ggml-org/llama.cpp/releases/tag/b11146) | MIT，全文见 licenses/llama.cpp-MIT.txt |
| Qwen3.5-0.8B 模型 | [Qwen 原模型](https://huggingface.co/Qwen/Qwen3.5-0.8B)，[ggml-org 量化文件](https://huggingface.co/ggml-org/Qwen3.5-0.8B-GGUF/tree/8fea620) | Apache 2.0，全文见 licenses/Qwen3.5-Apache-2.0.txt |
| PicoClaw 可选 Agent | [官方 v0.3.1 Windows 包](https://github.com/sipeed/picoclaw/releases/tag/v0.3.1) | MIT，全文见 licenses/PicoClaw-MIT.txt |
| WinDirStat 可选便携工具 | [官方 2.9.0 便携包](https://github.com/windirstat/windirstat/releases/tag/release/v2.9.0) | GPL-2.0；工具目录保留 LICENSE.md，工具包附对应源码 |
| PeaZip 可选便携工具 | [官方 11.3.0 WIN64 Portable](https://github.com/peazip/PeaZip/releases/tag/11.3.0) | LGPL-3.0；依赖另按 res/share/copying/third-parties 保留各自声明，工具包附 PeaZip 主项目源码 |

试用包使用 Q4_0 GGUF，属于上游模型的量化形式。未修改下载的模型和运行时文件；执行前校验的具体文件与 SHA-256 由 catalog/local-ai.json 记录，并嵌入对应构建。

本轮未打包 Ventoy、Hiren’s BootCD PE、SystemRescue 或其他候选安装软件；功能研究和未来资源适配不等于已分发这些项目。

`package-genie.py --with-tools` 包含 WinDirStat 与 PeaZip；不带该参数的精简包仍不包含它们。完整上游便携资源保留，未修改第三方二进制。`catalog/portable-tools.json` 记录注册入口、平台及固定资源哈希；启动前验证。源码及上游下载记录位于工具包的 `app/tool-sources/`。WinDirStat 启用旁置 INI，PeaZip 使用上游 Portable 配置；更新时保留各工具的用户配置。此工具包本轮用于用户两块 U 盘的本地试用，未对外发布。
