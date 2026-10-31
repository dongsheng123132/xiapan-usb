# Windows 真机冒烟记录 · 2026-10-31

设备：zjzhfs，remote-agent pc-1517。源码对应本地 HEAD 9d3cbd5。

## 已验证

- 221,334 字节源码包传入 Windows，原机 Go 构建成功。
- 原机 `go test ./...`：`ok xiapan-maintenance 3.997s`。
- 从 `X:\xiapan-056-test\xiapan-smoke.exe` 启动独立 HTTP 服务。
- `system.inspect` 读取到真实 Windows/amd64、20 逻辑处理器、内存及盘卷。
- `workspace.create` 在 `D:\xiapan-smoke-20261031-215642` 建项目，并创建关联对话。
- 浏览 D 盘目录、列出项目文件成功。
- HTTP multipart 上传落盘成功，首次 upload-test.txt，第二次 upload-test-1.txt，均27字节，未覆盖。
- 界面设置切至 en，读回持久化值 en。
- 测试结束停止了本次启动的 HTTP 服务。

## 物理介质核对（重要纠正）

X 盘虽卷标为“测试2”，Y盘虽卷标为“openclaw”，但 Win32_DiskDriveToDiskPartition / Win32_LogicalDiskToPartition 关联证明二者位于 Disk #0：SAMSUNG MZAL4512HBLU-00BL2 内部SSD。它们不是当前插入的实体USB盘。

实际 USB 为 Disk #2，VendorC ProductCode USB Device，容量约62,914,560,000字节。其两个分区均为 GPT 类型 `{0fc63daf-8483-4772-8e79-3d69d8477de4}`（Linux filesystem），没有 Windows 盘符或 AccessPaths。

**因此本次是 Windows 真机跨盘功能验收，不是实体 U 盘验收。** 未修改、格式化或重新分区实际 USB。

## 22:01 补验：Windows 实体 exFAT U 盘通过

用户新增 USB Disk #3（VendorC ProductCode，约63GB），映射为 E:，文件系统 exFAT。已通过磁盘关联核实 E: 是 USB，不是内部SSD。

- 新版程序复制到 `E:\xiapan-056-test\xiapan-smoke.exe`，从 E: 启动服务。
- 在 `D:\xiapan-smoke-20261031-220106` 建项目及关联会话。
- 系统检测、浏览目录、上传落盘、同名不覆盖（upload-test.txt / upload-test-1.txt）、列文件、语言 en 持久化全部成功，原始报告 `ok:true`。
- 测试服务已停止，未覆盖原盘内容或格式化。
- 首次调用脚本外层使用 `Select-Object -First 1` 导致退出码异常；重新读取落盘报告确认各项已完成且 `ok:true`。不把该管道退出码当作功能失败。

原始报告：E:\xiapan-056-test\smoke-20261031-220106.json；Mac副本 `.cache/windows-exfat-smoke.json`。

## 22:02 补验：Mac 实体 exFAT U 盘通过

`diskutil info` 确认 /Volumes/UCLAW 为外接 USB disk9s1，实际文件系统 ExFAT、可读写（分区类型标识 Windows_NTFS 不代表其实际文件系统）。

- 从 /Volumes/UCLAW/xiapan-056-test/xiapan-smoke 直接运行，根路径确认为该实体U盘。
- 项目在本机代码目录的 .cache/mac-usb-project-220230；建项目、关联会话成功。
- 上传到本机项目并读回内容成功，同名自动变为 usb-test-1.txt，未覆盖。
- 语言 en 保存并读回成功。服务已停止。
- 证据 `.cache/mac-exfat-smoke.json`，未读取或迁移盘内旧钱包、配置备份和历史资料。

## 尚未覆盖

- Windows 上真实云模型/图片识别本轮未测试；Mac上的真实钱包模型调用为独立证据，不替代Windows链路。
- 未进行同一块实体盘在Windows与Mac之间拔插接续、完整母盘打包与全部第三方工具验收。
- 本次上传成功仅证明文件保存，不能等同于模型已具备图片识别能力。

原始报告位于 Windows `X:\xiapan-056-test\smoke-20261031-215642.json`；Mac副本在本项目 `.cache/windows-smoke-20261031.json`（含本机目录信息，不上传公开仓）。
