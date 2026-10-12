# 离线维护盘交付

此分支基于 `5e20bea`，保留 PicoClaw、离线模型、便携工具和网络诊断，独立于已精简的公司网管版本。

发盘目录必须由 `scripts/package-offline.py` 从源码及已校验的资源生成，不能复制用过的发行目录。客户使用后产生的 `data/` 不得回灌母盘。

本地模型为通用 Qwen3.5-2B Q4_K_M，配合维护知识使用，未进行电脑维修专项微调。无需账户、API Key 或互联网。工具适用范围为可以进入 Windows 的 x64 电脑；复制目录不会生成 BIOS/UEFI 启动盘。

## 生成与验收

构建环境需 Go、Python 3.11 或更高版本、Git。`resources` 指已有可信模型、llama.cpp、PicoClaw 和六款便携工具的资源根目录；打包器逐项校验固定 SHA-256。新增硬盘工具由 `scripts/prepare-customer-extras.py` 生成，包含对应许可证、源码和依赖源码。

```powershell
python scripts/package-offline.py --resources '资源根目录' --extras '工具准备目录/ready' --output '全新输出目录'
python scripts/verify-offline-package.py --master '全新输出目录' --qa-smoke
```

可选大龙虾必须明确传入 `--large-agent '干净原始自解压包.exe' --large-agent-sha256 '已审查原包的SHA256'`。客户母盘不允许 `--allow-dirty`；该参数仅用于内部候选。打包器从当前源码构建、检查离线模式与内置工具目录，写入源码快照、提交号、资源清单及校验脚本，不读取开发机的账号配置。

验收脚本仅运行临时副本：隔离用户目录、移除继承的提供商凭据、代理设为不可用地址但允许 localhost，校验模型加载、实际推理、工具就绪状态、中文空格目录迁移、进程退出及母盘未被污染。它没有禁用本机网卡，不能替代真正断网或另一台客户电脑测试。三种实际问答的内容须人工审读，功能通过不等于模型事实准确。

实际测试发现 2B 模型会编造故障原因，不能按维修专家交付。离线客户模式将磁盘异响、文件误删、组织设置错误及有线断网等已识别场景先交给 `catalog/offline-references.json` 的固定知识指引，回答明确标记“未调用模型”，并且不先扫描电脑。其余问题仍由本地模型回答，仅作参考。这是常见场景分流，不能保证识别所有描述或消除所有模型错误。`--qa-smoke` 分开验证三条固定指引及一条真实模型术语问答，并核对各轮实际生成次数，不能用固定答案冒充模型验收。

客户双击 `开始维护.cmd`。对话、报告和工具配置会写入客户副本，母盘 `data/` 应一直为空。交付目录内的 `校验母盘.ps1` 同时检测文件缺失、变化及额外文件；校验母盘时不要把自己的文件混放在里面。

## 本版内容边界

- 默认离线链路是 Qwen3.5-2B + llama.cpp CPU + 维护知识。PicoClaw 0.3.1 已附带，但未声称它已与小模型完成离线工具调用联调。
- 六款原有工具加 CrystalDiskInfo、PhotoRec 文字界面、TestDisk，共九个便携工具入口；另有八个 Windows 系统工具入口。PhotoRec/TestDisk 共用同一个 CLI 目录，不包含 Qt 图形版本。
- `drivers/` 为空，供后续按客户硬件 ID 放入正确的签名驱动。未预装通用驱动包、Windows 安装镜像或启动环境。
- U-DSH 可选原包需解压至本机 NTFS 新目录，云功能仍依赖网络与用户提供商配置。
- 实际验收结果保存在 `docs/offline-customer/验收记录.txt`，详尽运行证据在开发工作树 `.cache/verify-offline-*`，不会将测试对话带入客户母盘。
