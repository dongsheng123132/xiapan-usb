# usb.u-claw.org.cn → 虾盘 lite 0.5.8 更新备料

日期：2026-11-02 · 来源版本：`v0.5.8-preview`（GitHub `dongsheng123132/xiapan-usb`，发布 2026-11-01 07:04 UTC）

## 1. 现状（实测）

| 项 | 更新前线上 | 本次目标 |
| --- | --- | --- |
| 下载包 | `/download/xiapan-usb-0.4.4-preview-windows-x64.zip`（16,898,592 B，last-modified 2026-10-03） | `0.5.8-preview` Windows x64 zip，29,876,031 B |
| 首页下载卡 | `Windows x64 · 0.4.4-preview` | `Windows x64 · 0.5.8-preview` |
| SHA-256 展示 | `ec476844…395c7c21` | `e00b8816…cf588ab5` |
| 统计条 / TINY 卡 / TOOLBOX 卡 / meta / og:description | 16.9 MB、主程序 8.4 MB | 29.9 MB、主程序 13.2 MB |
| engineering.html | 「网站的 16.9 MB 是旧 Windows 体验包的下载大小」 | 「网站的 29.9 MB 是 Windows 体验包的下载大小」 |
| GitHub 链接 | `tree|blob/codex/u-picoclaw-portable`（旧分支） | `tree|blob/main` |

站点没有 `version.json`/`sitemap.xml`/`robots.txt`（均 404），所以版本只能从页面文字判断。

## 2. 物料

| 文件 | 大小 | SHA-256 |
| --- | --- | --- |
| `xiapan-usb-0.5.8-preview-windows-x64.zip` | 29,876,031 B（29.9 MB） | `e00b881635b041204cd1690f0961079012ab67895b99a706482f63eacf588ab5` |
| `site-index.html` | 39,101 B（CRLF 277 行） | — |
| `site-engineering.html` | 20,212 B（CRLF 62 行） | — |

zip 来源：GitHub Release 资产 `xiapan-u-claw-lite-0.5.8-windows-x64.zip`，**只改名，未重新打包**（内容逐字节一致）。
zip 根目录 `虾盘-u-claw-lite-0.5.8-windows-x64/`，含 `虾盘.exe`（13,210,624 B）、`Start Windows.cmd`、`data/`（空）、`app/runtime/picoclaw/`。

页面改动共 12 处（`diff orig-index.html site-index.html` 只有 10 行、engineering 2 行变化；行尾 CRLF 原样保留，未做任何重排）。

## 3. 发布方式（本机没有这台机器的发布权限）

`usb.u-claw.org.cn` 与 `u-claw.org.cn` 同解析到 **203.195.204.252**（nginx/1.24.0 Ubuntu）。
本机 `~/.ssh` 只有广州只读库账号（`command=` 锁死只读 sqlite3）；`root/claude/ubuntu@203.195.204.252` 用现有 key 均 `Permission denied`。

给到发布通道后执行：

```bash
cd <本目录>
SSH_TARGET=root@203.195.204.252 bash deploy-usb-site.sh --dry-run   # 先跑预演
SSH_TARGET=root@203.195.204.252 bash deploy-usb-site.sh             # 正式发布
```

脚本不猜路径：从 `nginx -T` 里读 `server_name usb.u-claw.org.cn` 的 `root`，再定位放着 0.4.4 旧 zip 的 `download/` 目录；两步任一失败即中止、不写任何文件。
发布步骤：备份 → 上传 → `install`+`mv` 原子替换（旧 zip 全部保留）→ `nginx -t && systemctl reload nginx` → 线上验收（页面含 0.5.8、SHA-256 一致、zip 的 Content-Length 与本地一致）。
回滚：把 `/var/www/_backup-usb-<ts>/` 的两个页面覆盖回站点根，`nginx -t && reload`。

## 4. 未决事项

1. **macOS 下载卡**：Release 里另有 `xiapan-u-claw-lite-0.5.8-macos-arm64.zip`（27.4 MB）。本次页面**保持不变**——现有文案写「需 Developer ID 签名公证，暂请从源码构建」，在没有做 `codesign`/`spctl` 实测前不改口，也不上未公证的包。
2. **站点源码不在任何可达仓库**：`u-claw.org` / `u-claw.org.cn` 两个私有仓都没有这个 usb 页面，GitHub 代码搜索 `xiapan-demo.mp4`、`0.4.4-preview-windows-x64` 均 0 命中 → 以线上页面为基线改（本目录 `orig-*.html` 是发布前的线上原件）。
3. **engineering.html 的 0.5.2 实测数据**：那是 2026-10-14 的真实记录，未改数字（没有 0.5.8 的同条件实测就不编新数）。
