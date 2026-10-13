"""Assemble a fresh three-platform U-PicoClaw master. Never copy customer data."""
import argparse
import hashlib
import json
import os
from pathlib import Path
import shutil
import uuid
import subprocess
from datetime import datetime, timezone

ROOT = Path(__file__).resolve().parent.parent


def digest(path):
    with path.open('rb') as stream:
        return hashlib.file_digest(stream, 'sha256').hexdigest()


def checked_path(root, relative):
    path = root / relative
    if not path.resolve().is_relative_to(root.resolve()) or path.is_symlink():
        raise ValueError(f'Unsafe resource path: {relative}')
    return path


def copy_checked(source, destination, expected=None):
    if not source.is_file() or source.is_symlink():
        raise ValueError(f'Missing regular file: {source}')
    if expected and digest(source) != expected:
        raise ValueError(f'Checksum mismatch: {source}')
    destination.parent.mkdir(parents=True, exist_ok=True)
    shutil.copy2(source, destination)
    copied_hash = digest(destination)
    if (expected and copied_hash != expected) or digest(source) != copied_hash:
        raise ValueError(f'Copy mismatch: {destination}')


def copy_public_tree(source, destination):
    if not source.is_dir() or source.is_symlink():
        raise ValueError(f'Missing or linked resource directory: {source}')
    for ancestor in source.parents:
        if ancestor.is_symlink():
            raise ValueError(f'Linked resource parent: {ancestor}')
    for path in source.rglob('*'):
        if path.is_symlink():
            raise ValueError(f'Links are not release resources: {path}')
        if path.is_file():
            copy_checked(path, destination / path.relative_to(source))


def current_source_fingerprint(source_root=ROOT):
    paths = list(source_root.glob('*.go')) + [source_root / 'go.mod', source_root / 'go.sum', source_root / 'scripts/build.ps1']
    paths += [p for p in (source_root / 'web').rglob('*') if p.is_file()]
    paths += list((source_root / 'catalog').glob('*.json')) + list((source_root / 'skills').glob('*/SKILL.md'))
    entries = {p.relative_to(source_root).as_posix(): digest(p) for p in paths if p.is_file()}
    canonical = ''.join(name + '\0' + entries[name] + '\n' for name in sorted(entries, key=lambda n: n.encode('utf-16-be')))
    return hashlib.sha256(canonical.encode('utf-8')).hexdigest()


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument('--resources', type=Path, required=True,
                        help='Existing verified tools-only resource source; data is never copied')
    parser.add_argument('--output', type=Path, required=True, help='A new, nonexistent master directory')
    args = parser.parse_args()
    resources, destination = args.resources.resolve(), args.output.resolve()
    target = destination
    if target.exists() or target.is_relative_to(resources):
        raise ValueError('Output must be a new directory outside the resource source')
    build = json.loads((ROOT / 'dist/build-manifest.json').read_text(encoding='utf-8-sig'))
    if build['fingerprint_algorithm'] != 'sha256-ordinal-path-nul-file-sha256-lf-v1' or build['source_fingerprint'] != current_source_fingerprint():
        raise ValueError('Source changed since build; rebuild all platforms')
    source_commit = subprocess.check_output(['git', 'rev-parse', 'HEAD'], cwd=ROOT).decode().strip()
    if build['source_commit'] != source_commit:
        raise ValueError('Commit changed since build; rebuild release evidence')
    if subprocess.check_output(['git', 'status', '--porcelain'], cwd=ROOT).strip():
        raise ValueError('Commit the reviewed source before making a shipping master')
    manifest = json.loads((ROOT / 'catalog/picoclaw.json').read_text(encoding='utf-8'))
    platforms = {'windows/amd64': ('windows-x64', '虾盘.exe', '虾盘.exe'),
                 'darwin/arm64': ('macos-arm64', 'xiapan', 'app/bin/xiapan-macos-arm64'),
                 'darwin/amd64': ('macos-x64', 'xiapan', 'app/bin/xiapan-macos-x64')}
    target = destination.with_name('.building-' + destination.name + '-' + uuid.uuid4().hex[:8])
    target.mkdir(parents=True)
    incomplete = target / '未完成-不能发货.txt'
    incomplete.write_text('构建尚未完成。只有脚本成功改名后的目录可作为母盘。', encoding='utf-8')
    (target / 'data').mkdir()
    for platform, (build_dir, binary, binary_destination) in platforms.items():
        spec = build['platforms'][platform]
        expected_path = 'dist/' + build_dir + '/' + binary
        if spec['path'] != expected_path or spec['version'] != build['version']:
            raise ValueError(f'Invalid build provenance: {platform}')
        copy_checked(ROOT / expected_path, target / binary_destination, spec['sha256'])
        slot = manifest['platforms'][platform]
        for path_key, hash_key in [('path', 'sha256'), ('launcher_path', 'launcher_sha256')]:
            if not slot.get(path_key) or not slot.get(hash_key):
                raise ValueError(f'Core and official launcher must both be pinned: {platform}')
            relative = slot[path_key]
            copy_checked(checked_path(ROOT, relative), checked_path(target, 'app/' + relative), slot[hash_key])
    build_version = subprocess.check_output([str(target / '虾盘.exe'), 'version'], timeout=10).decode().strip()
    if build_version != build['version']:
        raise ValueError('Unexpected application build version')
    # Copy only pinned immutable files and clean initial preferences, never used tool settings.
    all_tools = []
    for name in ['portable-tools.json', 'emergency-tools.json']:
        all_tools.extend(json.loads((ROOT / 'catalog' / name).read_text(encoding='utf-8')))
    for tool in all_tools:
        for relative, expected in tool['files'].items():
            source = checked_path(resources, tool['directory'] + '/' + relative)
            dest = checked_path(target, tool['directory'] + '/' + relative)
            if not dest.exists():
                copy_checked(source, dest, expected)
            elif digest(dest) != expected:
                raise ValueError(f'Conflicting pinned tool content: {dest}')
        for relative, body in tool.get('init_files', {}).items():
            dest = checked_path(target, tool['directory'] + '/' + relative)
            if not dest.exists():
                dest.parent.mkdir(parents=True, exist_ok=True)
                dest.write_text(body, encoding='utf-8')
    for relative in ['app/tool-sources', 'app/extras/licenses', 'app/extras/sources']:
        copy_public_tree(resources / relative, target / relative)
    for name in ['UPSTREAM.json', 'IMPORT-CLOSURE.json', 'README.txt']:
        copy_checked(resources / 'app/extras' / name, target / 'app/extras' / name)
    copy_public_tree(ROOT / 'licenses', target / 'app/licenses')
    for name in ['LICENSE', 'THIRD_PARTY.md']:
        copy_checked(ROOT / name, target / 'app' / name)
    copy_public_tree(ROOT / 'catalog', target / 'app/catalog')
    copy_checked(ROOT / 'dist/build-manifest.json', target / 'app/build-manifest.json')
    # This known historical driver kit is optional and explicitly limited to old hardware.
    driver_source = resources / '05-旧网卡离线驱动-可选'
    driver_dest = target / 'drivers/windows-x64/3dp-net-21.01'
    for name in ['3DP_Net_v2101.exe', '使用前先看.md', 'INF-DRIVERS.json', 'INSPECTION.json', 'LICENSE-SOURCE.txt', 'MANIFEST.json', 'SHA256SUMS.txt']:
        copy_checked(checked_path(driver_source, name), driver_dest / name)
    driver_files = list(driver_dest.rglob('3DP_Net_v2101.exe'))
    if len(driver_files) != 1 or digest(driver_files[0]) != '67cafee05aea6f706c5a82125b0b57c4e12b43f6b74738d7f276fde4de62058a':
        raise ValueError('Historical driver package checksum mismatch')
    # Source snapshot is generated from this repository, never from the old master.
    tracked = subprocess.check_output(['git', 'ls-files', '--cached', '-z'], cwd=ROOT).decode().split('\0')
    for relative in sorted(set(tracked)):
        if not relative or relative.split('/')[0] in {'data', 'runtime', 'dist', '.cache', 'evidence'}:
            continue
        top = relative.split('/')[0]
        if '/' in relative:
            if top not in {'catalog', 'docs', 'licenses', 'scripts', 'skills', 'web', 'diagrams', '.github'}:
                continue
        elif Path(relative).suffix not in {'.go', '.md', '.mod', '.sum', '.svg'} and relative not in {'LICENSE', '.gitignore'}:
            continue
        source = checked_path(ROOT, relative)
        if source.is_file() and not source.name.startswith('.env'):
            copy_checked(source, target / 'app/source/xiapan-usb' / relative)
    (target / '开始使用.cmd').write_text('@echo off\r\nchcp 65001 >nul\r\nstart "" "%~dp0虾盘.exe"\r\n', encoding='utf-8')
    bootstrap_arm = digest(target / 'app/bin/xiapan-macos-arm64')
    bootstrap_intel = digest(target / 'app/bin/xiapan-macos-x64')
    mac_script = '''#!/bin/sh
set -eu
if ! ROOT=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd); then
  echo '无法访问U盘。请在系统设置的隐私与安全性中，允许当前终端/启动程序访问文件与文件夹或可移除宗卷。'
  exit 1
fi
case "$(uname -m)" in arm64) BIN="$ROOT/app/bin/xiapan-macos-arm64"; SHA="__ARM_SHA__";; x86_64) BIN="$ROOT/app/bin/xiapan-macos-x64"; SHA="__INTEL_SHA__";; *) echo '不支持此 Mac 架构'; exit 1;; esac
# Copy only the public executable. Private state always stays at --root/data.
CACHE="$HOME/Library/Caches/Xiapan/bootstrap"
for PART in "$HOME/Library/Caches" "$HOME/Library/Caches/Xiapan" "$CACHE"; do
  [ ! -L "$PART" ] || { echo '缓存目录不能是符号链接'; exit 1; }
done
mkdir -p "$CACHE"
DEST="$CACHE/xiapan-$SHA"
[ ! -L "$DEST" ] || { echo '缓存程序不能是符号链接'; exit 1; }
if [ ! -f "$DEST" ] || [ "$(shasum -a 256 "$DEST" | awk '{print $1}')" != "$SHA" ]; then
  TEMP="$DEST.tmp.$$"
  trap 'rm -f "$TEMP"' EXIT HUP INT TERM
  cp "$BIN" "$TEMP"
  [ "$(shasum -a 256 "$TEMP" | awk '{print $1}')" = "$SHA" ] || { echo '程序校验失败'; exit 1; }
  chmod 700 "$TEMP"
  mv "$TEMP" "$DEST"
  trap - EXIT HUP INT TERM
fi
chmod 700 "$DEST"
exec "$DEST" --root "$ROOT"
'''
    mac_script = mac_script.replace('__ARM_SHA__', bootstrap_arm).replace('__INTEL_SHA__', bootstrap_intel)
    (target / '启动 Mac.command').write_text(mac_script, encoding='utf-8', newline='\n')
    os.chmod(target / '启动 Mac.command', 0o755)
    readme = '''# 虾盘 · U-PicoClaw 随身 AI 盘

Windows：双击“虾盘.exe”。Mac：打开“启动 Mac.command”。首次使用先选择“设备钱包 / 充值”或填写自己的 API Key。设置、凭证、对话留在本盘 data/。

自建界面用于日常快速对话；点击“完整 PicoClaw”打开官方高级界面，按页面设置本地访问密码。两个界面使用同一个持久 Gateway。第一次启动 AI 会验证并复制固定程序到电脑缓存，之后复用本机缓存；不把钱包、Key 或工作文件复制到程序缓存。拔盘前在页面点击“退出”，再关闭已打开的维护工具。

Mac 首次使用：exFAT 的权限和系统信任提示取决于目标电脑。可在终端输入 `sh `（末尾一个空格），拖入本盘的“启动 Mac.command”，按回车。系统安全提示按 macOS 的“隐私与安全性”流程允许本次可信程序；此脚本不会关闭 Gatekeeper 或批量删除 quarantine。Mac 成品仍需目标机器验收。

断网：点击“应急工具”，可检查系统、网络设备及驱动，打开盘内传统工具。Windows 工具不适用于 Mac。离线网卡资源位于 drivers/windows-x64/3dp-net-21.01/，是 2021 年旧库，使用前阅读其中说明，按硬件 ID、系统和签名核对。当前不会自动匹配/安装驱动，也不是可引导的 WinPE 救援盘。

不带本地大模型，不含 OpenClaw，不在客户盘自动更新。软件启动速度与模型联网响应速度分开计算；未对所有 U 盘承诺 0.3 秒。

请自行备份钱包凭证。钱包余额跟随凭证，在另一台电脑插入本盘仍可使用。发货模板 data/ 为空，不能把自己使用过的 data/ 复制给客户。
'''
    (target / '先看这里.md').write_text(readme, encoding='utf-8')
    (target / '校验母盘.ps1').write_text('''$ErrorActionPreference='Stop'
$base=Split-Path -Parent $MyInvocation.MyCommand.Path
foreach($line in Get-Content -LiteralPath (Join-Path $base 'SHA256SUMS') -Encoding UTF8){
  $parts=$line -split '  ',2
  $path=Join-Path $base $parts[1]
  if((Get-FileHash -LiteralPath $path -Algorithm SHA256).Hash.ToLowerInvariant() -ne $parts[0]){throw "校验失败: $($parts[1])"}
}
Write-Host '固定程序与资源校验通过；用户 data/ 不在母盘校验范围内。'
''', encoding='utf-8-sig')
    info = {'edition': 'U-PicoClaw portable AI + emergency toolkit', 'version': build_version,
            'built_at': datetime.now(timezone.utc).isoformat(),
            'source_commit': subprocess.check_output(['git', 'rev-parse', 'HEAD'], cwd=ROOT).decode().strip(),
            'source_dirty': bool(subprocess.check_output(['git', 'status', '--porcelain'], cwd=ROOT).strip()),
            'pico_version': manifest['version'], 'platforms': list(platforms),
            'source_fingerprint': build['source_fingerprint'],
            'customer_data_included': False, 'local_model_included': False, 'bootable': False,
            'automatic_program_updates': False, 'driver_auto_install': False,
            'portable_tools': [t['id'] for t in all_tools]}
    (target / 'app/manifest.json').write_text(json.dumps(info, ensure_ascii=False, indent=2)+'\n', encoding='utf-8')
    if current_source_fingerprint() != build['source_fingerprint'] or current_source_fingerprint(target / 'app/source/xiapan-usb') != build['source_fingerprint']:
        raise ValueError('Source snapshot does not match the binaries')
    if subprocess.check_output(['git', 'status', '--porcelain'], cwd=ROOT).strip():
        raise ValueError('Source changed during assembly')
    incomplete.unlink()
    files = sorted(p for p in target.rglob('*') if p.is_file())
    (target / 'SHA256SUMS').write_text(''.join(f'{digest(p)}  {p.relative_to(target).as_posix()}\n' for p in files), encoding='utf-8')
    total_bytes = sum(p.stat().st_size for p in files)
    target.rename(destination)
    print(json.dumps({'root': str(destination), 'files': len(files), 'bytes': total_bytes, **info}, ensure_ascii=False, indent=2))


if __name__ == '__main__':
    main()
