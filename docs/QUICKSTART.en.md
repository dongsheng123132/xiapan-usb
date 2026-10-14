# Xiapan Portable AI USB · Quick Start

For version 0.5.4. **Portable AI, a wallet that travels with you, cached programs, offline diagnostics, and a computer maintenance toolkit.**

## Copy to a USB drive

Copy **everything inside the fresh master folder** to your USB drive, keeping the folder structure. Use your file manager or the ordinary file-copy mode of a USB distribution tool. Formatting and bootable-drive creation are not required. Wait for copying to finish, then use the launcher on the USB drive.

Prepare each customer's drive from a clean master with an empty `data/` folder. Never distribute your used `data/`: it may contain API keys, a wallet, conversations, and work files. To verify copied files, run `Verify Files.ps1` in Windows PowerShell, or open `Verify Files Mac.command` on a Mac (or run it with `sh` in Terminal). Verification reads fixed files and excludes personal `data/`.

## Start the app

| Computer | Open this file on the USB drive |
| --- | --- |
| Windows x64 | `Start Windows.cmd` or `开始使用.cmd` |
| Mac, Apple Silicon or Intel | `Start Mac.command` or `启动 Mac.command` |

The lightweight home interface supports English and Chinese. Chat here for everyday tasks, or open **Full PicoClaw** for the official workspace and advanced options. Both interfaces share one persistent AI service. Languages available in third-party tools and the official workspace depend on those products.

On a Mac, the first launch may require system trust or file-access permission. If double-clicking the script does not work, open Terminal, type `sh` followed by a space, drag the `.command` file into Terminal, and press Return. The script copies programs to the computer's cache and sets their executable permissions; it does not disable Gatekeeper. Apple Silicon has passed startup, native file tools, credential restart and exit checks on an ExFAT USB drive. Intel is build-only; first Finder trust and all system permissions still need verification on the target Mac.

## Choose an AI service

**Xiapan Cloud:** Open the wallet and follow the prompts to bind or create this drive's wallet. The recharge button opens the payment page; refresh the balance after payment. The wallet and its credentials stay on the USB drive for use on another computer.

**Your own API:** Open **Model settings**, select **Own API · OpenAI-compatible**, and enter the API base URL, API key, and model ID. A Xiapan Cloud wallet is not required.

| Provider | API base URL | Provider documentation |
| --- | --- | --- |
| Kimi | `https://api.moonshot.cn/v1` | [Kimi API compatibility](https://platform.kimi.com/docs/guide/migrating-from-openai-to-kimi) |
| Zhipu | `https://open.bigmodel.cn/api/paas/v4` | [Zhipu developer documentation](https://docs.bigmodel.cn) |
| Gemini | `https://generativelanguage.googleapis.com/v1beta/openai` | [Gemini OpenAI compatibility](https://ai.google.dev/gemini-api/docs/openai) |

Use a model ID available to your provider account. Choose a model with tool calling if you want the AI to operate tools. **Saving settings does not call a model. Test connection sends one model request and may incur a charge.** Your own API usage is billed to the corresponding provider account.

Gemini's compatibility mode is not guaranteed to support complex, multi-turn tool interactions. For those tasks, configure native Gemini in Full PicoClaw. Online AI requires access to your chosen provider; availability depends on your region and network.

## Use the computer maintenance toolkit offline

Local diagnostics and tool launchers work without AI, a balance, or an internet connection. Review the diagnostic results, then choose a tool. Opening a tool does not automatically repair the computer.

- **Windows: 9 bundled portable tools.** WinDirStat, Everything, PeaZip, Notepad++, SumatraPDF, Explorer++, CrystalDiskInfo, PhotoRec, and TestDisk cover disk space, file search, archives, text/PDF viewing, disk health, and data recovery.
- **Windows: 9 system tool shortcuts.** Memory Diagnostic, Network Adapters, Network Settings, Task Manager, Disk Cleanup, Device Manager, System Information, Resource Monitor, and Event Viewer.
- **Mac: 5 built-in system tool shortcuts.** Activity Monitor, Disk Utility, Console, System Information, and Wireless Diagnostics. These open tools already installed on the Mac; they are not third-party Mac applications bundled on the USB drive. Missing tools are shown as unavailable.

The drive includes **3DP Net 21.01, a legacy 2021 network driver collection**, with instructions, as a Windows emergency resource. It is not a universal driver pack and does not automatically match or install drivers. Check the operating system and hardware model first. This product runs inside your operating system; it is not WinPE or a bootable rescue drive.

## Mac networking and startup recovery

The Mac network view shows actual interfaces, their state and addresses, and can open Wireless Diagnostics. macOS provides its system drivers; software for an external adapter must match the model and macOS version. The Windows driver collection does not apply to a Mac. Startup inspection currently supports Windows only, so the Mac homepage offers system information instead.

If macOS cannot start, follow [Apple Recovery instructions](https://support.apple.com/en-us/102518). On Apple Silicon, shut down and hold the power button to reach Options; on Intel, hold Command-R at startup. Confirm backups and the target disk before following [Apple Disk Utility instructions](https://support.apple.com/en-us/102611). This app does not restart into Recovery, erase disks or reinstall the operating system.

## Before you unplug

The first AI launch copies the fixed programs to the current computer's cache. Later launches can reuse that cache, reducing large reads from the USB drive. Keys, the wallet, conversations, and work files stay on the USB drive. The initial copy time depends on the computer and drive.

The bundled PicoClaw version is pinned and does not automatically update. No local language model is included. Keep the drive secure: its API keys are sensitive credentials, and the drive should not be treated as an encrypted password vault.

Close any maintenance tools you opened, click **Quit** in the app, and safely eject the USB drive through your operating system. Closing only the browser tab does not stop the background AI service.
