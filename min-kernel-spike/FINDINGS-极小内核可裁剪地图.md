# DSH 极小内核可裁剪地图（USB 版）

> 2026-11-01 侦察 + 实测结论。目标：回答「PicoClaw 是否该换成 DSH 极小内核」「能砍多少」。
> 本文件是 staging，正式版应落到 u-dsh 仓的 min-kernel 分支。

## 0. 一句话

**DSH 不是「一个内核」，是 cordis 插件框架 + bundle 拼装。砍功能 = 删 bundle / 删依赖，不动源码。**
虾盘只需要 headless 的 `dsh-base`（自己带 UI），实测可从完整 545MB/2.78万文件压到 **233MB/2.17万文件**。

## 1. 架构真相

- `dsh` 命令 = 薄启动器（`@deepseek-ai/dsh/lib/bin.js`，~12KB），只选 profile、解析参数。
- profile（`~/.dsh/profiles/<name>/package.json`）用 `dsh.profile.bundles` 声明要哪些 bundle。
- 每个 bundle = 一个 npm 包，带 `dsh.bundle.patch`（一份 `cordis.patch.yml`），往插件树里 `insert` 一串插件。
- `@deepseek-ai/dsh` CLI 的 `dependencies`（~70 个包）只是「随包出厂清单」，很多 bundle 根本不会被加载。

## 2. bundle 全景

| bundle | 作用 | 备注 |
|---|---|---|
| `@deepseek-ai/dsh-base` | 完整 agent 核：agent loop + 全部工具(fs/bash/web/subagent/workflow/goal/skill/todo) + 沙箱 + 会话 + 技能 + 插件管理 | 虾盘要的底子 |
| `@deepseek-ai/dsh-web-app` | Web UI：client-ui-* 全家桶、终端、webserver、frontend-static | 虾盘自带 Go UI，**不需要** |
| `@deepseek-ai/dsh-sdk-minimal` | SDK 最小 agent | **坑**：拖 office→LibreOffice 147MB，比 base 还大 |
| `@deepseek-ai/dsh-headless` | 极薄 headless 壳 | 只有 brand/cmdline/util/schemastery |
| 15 个 `experimental-*` | 语音/自动评审/ralph/徽章/插件市场/会话标题/终端 bundle/worktree/检查器/agent team | 全可砍 |

## 3. 实测体积（2026-11-01 本机 npm 实装，Mac darwin）

| 组合 | 体积 | 文件数 |
|---|---|---|
| 完整 `@deepseek-ai/dsh` 全量闭包（-ignore-scripts，纯 JS 侧） | 545 MB | 27,848 |
| 全局安装（含全部 bundle + darwin 原生二进制） | 841 MB | 63,688 |
| web profile（base+web-app）估算 | ~453 MB | ~41,507 |
| **剪枝到 dsh-base + 启动器闭包** | **233 MB** | **21,700** |
| 复制速度（APFS SSD，2.17 万文件） | — | 10.9 秒 |

大原生二进制几乎都在 base 之外（可砍）：
- LibreOffice 147.7MB（office 技能）、语音 ONNX 32.5MB、sharp/@img 27.8MB、OpenTelemetry 24.7MB、node-pty 25.6MB、openai 27.4MB、@anthropic-ai 13.7MB、@aws-sdk 14.5MB、@google 10.8MB、MCP 8MB。

**关键：DSH 自己的代码只有 ~18.8MB。233MB 里 90% 是第三方 SDK 和原生库，大多可砍。**

## 4. 对「换 PicoClaw」的判断

- 虾盘差异化 = UI + 钱包 + 维护工具 + 缓存 + 断网应急，**不是引擎**。
- 虾盘自带 Go UI/网关，**不应打包 dsh-web-app**；只要 `dsh-base` 的 headless agent。
- 差距是「打包哪个 bundle 子集」：headless 233MB vs web 453MB，砍近一半；文件数 2.17 万 vs 4.15 万，解压时间约减半。
- 再砍 openai/anthropic/google/aws SDK、otel、sharp、node-pty，可望压到 ~100MB 以内。

## 5. 已证实的点

- 最小 profile（只 `dsh-base`）结构有效：`dsh --profile web --dump-config` 输出 360 行插件配置 ✅
- 最小 profile 能启动：用 stock CLI 跑 45s 存活不崩 ✅
- 体积/文件数已实测（上表）✅

## 6. 待办 / 坑

- **自包含剪枝树的 peer 依赖**：手工剪 `node_modules` 会漏掉 `@deepseek-ai/cordis` 等 peer（--legacy-peer-deps 不装 peer）。正式做法必须走 u-dsh 已有的 `prepare-vendor.mjs` 那套「多轮 peer 收敛」，不要手剪。
- **profile 里几万行模型目录是配置**（`~/.dsh` 数据区），不进 vendor 闭包；但默认模型表很大，极小化时要同步砍一份精简表。
- **exFAT / 真 U 盘实测本机做不了**：沙箱禁 `hdiutil`（建不了磁盘镜像）、禁 `ps`。要在 Windows 机（pc-3116，remote-agent）或插真实 U 盘做。
- **DSH 内核 CLI 参数跨版本会移除**（skill 已记）：升级后必须实测参数集，别照抄旧参数。

## 7. 下一步（正式版）

1. 在 u-dsh 仓开 `min-kernel` 分支，写一个 `prepare-vendor` 变体：只 vendor `dsh-base` + 启动器闭包 + peer 收敛。
2. 在 Windows 真机 + exFAT U 盘上跑：量解压/启动/首答。
3. 结论：能否替 PicoClaw。
