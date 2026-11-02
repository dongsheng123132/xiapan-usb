#!/usr/bin/env bash
# ============================================================
# usb.u-claw.org.cn 站点更新到虾盘 lite 0.5.8
#
# 用法（在有服务器发布权限的机器上跑）：
#   SSH_TARGET=root@203.195.204.252 bash deploy-usb-site.sh --dry-run
#   SSH_TARGET=root@203.195.204.252 bash deploy-usb-site.sh
#
# 前提：本目录里已有
#   site-index.html / site-engineering.html        已改好的两个页面（CRLF 保留）
#   xiapan-usb-0.5.8-preview-windows-x64.zip       29.9 MB 体验包
#   （可选）xiapan-usb-0.5.8-preview-macos-arm64.zip
#
# 做的事：
#   1. 从 nginx -T 读出 usb.u-claw.org.cn 的 root 与 /download/ 落盘目录（不猜路径）
#   2. 备份线上两个页面 + 旧 zip 清单到 /var/www/_backup-usb-<ts>/
#   3. 上传新页面与新 zip；旧 zip 一律保留（回滚用）
#   4. nginx -t && systemctl reload nginx
#   5. 线上验收：页面含 0.5.8、zip 200 且 Content-Length / sha256 与本地一致
# 回滚：把备份里的 index.html / engineering.html 覆盖回 SITE_ROOT，nginx -t && reload
# ============================================================
set -euo pipefail

SSH_TARGET="${SSH_TARGET:-root@203.195.204.252}"   # 默认广州机；可用环境变量覆盖
DOMAIN="usb.u-claw.org.cn"
HERE="$(cd "$(dirname "$0")" && pwd)"
WIN_ZIP="xiapan-usb-0.5.8-preview-windows-x64.zip"
MAC_ZIP="xiapan-usb-0.5.8-preview-macos-arm64.zip"
TS="$(date +%Y%m%d-%H%M%S)"
BACKUP="/var/www/_backup-usb-$TS"
DRY=0
[ "${1:-}" = "--dry-run" ] && DRY=1

say() { printf '\033[36m== %s\033[0m\n' "$*"; }
die() { printf '\033[31m[FAIL] %s\033[0m\n' "$*" >&2; exit 1; }
filesize() { stat -f%z "$1" 2>/dev/null || stat -c%s "$1"; }

say "0/6 本地物料检查"
for f in site-index.html site-engineering.html; do
  [ -f "$HERE/$f" ] || die "缺文件：$HERE/$f"
done
EXPECT_SHA="e00b881635b041204cd1690f0961079012ab67895b99a706482f63eacf588ab5"
if [ ! -f "$HERE/$WIN_ZIP" ]; then
  say "本目录没有体验包 → 从 GitHub Release 取（只改名，不重新打包）"
  curl -fL --retry 3 --connect-timeout 15 -o "$HERE/$WIN_ZIP" \
    "https://github.com/dongsheng123132/xiapan-usb/releases/download/v0.5.8-preview/xiapan-u-claw-lite-0.5.8-windows-x64.zip" \
    || die "体验包下载失败；请手工把 Release 资产放进 $HERE/$WIN_ZIP"
fi
HAVE_MAC=0; [ -f "$HERE/$MAC_ZIP" ] && HAVE_MAC=1
WIN_SHA=$( (shasum -a 256 "$HERE/$WIN_ZIP" 2>/dev/null || sha256sum "$HERE/$WIN_ZIP") | awk '{print $1}')
[ "$WIN_SHA" = "$EXPECT_SHA" ] || die "体验包 SHA256 不符（期望 $EXPECT_SHA，实际 $WIN_SHA）——拒绝发布"
WIN_SIZE=$(filesize "$HERE/$WIN_ZIP")
echo "   win zip: $WIN_SIZE bytes  sha256=$WIN_SHA"
echo "   mac zip: $([ "$HAVE_MAC" = 1 ] && echo 有 || echo 无（本次不发 mac 包）)"

say "1/6 从 nginx 配置定位站点目录（不猜）"
ssh "$SSH_TARGET" 'nginx -T 2>/dev/null' > /tmp/nginx-T.conf || die "拿不到 nginx -T"
SITE_ROOT=$(python3 - /tmp/nginx-T.conf "$DOMAIN" <<'PY'
import re,sys
conf=open(sys.argv[1],encoding='utf-8',errors='ignore').read()
domain=sys.argv[2]
for m in re.finditer(r'server\s*\{', conf):
    i=m.end(); depth=1
    while i < len(conf) and depth:
        if conf[i]=='{': depth+=1
        elif conf[i]=='}': depth-=1
        i+=1
    block=conf[m.start():i]
    if re.search(r'server_name[^;]*\b'+re.escape(domain)+r'\b', block):
        root=re.search(r'\broot\s+([^;]+);', block)
        print(root.group(1).strip() if root else '')
        break
PY
)
[ -n "$SITE_ROOT" ] || die "nginx -T 里没找到 $DOMAIN 的 root —— 不写任何文件，先人工确认"
echo "   SITE_ROOT = $SITE_ROOT"

DL_DIR=$(ssh "$SSH_TARGET" "for d in '$SITE_ROOT/download' /var/www/*/download; do [ -d \"\$d\" ] && ls \"\$d\"/xiapan-usb-0.4.4-preview-windows-x64.zip >/dev/null 2>&1 && echo \"\$d\" && break; done")
[ -n "$DL_DIR" ] || die "找不到放着 0.4.4 旧 zip 的 download 目录 —— 不写任何文件"
echo "   DL_DIR    = $DL_DIR"

if [ "$DRY" = 1 ]; then
  say "dry-run：以下动作不会真的执行"
  echo "   backup  -> $BACKUP （两个页面 + download-before.txt）"
  echo "   install -> $SITE_ROOT/site-index.html site-engineering.html"
  echo "   install -> $DL_DIR/$WIN_ZIP"
  echo "   reload  -> nginx -t && systemctl reload nginx"
  exit 0
fi

say "2/6 备份线上现状"
ssh "$SSH_TARGET" "install -d -m 755 '$BACKUP' && cp -a '$SITE_ROOT/index.html' '$BACKUP/index.html' && cp -a '$SITE_ROOT/engineering.html' '$BACKUP/engineering.html' && (cd '$DL_DIR' && ls -la *.zip > '$BACKUP/download-before.txt')"
echo "   备份在 $BACKUP"

say "3/6 上传物料到临时目录"
ssh "$SSH_TARGET" 'install -d -m 755 /tmp/usb-site-new'
scp -q "$HERE/site-index.html" "$HERE/site-engineering.html" "$SSH_TARGET:/tmp/usb-site-new/"
scp -q "$HERE/$WIN_ZIP" "$SSH_TARGET:/tmp/usb-site-new/$WIN_ZIP"
if [ "$HAVE_MAC" = 1 ]; then scp -q "$HERE/$MAC_ZIP" "$SSH_TARGET:/tmp/usb-site-new/$MAC_ZIP"; fi

say "4/6 落地 + 原子替换"
ssh "$SSH_TARGET" "set -e
  install -m 644 -o root -g root /tmp/usb-site-new/site-index.html      '$SITE_ROOT/.index.html.new'
  install -m 644 -o root -g root /tmp/usb-site-new/site-engineering.html '$SITE_ROOT/.engineering.html.new'
  mv -f '$SITE_ROOT/.index.html.new'       '$SITE_ROOT/index.html'
  mv -f '$SITE_ROOT/.engineering.html.new' '$SITE_ROOT/engineering.html'
  install -m 644 -o root -g root /tmp/usb-site-new/$WIN_ZIP '$DL_DIR/.$WIN_ZIP.new'
  mv -f '$DL_DIR/.$WIN_ZIP.new' '$DL_DIR/$WIN_ZIP'
  if [ -f /tmp/usb-site-new/$MAC_ZIP ]; then
    install -m 644 -o root -g root /tmp/usb-site-new/$MAC_ZIP '$DL_DIR/.$MAC_ZIP.new'
    mv -f '$DL_DIR/.$MAC_ZIP.new' '$DL_DIR/$MAC_ZIP'
  fi
  ls -la '$SITE_ROOT/index.html' '$SITE_ROOT/engineering.html' '$DL_DIR'/*0.5.8*.zip"

say "5/6 nginx 语法检查 + 重载"
ssh "$SSH_TARGET" 'nginx -t && systemctl reload nginx'

say "6/6 线上验收"
curl -fsS "https://usb.u-claw.org.cn/index.html" | grep -q "Windows x64 · 0.5.8-preview" \
  || die "线上首页还没出现 0.5.8 下载卡（可能被 CDN/缓存挡住）"
curl -fsS "https://usb.u-claw.org.cn/index.html" | grep -q "$WIN_SHA" \
  || die "线上首页的 SHA-256 与本地 zip 不一致"
curl -fsS "https://usb.u-claw.org.cn/engineering.html" | grep -q "xiapan-usb/tree/main" \
  || die "engineering.html 的 GitHub 链接没换成 main"
REMOTE_SIZE=$(curl -fsSI "https://usb.u-claw.org.cn/download/$WIN_ZIP" | tr -d '\r' | awk 'tolower($1)=="content-length:"{print $2}')
echo "   zip Content-Length: $REMOTE_SIZE（本地 $WIN_SIZE）"
[ "$REMOTE_SIZE" = "$WIN_SIZE" ] || die "线上 zip 大小与本地不一致"
echo
echo "✅ 完成。页面：https://usb.u-claw.org.cn/   包：https://usb.u-claw.org.cn/download/$WIN_ZIP"
echo "   回滚：ssh $SSH_TARGET \"cp -a $BACKUP/index.html $BACKUP/engineering.html $SITE_ROOT/ && nginx -t && systemctl reload nginx\""
