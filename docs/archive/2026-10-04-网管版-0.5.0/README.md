# 存档：0.5.0 网管版改道（2026-10-04）

这里保存的是 `main` 分支 2026-10-04 那次「公司网管工具箱」改道的说明文档，仅作历史归档，**不是当前产品范围**。

## 为什么存档

- 0.5.0 曾把产品收窄成「给 Windows 公司网管用的现场排障工具箱」，删掉钱包、充值、云模型、便携工具目录和启动救援。
- 之后 0.5.3 → 0.5.5 的实际交付线回到「随身 AI 优盘 + 电脑维护工具包」，也就是当前 `main` 的方向。
- 2026-10-31 收口 0.5.6 时，合并以 0.5.5 主线为准，网管版的代码和文档不再进主线，此目录保留文字说明以免后人翻历史时误读。

## 相关提交

| 提交 | 内容 |
| --- | --- |
| `615f2bb` | refactor: focus Xiapan on Windows company IT troubleshooting |
| `c85f2d2` | release: prepare v0.5.0-preview for company IT |
| `1c624c4` | Merge pull request #5 from codex/sysadmin-focus |

网管版新增的代码文件（`api.go`、`host.go`、`runtime.go`、`network_rescue.go`、`engine/network.ps1` 等）没有带进主线，需要时按上表提交号从 git 历史取回。

当前产品范围以 [docs/PRODUCT.md](../../PRODUCT.md) 为准。
