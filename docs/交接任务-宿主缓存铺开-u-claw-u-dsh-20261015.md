# 交接任务：把宿主缓存铺到 u-claw / u-dsh（对齐虾盘精灵模式）

> 交接对象：claude CLI（sonnet 或 haiku 5.5）
> 交接日期：2026-10-15
> 任务性质：代码改造 + 真机验收，产物必须可复现、可核验
> 目标产品：u-claw（U-Claw / OpenClaw 便携）、u-dsh（U-DSH Web 便携）

---

## 一、任务目标（为什么做）

用户对 u 系列产品的核心诉求：**U 盘 IO 性能差、大量读取慢、希望延长 U 盘寿命**。
设计原则只有一条，必须贯彻到所有产品线：

> **可再生成的东西（程序本体、依赖、缓存、临时/日志/索引文件）→ 放本地硬盘运行；不可再生成的东西（Key、钱包、会话、工作文件）→ 保留在 U 盘。**

现状差距：u-picoclaw（虾盘精灵）0.5.4 起已实现宿主缓存；**u-claw 和 u-dsh 在 0.5.5 仍是"运行时+依赖整体放 U 盘、每次启动从 U 盘读"**——这正是慢和伤盘的大头。本任务把宿主缓存机制铺到这两条线。

## 二、先读这些文件（必读，按顺序）

1. `C:\1new-code\虾盘-ai优盘精灵版u盘版本\docs\u系列多平台兼容开发经验-20261014.md`
   —— 总纲：exFAT 三条硬限制、平台能力对照表、三条产品线落地记录、检查清单、已踩坑清单。**必须遵守**。
2. `C:\1new-code\虾盘-ai优盘精灵版u盘版本\public-source\xiapan-usb\runtime_cache.go`
   —— **参考实现（抄这个模式）**：宿主缓存目录定位、SHA-256 校验、命中不读源、损坏重校验、不可用降级、缓存上限 256MB、`XIAPAN_RUNTIME_CACHE_DIR` 覆盖、`runtime.cache.status` 只读状态。
3. `C:\1new-code\虾盘-ai优盘精灵版u盘版本\public-source\xiapan-usb\docs\PRODUCT.md`
   —— 虾盘精灵的产品口径（缓存只装可再生成程序、状态数据留盘、版本冻结）。
4. `C:\1new-code\虾盘-ai优盘精灵版u盘版本\public-source\customer-udsh\docs\ROADMAP.md`
   —— U-DSH 的"不许推翻的既有决定"（junction 必须 NTFS、数据侧边界）与"别再踩的坑"。
5. `C:\1new-code\虾盘-ai优盘精灵版u盘版本\发盘\本次发货说明-0.5.5.md`
   —— 0.5.5 当前打包结构、u-claw/u-dsh 入盘方式、已修的 exFAT 问题（OpenClaw 原子写 path-mismatch、web-launch.cjs）。

u-claw 仓库：`github.com/dongsheng123132/u-claw`（便携版 `portable/`，含 Mac/Windows 启动脚本、config-server、skills-cn）。
u-dsh 仓库：`github.com/dongsheng123132/u-dsh-deepseek-harness-portable`（webui 便携内核）。

## 三、硬约束（不可违反）

1. **数据红线**：Key、钱包、会话、记忆、工作文件**只写盘内 data/**（u-claw：`app/portable-apps/u-claw/data`；u-dsh：`app/portable-apps/u-dsh/data/dsh-home`），**绝不进本机缓存**。不采用"退出时才同步"方案，防异常拔盘丢状态。
2. **缓存只装可再生成的已校验程序**：node 运行时、OpenClaw 内核、DSH 依赖等；每个条目锁定版本 + SHA-256（manifest），新版本由新 hash 隔离。
3. **降级路径**：缓存不可用（损坏/不可写/首次）时，验证盘上原程序并明确降级使用，**不能因为缓存坏了就启动失败**；损坏缓存重新校验准备。
4. **exFAT 兼容不回归**：u-claw 的 `patch-portable-filesystem.mjs`（零 inode 识别、`verify-content-with-lock`）、`.uclaw-runtime-platform` 标记、`runtime-requirements.mjs` 三平台标记**必须保留**；u-dsh 的 `web-launch.cjs`、`--no-open` 启动方式不变。缓存逻辑不得破坏已修好的 exFAT 启动路径。
5. **Mac 侧**：首次复制到缓存后补可执行权限（exFAT 无权限位）；跳过 `._*` / `.DS_Store`；可移除宗卷权限先请求、拒绝给「系统设置」指引；脚本不关 Gatekeeper。
6. **Windows 侧**：判断文件系统用 `Get-Volume`，不用 `Win32_LogicalDisk.FileSystem`（缓存过期值）。
7. **版本冻结**：运行时 freeze 版本 + manifest 锁 SHA-256，客户盘不跟踪 upstream nightly。
8. **不越界**：不改用户数据、不格式化、不改网络设置、不自动升级；临时/日志重定向只影响本应用自己的文件。

## 四、实施清单

### A. u-claw（OpenClaw 便携，约 1.4GB 包）

1. 移植宿主缓存机制：OpenClaw 内核 + node 运行时（win-x64/mac-arm64/mac-x64 三个平台的 node 本体）+ 依赖闭包 → 按平台 + SHA-256 复制到本机缓存（Win `%LOCALAPPDATA%`，Mac `~/Library/Caches`），首次复制校验、命中不读 U 盘、损坏重校验、不可用降级回盘上原包。
2. 临时/日志重定向：OpenClaw 的日志、临时目录、索引 → 本机缓存目录（不落 U 盘，减少频繁写磨损）；**配置、凭据、会话数据仍写盘内 `data/`**。
3. 保留 exFAT 兼容补丁（见硬约束 4），并补测：补丁后缓存的程序在 exFAT 盘上以"缓存命中"方式启动不回归 `path-mismatch`。
4. 启动脚本（`Windows-Start.bat` / `Mac-Start.command` / `setup.ps1` / `setup.sh`）接入缓存逻辑；Mac `.command` 保留 `bash` 替代入口说明。

### B. u-dsh（U-DSH Web 便携，约 0.9GB 包）

1. 移植宿主缓存机制：node 运行时（三平台）+ DSH 内核依赖 → 本机缓存 + SHA-256 锁定；数据仍走 `data/dsh-home`（junction 仍在 U 盘数据区——**这是既有决定，不许改**）。
2. 临时/日志重定向：DSH 运行时产生的日志、缓存、临时文件 → 本机缓存目录。
3. 保持 `web-launch.cjs`（CommonJS 显式）+ `--no-open` + token URL 打开浏览器的方式；启动脚本 `.bat` / `.command` 同步更新。
4. 注意 U-DSH 对 NTFS 的要求不变：junction 在数据区，exFAT 数据盘仍按既有探测逻辑给人话报错（`probeJunctionSupport` 保留）。

### C. 通用

- 两条线都提供只读状态查询（对齐虾盘精灵的 `runtime.cache.status`）：缓存目录、平台、命中/未命中、降级原因。
- 缓存上限设合理值（参考虾盘精灵 256MB，可按 OpenClaw/DSH 实际体积调整并写明理由）。
- 构建脚本把缓存逻辑纳入打包验证（对齐 `PORTABLE-VALIDATION.md` 的验收方式）。

## 五、验收标准（发版硬门槛，逐项记录）

1. **冷/热启动对比**：同一块真实 exFAT U 盘、同一台机，记录"首次（冷缓存）"与"二次（命中）"的启动到服务就绪时间；命中后**U 盘不再读取程序本体**（可用资源监视/IO 计数验证，写明测量方法）。
2. **U 盘读写量对比**：改造前后各测一轮（如 3 分钟典型操作），给出 U 盘读/写字节数变化；目标：程序本体读归零、临时/日志写归零，只剩 data/ 用户数据写。
3. **拔盘数据仍在**：配置、会话、Key 在缓存命中模式下仍随盘；拔盘前退出 → 换电脑 → 数据完整可用（如条件不允许第二台机，至少验证拔插重分配盘符）。
4. **降级路径**：手动删除/损坏本机缓存 → 程序仍能从 U 盘原包启动并给出明确提示。
5. **exFAT 回归**：u-claw 启动 `/ready` 200；u-dsh Web 内核就绪并自动开浏览器；0.5.5 修的两个问题（path-mismatch、ESM 误判）不复发。
6. **多平台**：Windows x64 + Mac Apple Silicon 实机；Intel Mac 若条件不具备，如实标注"构建+界面验证，待实机"，不宣称支持。
7. **数据红线自查**：grep 确认缓存目录里没有 Key/钱包/会话/工作文件残留。

## 六、产出要求

- 代码提交到对应仓库（u-claw、u-dsh），提交信息说明改动与理由；**不带 Co-Authored-By 之外的虚构署名**。
- 每个结论可溯源：验收数据写进仓库 docs/（或发盘验收记录），注明日期、机器、盘、版本、SHA-256。
- 更新 `C:\1new-code\虾盘-ai优盘精灵版u盘版本\docs\u系列多平台兼容开发经验-20261014.md`：把两条线的 IO 设计从"整体在 U 盘跑"更新为"宿主缓存 + 数据留盘"，并记录本轮新踩的坑（如果有）。
- 完成后交付一份简短验收报告：改了什么、验证了什么、边界是什么、下一步建议。

## 七、禁止

- 禁止改动 u-picoclaw（虾盘精灵）已验收的缓存逻辑（除非发现必须修的问题，先说明再动）。
- 禁止把数据/凭据写入任何本机目录；禁止格式化任何盘；禁止修改用户现有 U 盘内容。
- 禁止宣称"所有 I/O 已脱离 U 盘"——口径只能是"程序本体与临时文件走本机缓存，用户数据仍在盘内"。
- 禁止跳过程序化验收直接交付。
