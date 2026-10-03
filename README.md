# Xiapan · IT Support Toolkit

**For company IT staff on Windows: select a problem → inspect evidence → open a system tool → save a report.**

[简体中文](README.zh-CN.md) · [日本語](README.ja.md)

This source is **0.5.0-dev**, an unpublished, reduced Windows x64 edition. The [product page](https://usb.u-claw.org.cn/) and [v0.4.4-preview Release](https://github.com/dongsheng123132/xiapan-usb/releases/tag/v0.4.4-preview) still distribute the earlier product.

- Inspect system memory, disk space, processes and startup entries.
- Diagnose adapters, gateway reachability, DNS, web access and proxies without changing settings.
- Read network-device hardware IDs, fault codes and driver information.
- Open seven registered Windows system tools; actual changes remain with the operator.
- Keep sessions and reports on the drive, including recorded inspection evidence.

Inspections need no AI, account or payment. Optional AI uses one company-approved OpenAI-compatible API through PicoClaw. Sending an AI question shares its conversation and relevant evidence with that endpoint. Credentials and report backups are currently stored unencrypted on the drive.

## Build on Windows x64

```powershell
go test ./...
.\scripts\build.ps1
.\scripts\prepare-picoclaw.ps1
python scripts/package-genie.py
```

Extract a built package to a writable folder or USB drive and run `虾盘.exe`. No Node.js or Python installation is needed to use the package. [Development](docs/DEVELOPMENT.md) · [Distribution](docs/DISTRIBUTION.md).

Removed: cloud wallets, offline model management, multiple AI engines, third-party utility bundles and recommendation catalogs, resource scanning, generic writing prompts, the macOS packaging pipeline and boot-recovery entry points. Existing user files are preserved. Older cloud settings remain untouched until the operator saves a company API configuration, with a backup.

Public network probes may be restricted by company policy; they do not prove all business services work or fail. Do not bypass corporate DNS, proxies, VPN or endpoint controls. This is not an installer, automatic repair tool, bootable rescue disk or centrally managed fleet service.

Project code: [MIT](LICENSE). [Third-party notices](THIRD_PARTY.md) · [Scope decisions](docs/SCOPE.md).
