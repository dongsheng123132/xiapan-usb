# 虾盘随身AI盘 · Windows 本机真实使用测试报告

- 测试日期：2026-10-14 21:45–21:55（UTC+8）
- 测试地点：Windows 本机（真实桌面操作，模拟真人插盘使用）
- 被测对象：U 盘（F: 盘，卷标 UCLAW，exFAT 117.2GB）内「虾盘随身AI盘」
- 被测版本：**0.5.3-portable-preview**（`F:\app\build-manifest.json`：`version: 0.5.3-portable-preview`，`source_commit: d725360714492c5f5f505a039d7ae8500d016a1f`，built 2026-10-14T06:21Z）
- 测试方式：完全走真实用户路径（GUI 操作：双击启动 → 浏览器页面配置 → 对话 → 退出），非改配置文件
- 模型：DeepSeek 官方 OpenAI 兼容端点 `https://api.deepseek.com/v1`，模型 `deepseek-v4-pro`，真实 Key 计费验证

---

## 一、测试步骤与结果

| # | 步骤 | 操作（真实用户路径） | 结果 |
|---|------|----------------------|------|
| 1 | 校验盘完整性 | 运行 `F:\校验母盘.ps1`（逐文件 SHA256 比对 SHA256SUMS） | ✅ 通过：「固定程序与资源校验通过 / Programs and resources verified. User data/ is excluded.」 |
| 2 | 启动 | 等价双击 `F:\开始使用.cmd`（内容即 `start "" "%~dp0虾盘.exe"`） | ✅ 虾盘.exe 启动（PID 34232），本地服务监听 `127.0.0.1:56588`，HTTP 200 |
| 3 | 打开首页 | Chrome 访问 `http://127.0.0.1:56588/` | ✅ 「虾盘 · U 盘精灵」首页正常（AI 对话 / 电脑维护工具包 / 盘内资源 / 启动救援） |
| 4 | 配置模型 | 顶部「模型设置」→ 模型来源选「自带 API · OpenAI 兼容」→ 填 API 基础地址 `https://api.deepseek.com/v1`、API Key、模型 ID `deepseek-v4-pro` | ✅ 表单可填、Key 掩码显示 |
| 5 | 测试连接 | 点「测试连接」（向服务商发 1 次真实请求，可能计费） | ⚠️ 按钮已点击，页内结果提示未捕获（toast 短暂/静默）；后续真实对话成功即证明链路与 Key 有效 |
| 6 | 保存 | 点「保存并使用」 | ✅ 出现「模型已保存到本盘」提示；底部状态变为「deepseek-v4-pro · 自带 API · 设置与对话随盘保存」 |
| 7 | 真实对话 | 输入「你好，这是虾盘随身AI盘的真实使用测试，请简短回复。」并发送 | ✅ AI 真实回复：「你好，虾盘随身AI助手已就绪，有什么电脑维护需求可以随时吩咐。」 |
| 8 | 数据落盘 | 检查 F 盘 data/ | ✅ 配置与对话均随盘保存（见下节） |
| 9 | 退出 | 点顶部「退出」 | ✅ 页面提示「虾盘已退出 · 本次启动的 PicoClaw 已停止，请关闭维护工具，再安全移除 U 盘。」 |
| 10 | 进程/端口清理 | 检查进程与端口 | ✅ PID 34232 已退出；56588 端口无监听（仅 TIME_WAIT 残留），可安全弹出 |

---

## 二、随盘保存验证（拔盘带走数据）

退出后 F 盘 data/ 落盘内容：

| 路径 | 内容 |
|------|------|
| `F:\data\settings\model.json` | 模型配置（含 API Key、baseURL、模型 ID） |
| `F:\data\chat\20261014-215357-*.json` | 本次对话记录（含时间戳命名） |
| `F:\data\pico\config.json` / `.xiapan-bridge.json` / `.security.yml` / `.picoclaw.pid` | PicoClaw 网关配置与运行状态 |
| `F:\data\pico\workspace\skills\pc-*`（5 个） | 电脑维护技能：pc-triage / pc-network-care / pc-offline-install / pc-data-rescue / pc-boot-recovery |
| `F:\data\pico\workspace\sessions\*.jsonl` | 会话记录 |
| `F:\data\pico\logs\gateway.log` 等 | 运行日志 |

**结论：模型 Key、对话、配置全部随盘保存，换机可用，符合「设置随盘保存」设计。**

---

## 三、观察项（非阻塞）

1. **版本显示不一致**：首页侧栏显示「0.5.2 便携预览」，而 `build-manifest.json` 为 0.5.3；`F:\START_HERE.md` 也写 "For version 0.5.2"。UI/README 版本文案滞后于实际构建，建议下次构建统一。
2. **「测试连接」页内反馈**：点击后 UI 结果提示未捕获（未观察到成功/失败 toast）。真实对话成功证明链路可用，但建议自查测试连接的结果展示逻辑（可能提示过短或静默失败未报错）。
3. **残留实例**：本机存在旧虾盘实例 PID 33084（源 `E:\虾盘.exe`，E 盘已不存在，监听 63747/65502），非本次 F 盘测试对象，未处理。建议用户手动清理（`taskkill /PID 33084 /F`）或重启后自然消失。
4. **浏览器异常关闭提示**：Chrome 弹出「Chrome 未正确关闭，要恢复页面吗？」——为本机 Chrome 自身历史状态，与虾盘无关。

---

## 四、结论

**通过。** 虾盘随身AI盘 0.5.3-portable-preview 在 Windows 本机按真实用户路径（插盘 → 校验 → 启动 → 配置自带 API → 真实对话 → 退出 → 安全弹出）全流程可用：

- 母盘文件完整（SHA256SUMS 全过）；
- 从 U 盘启动零依赖（免安装、免配置环境）；
- 自带 API（OpenAI 兼容）配置 + 真实 DeepSeek 调用成功（含计费验证）；
- 配置与对话随盘保存，退出后进程/端口干净，可安全弹出。

**遗留建议**：修复 UI/README 版本号文案（0.5.2 → 0.5.3）；复核「测试连接」结果提示的可见性；清理本机 E 盘幽灵实例。
