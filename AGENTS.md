# 本仓库工作约定

先阅读 [产品边界与便携架构](docs/PRODUCT.md) 和 [开发指南](docs/DEVELOPMENT.md)。产品范围以 `docs/PRODUCT.md` 为唯一说明；其中的迁移目标不代表已实现。

- 本仓库专注 U-PicoClaw 便携化与传统维护应急。新增功能先检查是否属于已确定范围。
- PicoClaw 保持当前冻结基线；客户盘不自升级。不得因出现上游 main/nightly 或更新按钮就擅自升级。
- 此公开仓是源码唯一入口；父目录旧源码、母盘和独立历史归档都不是开发入口。
- 修改前查看 Git 状态与近期提交，保留其他会话未提交的修改。发行配方不能带入真实 `data/`、Key、钱包或用户工作文件。
- 界面、CLI、维护 MCP 复用同一动作核心。平台相关功能必须区分源码支持、构建成功与实机验收。
- 用户最新的明确指示优先；产品范围变更在 `docs/PRODUCT.md` 更新，不另写一份互相漂移的路线。

## 产品页发布（usb.u-claw.org.cn）

- 该站是虾盘 lite 的产品页，站点根 `/var/www/usb-uclaw/`，下载区 `/var/www/usb-uclaw/download/`（zip 与同名 `.sha256` 成对放）。
- **页面是 nginx 动态注入统计脚本后的产物**（磁盘 38,979 B → HTTP 39,101 B）。改页面要改磁盘上的原件；拿 HTTP 抓下来的文件改会把注入脚本写死进页面。
- Mac 维护台在 `.cache/xiapan-usb-site/`（`pages/`、`payload/`、`ssh/gz_deploy`、`deploy.sh`、`README.md`）。该目录已被 `.gitignore`，私钥不进仓库。
- 发版流程：改页面 → `bash deploy.sh --dry-run` → `bash deploy.sh`（脚本自带备份、`nginx -t`、线上验收与回滚命令）。
- Windows 原件：`C:\1new-code\虾盘-ai优盘精灵版u盘版本\gosim-2026\site\`；在线发布后要回推这两个页面，否则两边会漂。
- 发页面前后都要核对「版本号 / 体积 / SHA-256」在页面里的 12 处是否一致，并用线上 zip 的实际 Content-Length 与本地 `shasum` 对比。
