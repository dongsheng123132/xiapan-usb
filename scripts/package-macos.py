"""Create a fresh macOS USB package from this source tree; never copy user data.

Run scripts/build.sh first. Without --sign-identity the app is ad-hoc signed:
fine for drives written directly, but a downloaded copy needs Developer ID
signing and notarization (prepare the runtimes with the same identity first).
"""
import argparse
import hashlib
import json
import os
import plistlib
import re
import shutil
import subprocess
import uuid
from pathlib import Path

ROOT = Path(__file__).resolve().parent.parent
parser = argparse.ArgumentParser()
parser.add_argument('--with-local-ai', action='store_true', help='Include the verified llama.cpp runtime and the shared GGUF model')
parser.add_argument('--sign-identity', help='Developer ID Application identity; default is ad-hoc signing')
parser.add_argument('--notary-profile', help='notarytool keychain profile; requires --sign-identity')
args = parser.parse_args()
if args.notary_profile and not args.sign_identity:
    raise SystemExit('公证需要 Developer ID 签名：请同时提供 --sign-identity')


def digest(path):
    with path.open('rb') as stream:
        return hashlib.file_digest(stream, 'sha256').hexdigest()


def copy_verified(relative, expected, target):
    source = ROOT / relative
    if not source.is_file() or digest(source) != expected:
        raise SystemExit(f'文件缺失或校验不符：{relative}；请重新运行 scripts/prepare-macos-runtime.py')
    destination = target / relative
    destination.parent.mkdir(parents=True, exist_ok=True)
    shutil.copy2(source, destination)
    return destination


def codesign(path, identity):
    command = ['codesign', '--force', '--sign', identity]
    if identity != '-':
        command += ['--options', 'runtime', '--timestamp']
    subprocess.run(command + [str(path)], check=True)


version = re.search(r'const version = "([^"]+)"', (ROOT / 'main.go').read_text(encoding='utf-8')).group(1)
short = version.split('-')[0]
binary = ROOT / 'dist/macos-universal/xiapan'
if not binary.is_file():
    raise SystemExit('缺少 dist/macos-universal/xiapan，请先运行 scripts/build.sh')
if subprocess.check_output([str(binary), 'version'], timeout=10, text=True).strip() != version:
    raise SystemExit('Build version mismatch')
load_commands = subprocess.check_output(['otool', '-l', str(binary)], text=True)
minimum = max((m for m in re.findall(r'minos (\d+(?:\.\d+)*)', load_commands)), key=lambda v: tuple(map(int, v.split('.'))))

name = f'虾盘 U盘精灵 {short}-macOS' + ('-离线AI' if args.with_local_ai else '') + '-' + uuid.uuid4().hex[:6]
target = ROOT / 'dist' / name
target.mkdir()
(target / 'data').mkdir()
contents = target / '虾盘.app/Contents'
(contents / 'MacOS').mkdir(parents=True)
(contents / 'Resources').mkdir()
shutil.copy2(binary, contents / 'MacOS/xiapan')
os.chmod(contents / 'MacOS/xiapan', 0o755)
with (contents / 'Info.plist').open('wb') as f:
    plistlib.dump({'CFBundleDevelopmentRegion': 'zh_CN', 'CFBundleExecutable': 'xiapan', 'CFBundleIdentifier': 'org.u-claw.xiapan',
                   'CFBundleInfoDictionaryVersion': '6.0', 'CFBundleName': '虾盘', 'CFBundleDisplayName': '虾盘',
                   'CFBundlePackageType': 'APPL', 'CFBundleShortVersionString': short, 'CFBundleVersion': short,
                   'LSApplicationCategoryType': 'public.app-category.utilities', 'LSMinimumSystemVersion': minimum,
                   # A faceless launcher: the interface is the local browser page.
                   'LSUIElement': True, 'NSHumanReadableCopyright': 'MIT License · Xiapan USB Toolkit'}, f)

included = {'picoclaw': [], 'local_ai': []}
for path in sorted((ROOT / 'catalog').glob('picoclaw.darwin-*.json')):
    m = json.loads(path.read_text(encoding='utf-8'))
    if m.get('ready'):
        copy_verified(m['path'], m['sha256'], target / 'app')
        included['picoclaw'].append(m['platform'])
if args.with_local_ai:
    for path in sorted((ROOT / 'catalog').glob('local-ai.darwin-*.json')):
        m = json.loads(path.read_text(encoding='utf-8'))
        if not m.get('ready'):
            continue
        for f in m['engine_files']:
            copy_verified(f['path'], f['sha256'], target / 'app')
        if not (target / m['model_path']).exists():
            copy_verified(m['model_path'], m['model_sha256'], target)
        included['local_ai'].append(m['platform'])
    if not included['local_ai']:
        raise SystemExit('没有已准备的 macOS 离线 AI 运行时')
shutil.copytree(ROOT / 'licenses', target / 'app/licenses')
for filename in ['LICENSE', 'THIRD_PARTY.md']:
    shutil.copy2(ROOT / filename, target / 'app' / filename)
(target / 'app/使用说明.txt').write_text(
    '双击「虾盘.app」，浏览器会打开虾盘界面；关掉页面后再次双击即可重新打开。\n'
    '用完点击页面右上角「退出」，再推出 U 盘。\n'
    '模型设置可选择虾盘云或自带 OpenAI 兼容 API；设置、密钥及备份、对话保存在 data/，请妥善保管。\n'
    'Windows 与 Mac 共用一只盘时请使用 exFAT 格式；Mac 不能写入 NTFS。\n'
    '如果系统提示无法验证开发者，说明这份包未经过 Apple 公证：请使用公证版本，不要为此关闭系统安全功能。\n'
    '此包不是可启动系统盘；Mac 无法开机时请使用 macOS 恢复功能。\n', encoding='utf-8')

identity = args.sign_identity or '-'
runtime_dir = target / 'app/runtime'
if args.notary_profile and runtime_dir.exists():
    # Notarization rejects nested code that lacks Developer ID and hardened runtime.
    for code in (p for p in runtime_dir.rglob('*') if p.is_file()):
        details = subprocess.run(['codesign', '-dv', '--verbose=2', str(code)], capture_output=True, text=True).stderr
        if 'Authority=Developer ID Application' not in details or '(runtime)' not in details:
            raise SystemExit(f'运行时未使用 Developer ID 与 hardened runtime 签名：{code.name}；请用 --sign-identity 重新准备')
codesign(target / '虾盘.app', identity)
subprocess.run(['codesign', '--verify', '--strict', '--verbose=2', str(target / '虾盘.app')], check=True)
if args.notary_profile:
    submission = target.parent / (name + '.notary.zip')
    subprocess.run(['ditto', '-c', '-k', '--sequesterRsrc', '--keepParent', str(target), str(submission)], check=True)
    result = json.loads(subprocess.check_output(['xcrun', 'notarytool', 'submit', str(submission), '--keychain-profile',
                                                 args.notary_profile, '--wait', '--output-format', 'json'], text=True))
    submission.unlink()
    if result.get('status') != 'Accepted':
        raise SystemExit('公证未通过：' + json.dumps(result, ensure_ascii=False))
    subprocess.run(['xcrun', 'stapler', 'staple', str(target / '虾盘.app')], check=True)

files = {p.relative_to(target).as_posix(): {'bytes': p.stat().st_size, 'sha256': digest(p)} for p in target.rglob('*') if p.is_file()}
commit = subprocess.check_output(['git', 'rev-parse', 'HEAD'], cwd=ROOT, text=True).strip()
info = {'version': version, 'platform': 'macos-universal', 'minimum_macos': minimum, 'source_commit': commit, 'picoclaw_platforms': included['picoclaw'],
        'local_ai_platforms': included['local_ai'], 'signing': 'developer-id' if args.sign_identity else 'ad-hoc',
        'notarized': bool(args.notary_profile), 'files': files, 'user_data_included': False}
(target / 'app/manifest.json').write_text(json.dumps(info, ensure_ascii=False, indent=2), encoding='utf-8')
archive = Path(str(target) + '.zip')
subprocess.run(['ditto', '-c', '-k', '--sequesterRsrc', '--keepParent', str(target), str(archive)], check=True)
archive.with_suffix('.zip.sha256').write_text(digest(archive) + '  ' + archive.name + '\n', encoding='utf-8')
result = {'root': str(target), 'archive': str(archive), 'zip_bytes': archive.stat().st_size, 'sha256': digest(archive),
          'picoclaw': included['picoclaw'], 'local_ai': included['local_ai'], 'signing': info['signing'], 'notarized': info['notarized']}
(ROOT / 'evidence').mkdir(exist_ok=True)
(ROOT / 'evidence/genie-package-macos.json').write_text(json.dumps(result, ensure_ascii=False, indent=2), encoding='utf-8')
print(json.dumps(result, ensure_ascii=False, indent=2))
if not included['picoclaw']:
    print('提示：未包含 PicoClaw，「AI 助手」引擎不可用；可先运行 scripts/prepare-macos-runtime.py。虾盘云直连与本地工具不受影响。')
