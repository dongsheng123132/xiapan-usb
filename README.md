<p align="center">
  <img src="banner.svg" alt="Xiapan USB Toolkit — portable PC maintenance with optional AI" width="100%">
</p>

<p align="center"><a href="README.md">English</a> · <a href="README.zh-CN.md">简体中文</a> · <a href="README.ja.md">日本語</a></p>

# Xiapan USB Toolkit

**Portable PC maintenance tools, with optional AI.**

![Preview](https://img.shields.io/badge/version-0.4.4%20preview-8b6b33)
![Go](https://img.shields.io/badge/Go-1.24%2B-367a8d)
![License](https://img.shields.io/badge/license-MIT-527c46)
![AI optional](https://img.shields.io/badge/AI-optional-4c7770)

[Quick start](#quick-start) · [Platforms](#platforms) · [Build a USB package](#build-a-windows-usb-package) · [Contribute](CONTRIBUTING.md)

> [!NOTE]
> **Preview, not a universal repair disk.** Windows x64 has been tested on physical USB drives. A macOS preview (`虾盘.app`) has been tested on Apple Silicon; Linux packages are still in development. This repository currently provides source; it does not ship all the tools or model weights.

## A maintenance kit you can carry

Inspect a computer, find a useful tool, and keep your settings and reports on your drive. Use the toolkit on its own, or connect AI when you want help interpreting results.

| Capability | What you get |
|------|------|
| Inspect | System information, memory-heavy processes, common startup entries and network devices. |
| Choose a tool | A searchable catalog with clear availability and official download links. |
| Keep it portable | Settings, conversations and maintenance reports stay on your drive. |
| Connect your AI | Bring your own compatible API, use Xiapan Cloud, or prepare an optional local model. |
| Your language | Switch the whole interface between Chinese and English. The choice is saved on the drive, and AI answers follow it. |

### What makes it small and quick

| Property | Reality |
| --- | --- |
| About 44 MiB | The slim package ships the program and PicoClaw; it carries no offline model, no drivers and no rescue images. |
| Double-click to run | No Node.js or Python install, and no per-machine setup. |
| Works offline | Local inspection tools and the maintenance guide need no cloud wallet; the optional offline model runs on the CPU. |
| Written for three systems | Windows, macOS and Linux are build targets for the core. The full packaged drive is **Windows x64 today**; macOS and Linux are not yet a complete packaged experience. |

## Interface language

The language selector in the top bar switches every application-owned label between **简体中文** and **English**. It saves to `data/settings/ui.json` on the drive, so the choice follows you to the next computer, and it also tells the AI which language to answer in.

Chat text, file paths, raw JSON evidence and your own session titles are never translated — only the interface itself. If the language resource fails to load, the interface stays in Chinese rather than breaking.

## Sidebar

The navigation sidebar is visible by default, with a toggle to collapse it into a narrow icon rail. The collapsed state is remembered in the browser. Below 780px the sidebar becomes an overlay drawer that the **History** button opens, so the content is never squeezed on a small screen.

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

The optional tools package includes these six utilities after official downloads and checksum verification. **An entry in the catalog is not an installed application.** Some tools remain download links because of licensing or incomplete validation. System tools include Task Manager, Resource Monitor, Event Viewer, Disk Cleanup, Device Manager and System Information on Windows; Activity Monitor, Login Items, Storage, Disk Utility, System Information, Console and Wireless Diagnostics on macOS.

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
| macOS · Apple Silicon | Universal `虾盘.app`. Inspection, startup items, processes, network, system utilities, PicoClaw and the Metal-accelerated offline model tested on an M4 Mac running macOS 15. Third-party portable utilities are not adapted yet. See [macOS (Chinese)](docs/MACOS.md). |
| macOS · Intel | The app and packaging support x64; x64 runtimes still need preparation and physical-device testing. |
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

## Build a macOS USB package

On a Mac with Go, Python 3 and the Xcode command-line tools:

```sh
python3 scripts/prepare-macos-runtime.py --arch arm64   # optional PicoClaw and offline-model runtimes; add --with-model for the weights
./scripts/build.sh
python3 scripts/package-macos.py                         # add --with-local-ai to include the offline model
```

Double-click `虾盘.app` to open the toolkit in your browser. The default package is ad-hoc signed for drives you write yourself; downloadable copies need Developer ID signing and notarization. See [macOS (Chinese)](docs/MACOS.md).

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
