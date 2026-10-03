# Third-party components / 第三方组件

Project-owned code is MIT licensed. Third-party notices remain separate.

| Component | Pinned source | License |
| --- | --- | --- |
| PicoClaw agent, Windows x64 | [v0.3.1](https://github.com/sipeed/picoclaw/releases/tag/v0.3.1); checksum in `catalog/picoclaw.json` | [MIT](licenses/PicoClaw-MIT.txt) |
| Open365 network probes | [Open365](https://github.com/dongsheng123132/Open365), commit `ab6d25d125a0bdcdbbff1632cf87bf5b5f6b11d9`, `engine/network.ps1`; copyright Open365 Contributors | [Apache-2.0](licenses/Open365-Apache-2.0.txt) |

Xiapan modifications to Open365 are documented in the script header: prefer an adapter with a default gateway; remove all repair commands, administrator escalation and speculative repair advice. Only read-only probes remain. Company-facing next steps are selected from measured results in the Go core.

The package no longer bundles third-party maintenance utilities or model weights. Windows system tools are opened from the current operating system and are not redistributed. Existing user-provided tools and data are not deleted.
