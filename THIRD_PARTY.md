# 第三方资源说明

本项目自有源码采用 MIT 许可证，见 LICENSE。可选本地 AI 包独立保留上游许可证；不把第三方模型或工具变成本项目自有作品。

| 资源 | 本轮版本与来源 | 许可证 |
| --- | --- | --- |
| llama.cpp CPU 推理运行时 | [官方 b11146 Windows CPU 包](https://github.com/ggml-org/llama.cpp/releases/tag/b11146) | MIT，全文见 licenses/llama.cpp-MIT.txt |
| Qwen3.5-0.8B 模型 | [Qwen 原模型](https://huggingface.co/Qwen/Qwen3.5-0.8B)，[ggml-org 量化文件](https://huggingface.co/ggml-org/Qwen3.5-0.8B-GGUF/tree/8fea620) | Apache 2.0，全文见 licenses/Qwen3.5-Apache-2.0.txt |

试用包使用 Q4_0 GGUF，属于上游模型的量化形式。未修改下载的模型和运行时文件；执行前校验的具体文件与 SHA-256 由 catalog/local-ai.json 记录，并嵌入对应构建。

本轮未打包 Ventoy、Hiren’s BootCD PE、SystemRescue 或其他候选安装软件；功能研究和未来资源适配不等于已分发这些项目。
