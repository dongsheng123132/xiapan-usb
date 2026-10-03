<p align="center">
  <img src="banner.svg" alt="Xiapan USB Toolkit — portable PC maintenance with optional AI" width="100%">
</p>

<p align="center"><a href="README.md">English</a> · <a href="README.zh-CN.md">简体中文</a> · <a href="README.ja.md">日本語</a></p>

# Xiapan USB Toolkit

**Portable PC maintenance tools, with optional AI.**

![Preview](https://img.shields.io/badge/version-0.4.2%20preview-8b6b33)
![Go](https://img.shields.io/badge/Go-1.24%2B-367a8d)
![License](https://img.shields.io/badge/license-MIT-527c46)
![AI optional](https://img.shields.io/badge/AI-optional-4c7770)

[Quick start](#quick-start) · [Platforms](#platforms) · [Build a USB package](#build-a-windows-usb-package) · [Contribute](CONTRIBUTING.md)

> [!NOTE]
> **Preview, not a universal repair disk.** Windows x64 has been tested on physical USB drives. Full macOS and Linux packages are still in development. This repository currently provides source; it does not ship all the tools or model weights.

## A maintenance kit you can carry

Inspect a computer, find a useful tool, and keep your settings and reports on your drive. Use the toolkit on its own, or connect AI when you want help interpreting results.

| Capability | What you get |
|------|------|
| Inspect | System information, memory-heavy processes, common startup entries and network devices. |
| Choose a tool | A searchable catalog with clear availability and official download links. |
| Keep it portable | Settings, conversations and maintenance reports stay on your drive. |
| Connect your AI | Bring your own compatible API, use Xiapan Cloud, or prepare an optional local model. |

## Quick start

Install **Git and Go 1.24+**, then build the core from the same repository on Windows, macOS or Linux:

```sh
git clone https://github.com/dongsheng123132/xiapan-usb.git
cd xiapan-usb
go test ./...
go build -o dist/xiapan .
./dist/xiapan action run system.inspect --json --no-input
```

On Windows, use `go build -o dist/xiapan.exe .` and `./dist/xiapan.exe` instead. The built core does not automatically include PicoClaw, local models or third-party tools. See the [development guide (Chinese)](docs/DEVELOPMENT.md) for platform work and contribution branches.

## The toolbox

**35 catalog entries:** 6 adapted portable utilities, 6 Windows system tools, 19 optional applications, and 4 recovery/driver resources. Search by name or purpose, then filter by category and availability.

| Adapted utility | Purpose |
| --- | --- |
| WinDirStat | Understand disk usage |
| PeaZip | Create and extract archives |
| Everything | Find files quickly |
| Explorer++ | Browse and manage files |
| Notepad++ | Edit text and configuration files |
| SumatraPDF | Read PDF documents |

The optional tools package includes these six utilities after official downloads and checksum verification. **An entry in the catalog is not an installed application.** Some tools remain download links because of licensing or incomplete validation. System tools include Task Manager, Resource Monitor, Event Viewer, Disk Cleanup, Device Manager and System Information.

## AI is a choice

| Mode | How it works |
| --- | --- |
| Without AI | Run supported inspections and open tools locally, without a cloud wallet. |
| Online AI | Use the optional PicoClaw agent with your own OpenAI-compatible endpoint and model, or choose Xiapan Cloud. |
| Offline AI | Prepare the separate Windows CPU runtime and Qwen3.5-0.8B resource pack. It is not included in the small default package. |

AI uses the same action core as the interface and CLI. Arbitrary shell execution, file editing, skill installation and subagents are disabled in the PicoClaw adapter. Model-generated confirmation flags cannot bypass user confirmation.

![Browser, CLI and optional AI share one maintenance core](diagrams/action-core.svg)

## Platforms

| Platform | Verified scope |
| --- | --- |
| **Windows x64** | Basic flows on two physical USB drives; the six-utility package on one drive, using the same Windows computer. This is not a compatibility guarantee for every PC. |
| macOS · Apple Silicon / Intel | Core cross-build targets exist. Native agent/model runtimes, system utilities and physical-device testing remain to be completed. |
| Linux x64 | Core build and WSL inspection/report flows tested. Full desktop and physical USB workflows still need validation. |

Several platform binaries can share one drive, but each OS runs its own executable. **This is not currently a bootable rescue OS** and cannot run inside a computer whose operating system will not start. Startup inspection covers common Run entries and startup folders, not every service or scheduled task; it does not disable entries automatically.

## Build a Windows USB package

Packaging requires Go, PowerShell and Python 3. End users do not need Go, Python or Node to run the resulting package.

```powershell
.\scripts\prepare-picoclaw.ps1
.\scripts\build.ps1
python scripts/package-genie.py
```

<details>
<summary>Include portable utilities or prepare offline AI</summary>

Run `python scripts/prepare-portable-tools.py`, rebuild, then run `python scripts/package-genie.py --with-tools`. Preparation uses pinned versions and SHA-256 values and refuses to overwrite an existing tool directory.

For the separately supported local-model resources, use `scripts/prepare-local-ai.ps1`. The default small package excludes model weights, drivers and rescue images.

</details>

```text
虾盘.exe   Launch the toolkit
app/       Engines, optional utilities, licenses and checksums
data/      Your settings, conversations and reports
```

> [!IMPORTANT]
> API keys and their backups are currently stored **in plaintext on the drive**. Keep the drive private. Moving it to another PC still requires a reachable provider and a valid key; a localhost model server must run on that PC. Preserve `data/` and each tool's personal configuration when updating. Never ship a used data directory to another customer.

## Open source, with optional preloaded drives

Our source is [MIT licensed](LICENSE). Download it and build your own drive, or choose a preloaded physical drive when available. Hardware, predownloaded models, verified setup and support are the service being sold. Cloud credit is optional and does not unlock basic local maintenance.

Third-party software and models retain their own licenses. Read [third-party notices](THIRD_PARTY.md) and the [distribution guide (Chinese)](docs/DISTRIBUTION.md) before redistributing a package.

<details>
<summary>Project artwork</summary>

![Xiapan USB Toolkit concept illustration](assets/xiapan-usb-concept.png)

Concept artwork, not a photograph of a shipping device. [Design notes and tooling](docs/README-DESIGN.md).

</details>

## Help build it

Platform adapters, reproducible bug reports and portable-tool recipes are welcome. Start with [Contributing](CONTRIBUTING.md), [Security](SECURITY.md) or [open an issue](https://github.com/dongsheng123132/xiapan-usb/issues). Do not post wallet links, API keys or private diagnostics.

[Xiapan Cloud](https://cloud.u-claw.org/) · [U-King](https://www.u-king.org/) · [Xiapan Canvas](https://tu.u-claw.org.cn/)
