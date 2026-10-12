"""Build a clean, relocatable customer master. Never copies customer data.

Builds dist/windows-x64/虾盘.exe from this worktree before staging.
All destinations must be new; failed staging remains for inspection.
"""
import argparse
import hashlib
import json
import os
import shutil
import subprocess
import tempfile
from pathlib import Path

ROOT = Path(__file__).resolve().parent.parent


def digest(path):
    with path.open('rb') as stream:
        return hashlib.file_digest(stream, 'sha256').hexdigest()


def checked_copy(source, target, expected=None):
    if source.is_symlink() or not source.is_file():
        raise ValueError(f'Not a regular source file: {source}')
    if expected and digest(source) != expected:
        raise ValueError(f'Checksum mismatch: {source}')
    target.parent.mkdir(parents=True, exist_ok=True)
    shutil.copy2(source, target)


def text(path, content, bom=False):
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(content, encoding='utf-8-sig' if bom else 'utf-8')


def copy_reviewed_tree(source, target):
    source = source.resolve()
    if any(p.is_symlink() for p in source.rglob('*')):
        raise ValueError('Supplemental assets may not contain links')
    expected_files = set()
    for line in (source / 'SHA256SUMS.txt').read_text('utf-8-sig').splitlines():
        expected, rel = line.split('  ', 1)
        path = (source / rel).resolve()
        if not path.is_relative_to(source) or not path.is_file() or digest(path) != expected:
            raise ValueError(f'Supplemental asset checksum mismatch: {rel}')
        expected_files.add(rel)
    actual = {p.relative_to(source).as_posix() for p in source.rglob('*') if p.is_file()}
    if actual != expected_files | {'SHA256SUMS.txt'}:
        raise ValueError('Supplemental staging contains unreviewed files')
    shutil.copytree(source, target)


def command(path, tool_id):
    # GUI and shortcuts invoke the same checked action, never a second launcher.
    text(path.parent / '参数' / (tool_id + '.json'), json.dumps({'tool_id': tool_id, 'confirmed': True}))
    text(path, '@echo off\nchcp 65001 >nul\n'
         'pushd "%~dp0.."\n'
         'if errorlevel 1 exit /b 1\n'
         f'"虾盘.exe" action run tools.launch --root . --json --no-input --input-file "%~dp0参数\\{tool_id}.json"\n'
         'if errorlevel 1 pause\n'
         'popd\n')


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--resources', type=Path, required=True)
    parser.add_argument('--extras', type=Path, required=True)
    parser.add_argument('--output', type=Path, required=True)
    parser.add_argument('--large-agent', type=Path)
    parser.add_argument('--large-agent-sha256', help='Hash of the separately audited clean original agent archive')
    parser.add_argument('--boot-assets', type=Path, help='Reviewed rescue preparation assets with SHA256SUMS.txt')
    parser.add_argument('--network-assets', type=Path, help='Reviewed optional offline network driver assets with SHA256SUMS.txt')
    parser.add_argument('--allow-dirty', action='store_true', help='Internal candidate only; never use for customer master')
    args = parser.parse_args()
    resource, target = args.resources.resolve(), args.output.resolve()
    if target.exists():
        raise SystemExit('Refusing to overwrite an existing master. Choose a new output directory.')
    source_dirty = bool(subprocess.check_output(['git', 'status', '--porcelain'], cwd=ROOT).strip())
    if source_dirty and not args.allow_dirty:
        raise SystemExit('Commit the reviewed source before creating a customer master.')
    if args.large_agent and not args.large_agent_sha256:
        raise SystemExit('The optional large agent requires its audited SHA256.')
    exe = ROOT / 'dist/windows-x64/虾盘.exe'
    exe.parent.mkdir(parents=True, exist_ok=True)
    build_env = dict(os.environ, GOOS='windows', GOARCH='amd64', CGO_ENABLED='0',
                     GOCACHE=str(ROOT / '.cache/go-build'))
    subprocess.run(['go', 'build', '-buildvcs=false', '-trimpath', '-ldflags=-s -w -H windowsgui',
                    '-o', str(exe), '.'], cwd=ROOT, env=build_env, timeout=300, check=True)
    version = subprocess.check_output([str(exe), 'version'], timeout=15).decode().strip()
    local = json.loads((ROOT / 'catalog/local-ai.json').read_text('utf-8-sig'))
    pico = json.loads((ROOT / 'catalog/picoclaw.json').read_text('utf-8-sig'))
    tools = json.loads((ROOT / 'catalog/portable-tools.json').read_text('utf-8-sig'))
    packages = json.loads((ROOT / 'catalog/tool-packages.json').read_text('utf-8-sig'))
    (ROOT / '.cache').mkdir(exist_ok=True)
    # Do not let a stale ordinary 0.4.4 binary masquerade as an offline edition.
    with tempfile.TemporaryDirectory(prefix='offline-package-probe-', dir=ROOT / '.cache') as folder:
        probe = Path(folder)
        text(probe / 'app/OFFLINE_ONLY', '')
        def action(name):
            output = subprocess.check_output([str(exe), 'action', 'run', name, '--root', str(probe), '--json', '--no-input'], timeout=30)
            result = json.loads(output)
            if not result['ok']:
                raise ValueError(f'Build probe failed: {result}')
            return result['data']
        chat = action('chat.start')
        if chat.get('offline_only') is not True or chat.get('engine') != 'guide':
            raise ValueError('Executable does not implement the offline customer edition')
        available = {r['id'] for r in action('tools.catalog')['portable_tools']}
        if not {'crystaldiskinfo', 'testdisk', 'photorec'}.issubset(available):
            raise ValueError('Executable has a stale embedded tool catalog')
    target.mkdir(parents=True)
    (target / 'data').mkdir()
    checked_copy(exe, target / '虾盘.exe')
    text(target / 'app/OFFLINE_ONLY', '')
    checked_copy(resource / pico['path'], target / 'app' / pico['path'], pico['sha256'])
    checked_copy(resource / local['model_path'], target / local['model_path'], local['model_sha256'])
    for item in local['engine_files']:
        checked_copy(resource / item['path'], target / 'app' / item['path'], item['sha256'])
    shutil.copytree(ROOT / 'licenses', target / 'app/licenses')
    for name in ['LICENSE', 'THIRD_PARTY.md']:
        checked_copy(ROOT / name, target / 'app' / name)
    for name in ['local-ai.json', 'picoclaw.json', 'portable-tools.json', 'tool-packages.json', 'offline-references.json']:
        checked_copy(ROOT / 'catalog' / name, target / 'app/catalog' / name)
    wrappers = target / '02-离线工具'
    wrappers.mkdir()
    for index, tool in enumerate(tools, 1):
        source = resource / 'runtime/tool-library/windows-x64' / tool['id']
        dest = target / tool['directory']
        # Whitelist only pinned upstream files, not a used tool's configuration or logs.
        for rel, expected in tool['files'].items():
            checked_copy(source / rel, dest / rel, expected)
        for rel, content in tool.get('init_files', {}).items():
            text(dest / rel, content)
        command(wrappers / f'{index:02}-{tool["name"]}.cmd', tool['id'])
    for spec in packages:
        archive = spec.get('source_archive')
        if archive:
            checked_copy(resource / 'runtime/tool-library/sources' / archive['filename'],
                         target / 'app/tool-sources' / archive['filename'], archive['sha256'])
    # Extra tools are generated in a clean separate staging directory by the preparer.
    extras = args.extras.resolve()
    if not (extras / 'UPSTREAM.json').is_file():
        raise ValueError('Missing extra-tool provenance manifest')
    if any(p.is_symlink() for p in extras.rglob('*')):
        raise ValueError('Extra tools may not contain links')
    extra_checksums = {}
    for line in (extras / 'SHA256SUMS.txt').read_text('utf-8-sig').splitlines():
        expected, rel = line.split('  ', 1)
        path = (extras / rel).resolve()
        if not path.is_relative_to(extras) or digest(path) != expected:
            raise ValueError(f'Extra source checksum mismatch: {rel}')
        extra_checksums[rel] = expected
    extra_files = {p.relative_to(extras).as_posix() for p in extras.rglob('*') if p.is_file()}
    if extra_files != set(extra_checksums) | {'SHA256SUMS.txt'}:
        raise ValueError('Extra staging contains files outside its reviewed checksum manifest')
    shutil.copytree(extras, target / 'app/extras')
    offline_tools = json.loads((ROOT / 'catalog/offline-tools.json').read_text('utf-8'))
    for tool in offline_tools:
        for rel, expected in tool['files'].items():
            if digest(target / tool['directory'] / rel) != expected:
                raise ValueError(f'Extra checksum mismatch: {tool["id"]}/{rel}')
    checked_copy(ROOT / 'catalog/offline-tools.json', target / 'app/catalog/offline-tools.json')
    for label, rel, tool_id in [('07-硬盘健康-CrystalDiskInfo', 'tools/crystaldiskinfo/DiskInfo64.exe', 'crystaldiskinfo'),
                       ('08-文件恢复-PhotoRec-文字界面', 'tools/testdisk/photorec_win.exe', 'photorec'),
                       ('09-分区恢复-TestDisk-专业人员使用', 'tools/testdisk/testdisk_win.exe', 'testdisk')]:
        if not (target / 'app/extras' / rel).is_file():
            raise ValueError(f'Missing extra executable: {rel}')
        command(wrappers / (label + '.cmd'), tool_id)
    native = {'10-任务管理器': 'task-manager', '11-资源监视器': 'resource-monitor',
              '12-设备管理器': 'device-manager', '13-事件查看器': 'event-viewer',
              '14-系统信息': 'system-info', '15-磁盘清理': 'disk-cleanup',
              '16-内存诊断-自行选择是否重启': 'memory-diagnostic', '17-网络适配器': 'network-adapters',
              '18-网络设置': 'network-settings'}
    for label, tool_id in native.items():
        command(wrappers / (label + '.cmd'), tool_id)
    if args.large_agent:
        checked_copy(args.large_agent, target / '03-大龙虾-可选' / args.large_agent.name, args.large_agent_sha256.lower())
        text(target / '03-大龙虾-可选/先看说明.txt',
             'U-DSH 是独立的大型 AI 工作台便携自解压包。它不属于本盘默认离线维修链路。\n'
             '双击自解压程序，选择客户电脑 NTFS 磁盘的新空文件夹，按该产品说明启动。此版本依赖目录联接，不能直接在 exFAT/FAT32 上运行。\n'
             '本次为 0.2.2 源码构建，Electron 桌面窗口内嵌 Harness WebUI；随包内核和 Node 运行时已齐。\n'
             '联网后设备钱包可自动开通并配置虾盘云模型，点击“一键开通并充值”进入对应钱包充值页，无需手填 API Key。充值由客户自行决定。\n'
             '本包没有预填账号或密钥。请自行备份钱包密钥；使用后的程序目录不能再作为客户发盘母版。\n'
             '断网不阻挡界面启动，但钱包充值和默认云模型需要网络。没有验证 U-DSH 对本盘小模型的工具调用能力。\n'
             '无网维护请用根目录的“开始维护.cmd”。\n')
    if args.boot_assets:
        copy_reviewed_tree(args.boot_assets, target / '04-启动救援准备')
    if args.network_assets:
        copy_reviewed_tree(args.network_assets, target / '05-旧网卡离线驱动-可选')
    text(target / '开始维护.cmd', '@echo off\nchcp 65001 >nul\nstart "" "%~dp0虾盘.exe"\n')
    shutil.copytree(ROOT / 'docs/offline-customer', target / '01-使用说明')
    checked_copy(ROOT / 'docs/offline-customer/先看这里.html', target / '先看这里.html')
    (target / 'drivers').mkdir()
    text(target / '01-使用说明/驱动准备.txt',
         'drivers/ 初始为空：供维护人员按客户硬件 ID 准备已匹配的驱动。先用虾盘的“驱动”检测取得硬件 ID，再按厂商、系统版本、位数准备签名驱动到 drivers/。不同机器的驱动不能凭名称混装。\n' +
         ('另附“05-旧网卡离线驱动-可选”传统网卡驱动池，请先读该目录说明。它不是当前全型号通用库，没有在客户设备上安装验收，也未接入主程序自动匹配或安装。\n' if args.network_assets else '当前没有附带实体网卡驱动包。\n'))
    # Exact source snapshot for this build, including the packaging logic and offline guide.
    own_source = target / 'app/source/xiapan-usb-offline'
    tracked = subprocess.check_output(['git', 'ls-files', '-z', '--cached'], cwd=ROOT).decode().split('\0')
    for rel in sorted(set(filter(None, tracked))):
        if rel.startswith(('.cache/', 'dist/', 'runtime/', 'data/', 'evidence/')):
            continue
        path = ROOT / rel
        if path.is_file():
            checked_copy(path, own_source / rel)
    source_commit = subprocess.check_output(['git', 'rev-parse', 'HEAD'], cwd=ROOT, text=True).strip()
    info = {'edition': '客户离线维护盘', 'date': '2026-10-12', 'version': version,
            'source_commit': source_commit, 'source_dirty': source_dirty,
            'source_snapshot': 'app/source/xiapan-usb-offline',
            'offline_default': True, 'model_finetuned_for_maintenance': False,
            'customer_data_included': False, 'bootable': False,
            'model': local, 'pico': pico, 'large_agent_included': bool(args.large_agent),
            'large_agent': {'filename': args.large_agent.name, 'sha256': args.large_agent_sha256.lower()} if args.large_agent else None,
            'boot_preparation_included': bool(args.boot_assets),
            'optional_network_driver_assets_included': bool(args.network_assets)}
    text(target / '校验母盘.ps1', '''$ErrorActionPreference='Stop'
$root=$PSScriptRoot
$m=Get-Content -LiteralPath (Join-Path $root 'app/manifest.json') -Raw -Encoding UTF8 | ConvertFrom-Json
$failed=@()
$known=@{}
foreach($p in $m.files.PSObject.Properties){
  $known[$p.Name]=$true
  $file=Join-Path $root $p.Name
  if(!(Test-Path -LiteralPath $file) -or (Get-FileHash -LiteralPath $file -Algorithm SHA256).Hash.ToLowerInvariant() -ne $p.Value.sha256){$failed+=$p.Name}
}
$known['app/manifest.json']=$true
$known['SHA256SUMS.txt']=$true
foreach($file in (Get-ChildItem -LiteralPath $root -File -Recurse -Force)){
  $relative=$file.FullName.Substring($root.Length+1).Replace([char]92,[char]47)
  if(!$known.ContainsKey($relative)){$failed+=('额外文件：'+$relative)}
}
if(@(Get-ChildItem -LiteralPath (Join-Path $root 'data') -Force).Count){$failed+='data 目录已有使用记录'}
if($failed.Count){$failed; throw '校验失败：文件缺失或变化'}
Write-Host ('校验通过，共 '+@($m.files.PSObject.Properties).Count+' 个资源文件。')
Write-Host '使用后的 data 和工具配置可能发生变化；母盘请始终保持未使用。'
''', bom=True)
    files = {p.relative_to(target).as_posix(): {'bytes': p.stat().st_size, 'sha256': digest(p)}
             for p in sorted(target.rglob('*')) if p.is_file()}
    info['files'] = files
    text(target / 'app/manifest.json', json.dumps(info, ensure_ascii=False, indent=2) + '\n')
    text(target / 'SHA256SUMS.txt', ''.join(f'{item["sha256"]}  {rel}\n' for rel, item in files.items()))
    print(json.dumps({'root': str(target), 'bytes': sum(p.stat().st_size for p in target.rglob('*') if p.is_file()),
                      'files': len(files), 'model': local['model_name'], 'source_commit': source_commit,
                      'source_dirty': source_dirty}, ensure_ascii=False))


if __name__ == '__main__':
    main()
