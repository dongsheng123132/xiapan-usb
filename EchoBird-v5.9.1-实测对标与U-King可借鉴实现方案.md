# EchoBird v5.9.1 实测对标与 U-King 可借鉴实现方案

> 日期：2026-10-15 · 作者：Doubao Work 编排（U-King 项目）
> 依据：Windows 本机 EchoBird 5.9.1 全量 UI 实测（UI Automation 逐页 dump，8 页全部拿到）+ U-King 仓库（`C:\Users\ZhuanZ\Desktop\claude\u-claw\u-king`）源码核对
> 前置结论：EchoBird v5+ 为 BUSL-1.1 许可（源码公开、商业再分发需授权，4 年后转 GPL-2.0），借鉴其**交互与产品形态**无碍，直接搬运代码需注意许可边界。

---

## 一、EchoBird v5.9.1 实测：8 个页面真实结构

本机实测（进程 echobird.exe，FileVersion 5.9.1，窗口 1500×1000），左侧 8 项导航，每页结构如下：

| 页面 | 实测内容（UIA 全量转储） |
|---|---|
| **应用桌面** | 顶部「我的 / 商店」tab + 自定义扫描路径 + 刷新；21 张工具卡（Claude 桌面端、ChatGPT、Cline 桌面端、DeepSeek Harness、Hermes 桌面端、WorkBuddy、WorkBuddy AI、Claude Code、Cline CLI、Codex CLI、Grok Build、Kimi CLI、MiMo CLI、Oh My Pi、OpenClaw、OpenCode、Pi、QwenCode、黑白棋、AI 翻译、Clash Verge）；右侧「当前选择 / 启动应用」，启动前可选「直接启动应用」或「修改模型配置」 |
| **模型管理（Model Nexus）** | 「配置 / 用量 / 测试速度」三 tab；模型卡**可拖拽排序**（实测 3 卡：DeepSeek 延迟 124ms、OpenCode Zen 472ms、bai 未测试），每卡展示模型名 / 来源域名 / 延迟，并打 **OpenAI / Anthropic 双协议标签**；「添加模型」支持自定义 OpenAI / Anthropic API；右侧「优质中转站」（CC Vibe、APIMart、88API、GrooRoute Pro、Sidrune AI）+「模型供应商」目录 **32 家**（小米、DeepSeek、MiniMax、智谱、Kimi、美团 LongCat、百度千帆、千问、混元、阶跃、火山、OpenAI、Gemini、BytePlus、Anthropic、xAI、Meta、Perplexity、Mistral、Cohere、Groq、Together、NVIDIA、OpenRouter…），每家「Add model + 打开官网」 |
| **账号管理** | 「一键领取奖励」「刷新额度」；**13 家服务商统一管理**（ChatGPT/Codex、Claude Code、Claude Desktop、DeepSeek Harness、WorkBuddy、WorkBuddy AI、Grok Build、Antigravity、Cursor、ZCode、Grok Bot、Manus、Cue），每家「添加账号 + 打开官网」双按钮，未安装的服务商标「· 未安装」灰显 |
| **智能路由** | 「免切换智能路由」总开关（本机未启用）；**OpenAI / Anthropic 两个可复制端点**（给 Claude Code 等外部工具直接指过来）；「当前已接入 N 个模型」；「我的模型 / 免费模型」tab；每个模型「添加模型到智能路由 + 刷新用量 / 删除 / 编辑」；自定义添加 |
| **AI 助手** | chat 输入框 + 模型选择（DeepSeek）+ 上下文用量 0KB/293KB；右侧**快捷命令组 9 个**：安装 OpenClaw / 安装 Claude Code / 安装 Hermes 桌面端 / 帮我安装 Git / 查看本机硬件配置 / OpenClaw 不好用了帮我修复 / 彻底卸载 OpenClaw / 查看内网公网 IP / 检测可疑活动 / 帮我安装 ChatGPT 汉化插件 |
| **我的AI生涯** | 统计看板：总会话数 1.2K、总消息数 325.6K、约累计 Token 22.7B、活跃天数 70、最长连续天数 35；**模型家族分布**（Claude / Codex / OpenCode / DeepSeek / Hermes / MiMo 家族卡片）；右侧**最近会话时间线**（标题 + 消息数 + 「复制路径」） |
| **我的AI项目** | 自定义项目（实测：黑白棋、AI 翻译），每项显示「模型 / 应用 / 配置」三字段（应用指向 `~/.echobird/reversi/game.html`，配置指向 `models.json`）；「添加我的AI项目（需支持文件 models.json）」；数据目录 `~/.echobird` |
| **本地大模型** | 「选择模型 → 配置 → 启动」三步；算力「⚡ GPU 自动」、上下文 32K、端口 11434、运行时 llama.cpp；「安装引擎 / 启动」+ 标准输出区；**入门 / 推荐 / 旗舰三档硬件卡**（入门：RTX 4090 24GB / Apple M4 Pro 48GB 配 Qwen3.5 9B 等；推荐：RTX 5090 / M4 Max；旗舰：H100 / H200 / Mac Studio M3 Ultra 192GB+）；「检测 CUDA 模块状态」「帮我安装 CUDA 模块」；「本地 / 商店」模型 tab + 模型目录 |

另：顶栏固定「更新日志 / 推荐应用 / 设置 / 最小化到托盘」。

---

## 二、U-King 现状对照（源码核对）

| EchoBird 能力 | U-King 现状 | 差距 |
|---|---|---|
| 应用桌面（工具卡 + 启动） | ✅ `toolhub/ToolHub.tsx`「我的 AI」页（工具卡片网格、换模型、一键启动） | 基本对齐，U-King 工具数更多（39+） |
| AI 助手（chat 驱动安装/修复） | ✅ `opencodex/` 工作台（Chat/Composer/会话）+ `AgentRepair.tsx` + `airuntime/doctorPrompt.ts` | 能力有，缺「快捷命令组」形态 |
| 本地大模型 | ✅ `LocalLLM.tsx` + `LocalLLMStore.tsx` | 有基础；缺硬件档位推荐卡、CUDA 检测引导按钮 |
| 模型管理（可拖拽 + 测速 + 双协议 + 32 供应商目录） | ◐ `ProviderManager.tsx` / `CustomProviderModal.tsx` / `providerTemplates.ts`（供应商模板已入 manifest，32+） | 供应商目录有；缺模型卡拖拽排序、延迟测速、双协议标签可视化 |
| 智能路由（免切换 + 端点复制 + 限额/故障转移） | ◐ 已有「DeepSeek 省钱路由」（`CodexZone.tsx` / `Manager.tsx` 后端自动开），`yieldChain.ts` / `freeGuide.ts` | 只有单一路由开关，缺通用可视化路由页、端点复制、额度耗尽故障转移 |
| 账号管理（13 服务商统一 + 领取奖励/刷新额度 + 未安装标灰） | ◐ `AI 设置`（manage）有换模型/充值/用量（`recharge.ts`），各工具卡有驱动/账号入口 | 缺统一服务商清单页、「未安装」灰显、一键领取奖励 |
| 账号切换备份回滚 | ◐ 已有 `ensureDriver.ts`、`active-drivers.json`、dsh-driver-backup.json 等切换/备份机制 | 缺「切换前快照 + 失败回滚」的完整闭环和 UI 呈现 |
| 我的AI生涯（统计看板 + 家族分布 + 时间线） | ✗ 无 | 全新页面；需要会话埋点/统计（可基于 `opencodex/session.ts` / store） |
| 我的AI项目（models.json 自定义应用启动器） | ✗ 无（`UsbToolDisk.tsx` 是 U 盘工具盘，形态不同） | 全新能力 |

---

## 三、可借鉴项实现方案（按优先级）

### P0-A 账号切换备份 + 失败回滚（对齐 EchoBird v5.8.8 同款机制）
- **目标**：任何「切换模型 / 切换账号 / 切换驱动」动作前自动快照（现有 `dsh-driver-backup.json` / `claude-env-provenance.json` 的机制泛化），动作失败自动回滚并提示。
- **落点**：`src/lib/ensureDriver.ts`（切驱动前快照）→ 新增 `snapshotBeforeSwitch()`；`opencodex/session.ts`（会话状态导出）；`Manager.tsx` / `ToolHub.tsx` 的切换入口接回滚钩子。
- **要点**：快照 = 环境文件（providers.json / active-drivers.json / 各 CLI 的 auth 配置）+ 当前会话的最近 N 条上下文；回滚 = 先恢复文件，再重启对应驱动进程；日志记 journal 可查。
- **验收**：手动切换驱动→制造失败（如断网）→ 自动恢复原账号并弹提示；反复 10 次无脏状态残留。
- **工作量**：约 0.5–1 人日（机制已有，主要是泛化 + UI 提示）。

### P0-B 智能路由页（对齐 EchoBird Smart Router）
- **目标**：把「DeepSeek 省钱路由」泛化成通用路由页：多模型拖拽排优先级、限额/额度耗尽自动故障转移到下一顺位、冷却重试；提供 OpenAI / Anthropic 两个可复制端点供外部 CLI（Claude Code / Codex / Cline）直连。
- **落点**：新页面（侧栏「更多」组，或并入 `AI 设置` 子 tab）；后端复用 `yieldChain.ts` / `freeGuide.ts` / 虾盘云路由逻辑；端点由 `dshdesk.rs` / `tools.rs` 的本地代理端口（如 3080）暴露。
- **要点**：路由策略三要素——优先级列表（前端拖拽序）、故障转移条件（HTTP 429 / 401 / 超时 / 余额不足）、冷却窗口；端点复制用一键复制（`lib/clipboard.ts` 已有）。
- **验收**：配 2 个模型（贵 + 便宜），把便宜模型设为高优；用工具连续调用触发贵模型额度耗尽 → 自动切便宜模型，日志可追溯；端点复制到 Claude Code 配置可直接用。
- **工作量**：约 1.5–2 人日（路由逻辑已有，主要做 UI 与端点暴露）。**路由策略（故障转移条件与冷却参数）建议用 claude CLI（opus）复核一次再定稿。**

### P0-C 统一账号管理页（对齐 EchoBird 账号管理）
- **目标**：一屏列出全部服务商（U-King 工具对应的 20+ 家），每家「添加账号 + 打开官网」，未安装的标灰；顶栏「一键领取奖励 / 刷新额度」（对接虾盘云 `recharge.ts` 与免费额度体系）。
- **落点**：`AI 设置`（manage）增加「账号」子 tab，或独立新页；数据源 = 现有 `providers.rs` 检测结果 + `providerTemplates.ts`。
- **要点**：服务商 = 工具 → 官网 / 添加账号入口的映射表（manifest 已有 provider_templates，可加 `account_page` 字段）；未安装判定复用 `toolbox.rs` 的 probe。
- **验收**：新装机用户一屏看清「哪些能领、哪些没装、去哪配」；奖励/额度按钮点击后余额实时刷新。
- **工作量**：约 1 人日（数据全有，纯 UI 聚合）。

### P1-A 模型管理页升级（对齐 Model Nexus）
- **目标**：模型卡可拖拽排序、一键测速（延迟 ms）、OpenAI / Anthropic 双协议标签；右侧供应商目录（32 家）一键「添加模型」。
- **落点**：`AI 设置` 或独立「模型」子 tab；测速后端 = 对已配模型发一次最小请求计时（复用 headless 通道）；双协议标签 = 模型配置里已有协议字段的展示。
- **验收**：3 个模型拖拽后顺序持久化（重启仍在）；测速 5 次取中位数；每个供应商「添加模型」能预填官方 API 域名。
- **工作量**：约 1–1.5 人日。

### P1-B 本地大模型页补齐（对齐 EchoBird Local LLM 硬件档位）
- **目标**：在现有 `LocalLLM.tsx` 上加「入门 / 推荐 / 旗舰」硬件推荐卡（按检测到的 GPU/内存自动推荐模型）、「检测 CUDA 模块状态 / 帮我安装 CUDA 模块」按钮（Windows 端，对接 nvidia-smi 与驱动修复通道）。
- **验收**：本机检测到 RTX 4090 → 推荐档位高亮；CUDA 缺失 → 引导安装并可跳转官方驱动页。
- **工作量**：约 1 人日。

### P1-C 我的AI生涯（新页面，对齐 EchoBird 生涯看板）
- **目标**：总会话数 / 总消息数 / 累计 Token / 活跃天数 / 最长连续天数 + 模型家族分布 + 最近会话时间线（复制路径）。
- **落点**：新页（「更多」组）；统计埋点 = `opencodex/store.tsx` 会话记录 + `MetricsPanel.tsx`（已有用量采集）+ `lib/useXiapanModels.ts`。
- **注意**：这是数据页，需要先确定统计口径（本地会话 vs 云端、跨工具合并）；建议 P1 先出「本地会话统计」最小版，云端合并 P2。
- **工作量**：最小版约 1 人日；完整版 2 人日。

### P2（可选，低优先）
- **我的AI项目**（models.json 驱动的自定义应用启动器）：U-King 若做 U 盘分发场景，可参考；对 U-King 现有用户价值中等。
- **AI 助手快捷命令组**：把「安装/修复/查硬件/查 IP/查可疑活动」做成可点命令卡片（复用 `AgentRepair` / `doctorPrompt` / 体检通道），形态借鉴，成本低，可提前到 P1。
- **免费模型目录**（EchoBird「免费模型」tab）：U-King 已有虾盘云免费额度体系，可借鉴「免费模型」tab 呈现，优先级低。

### 明确不做
- EchoBird 交互动画/皮肤级模仿（低收益、高成本，且许可是 UI 层面最接近代码的部分）。
- 直接搬运 EchoBird 代码（BUSL-1.1 再分发需授权；仅借鉴交互与信息架构）。

---

## 四、已在本轮一并完成的事项

1. **manifest v62（install-windows.json）**：DSH 插件安装/修复共 4 条 `dsh plugin add` 命令补 `-w` 参数（pnpm 10 在 profile workspace root 直接 add 报 `ERR_PNPM_ADDING_TO_ROOT`，`-w` 为 pnpm 官方推荐解法），已在 macOS 0.2.0-rc.2 实测幂等通过；`version` 字段 61→62（上一轮漏 bump，线上热下发会因 version 不大于线上而拒绝覆盖，已修正）；notes 新增 v63。⚠️ 需重新部署到线上（`installer.rs` 热拉地址：u-king.org / u-claw.org.cn / cloud.u-claw.org 三处 install-windows.json）。
2. **EchoBird 实测证据链**：`C:\Users\ZhuanZ\Desktop\claude\u-claw\eb-page-*.txt`（8 页 UIA dump，其中 6 页已读入本报告）+ 可复用脚本 `eb-nav2.ps1`（每次点击前实时取坐标，规避坐标错位）。

## 五、建议执行顺序

1. P0-A 账号切换回滚（收益最快，机制已有）→ 2. P0-C 统一账号页（纯 UI，数据全有）→ 3. P0-B 智能路由页（路由策略定稿前用 claude CLI 复核）→ 4. P1-A 模型管理 → 5. P1-B 本地大模型补齐 → 6. P1-C 生涯统计最小版。
- 按用户调度偏好：**P0 各页的批量文案/UI 骨架可派便宜模型（pi→DeepSeek/b.ai flash）执行，路由策略与账号切换回滚的边界判断走 claude CLI（opus）复核，豆包工作台总编排与验收。**
- 每完成一项先给用户评审再动下一项；本方案不代为实现，待用户拍板优先级。
