"""Prepare the pinned PicoClaw and llama.cpp runtimes for macOS.

Only the official upstream archives below are downloaded. Each archive must match
its pinned SHA-256 before anything is unpacked. The script writes the manifests
the Go build embeds (catalog/picoclaw.darwin-*.json, catalog/local-ai.darwin-*.json);
rebuild afterwards so the binary trusts the new file hashes.
"""
import argparse
import hashlib
import json
import os
import posixpath
import re
import shutil
import subprocess
import tarfile
import urllib.request
from pathlib import Path

ROOT = Path(__file__).resolve().parent.parent
CACHE = ROOT / '.cache/downloads'
PICO_VERSION, LLAMA_VERSION = '0.3.1', 'b11146'
PICO_URL = f'https://github.com/sipeed/picoclaw/releases/download/v{PICO_VERSION}/'
LLAMA_URL = f'https://github.com/ggml-org/llama.cpp/releases/download/{LLAMA_VERSION}/'
TARGETS = {
    'arm64': {'platform': 'darwin/arm64', 'dir': 'macos-arm64', 'gpu_layers': 99,
              'picoclaw': ('picoclaw_Darwin_arm64.tar.gz', '30266e4edd6b0dfb871c6ac58e69ffdf8039fc65f23f2612f8792f704930765b'),
              'llama': ('llama-b11146-bin-macos-arm64.tar.gz', '1ad3f9eff80edb9dbef4259ad564d1720612ef7eea48fa4afed0e54f5f3d5711')},
    'x64': {'platform': 'darwin/amd64', 'dir': 'macos-x64', 'gpu_layers': 0,
            'picoclaw': ('picoclaw_Darwin_x86_64.tar.gz', '57d4621f912bec29912bd0cdb52658c7410fb0beaaac7fffc85083d264b55127'),
            'llama': ('llama-b11146-bin-macos-x64.tar.gz', '305f0e3a17d2c01eb205cd0a62128357f1ec3b55329cb084d94e5ec0115d7a3b')},
}
# The same weights serve every platform; this pin matches prepare-local-ai.ps1.
MODEL = {'name': 'Qwen3.5-0.8B-Q4_0', 'path': 'models/Qwen3.5-0.8B-Q4_0.gguf',
         'url': 'https://huggingface.co/ggml-org/Qwen3.5-0.8B-GGUF/resolve/8fea620/Qwen3.5-0.8B-Q4_0.gguf?download=true',
         'source': 'https://huggingface.co/ggml-org/Qwen3.5-0.8B-GGUF/tree/8fea620',
         'sha256': '57d1997790d1744fba5b40a7317df71ea5e2acee28c47e78f0cce39c0703f8cf'}


def digest(path):
    with path.open('rb') as stream:
        return hashlib.file_digest(stream, 'sha256').hexdigest()


def fetch(url, filename, expected, destination=None):
    path = destination or CACHE / filename
    if path.exists():
        if digest(path) != expected:
            raise SystemExit(f'已有文件校验不符，保留并停止：{path}')
        return path
    path.parent.mkdir(parents=True, exist_ok=True)
    part = path.with_name(path.name + '.part')
    request = urllib.request.Request(url, headers={'User-Agent': 'Xiapan-runtime'})
    with urllib.request.urlopen(request, timeout=60) as response, part.open('wb') as out:
        shutil.copyfileobj(response, out, 1 << 20)
    if digest(part) != expected:
        part.unlink()
        raise SystemExit(f'下载文件校验失败，未使用：{filename}')
    part.rename(path)
    return path


def unpack(archive, pick):
    """Return {basename: bytes}. Links become plain copies: exFAT has no links and
    the portable path checks refuse them."""
    with tarfile.open(archive) as tar:
        members = {posixpath.normpath(m.name): m for m in tar.getmembers()}

        def body(member, depth=0):
            if member.issym() or member.islnk():
                link = member.linkname
                assert depth < 8 and not link.startswith('/'), member.name
                if member.issym():
                    link = posixpath.join(posixpath.dirname(posixpath.normpath(member.name)), link)
                return body(members[posixpath.normpath(link)], depth + 1)
            assert member.isfile(), member.name
            return tar.extractfile(member).read()

        return {posixpath.basename(name): body(m) for name, m in members.items() if pick(name, m)}


def linked(files, start):
    """Keep the entry and the libraries it loads through @rpath, recursively.
    Release archives also carry versioned aliases and other tools' libraries."""
    keep, queue = {}, [start]
    while queue:
        name = queue.pop()
        if name in keep:
            continue
        if name not in files:
            raise SystemExit(f'官方资源包缺少依赖库：{name}')
        keep[name] = files[name]
        queue += [m.decode() for m in re.findall(rb'@rpath/([A-Za-z0-9._+-]+\.dylib)', files[name])]
    return keep


def install(files, directory, identity):
    """Write files into a fresh or identical runtime directory, then sign Mach-O files."""
    directory.mkdir(parents=True, exist_ok=True)
    rows = []
    for name, data in sorted(files.items()):
        path = directory / name
        if path.exists() and path.read_bytes() != data:
            raise SystemExit(f'运行时已有不同文件，保留并停止：{path}（重新准备请先删除该目录）')
        if not path.exists():
            path.write_bytes(data)
        os.chmod(path, 0o755)
        if identity and data[:4] in (b'\xcf\xfa\xed\xfe', b'\xca\xfe\xba\xbe'):
            subprocess.run(['codesign', '--force', '--options', 'runtime', '--timestamp', '--sign', identity, str(path)], check=True)
        rows.append({'path': path.relative_to(ROOT).as_posix(), 'sha256': digest(path)})
    return rows


def write_manifest(name, platform_dir, body):
    path = ROOT / 'catalog' / f'{name}.{platform_dir}.json'
    temporary = path.with_suffix('.json.tmp')
    temporary.write_text(json.dumps(body, ensure_ascii=False, indent=2) + '\n', encoding='utf-8')
    temporary.replace(path)
    return path.relative_to(ROOT).as_posix()


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument('--arch', choices=['arm64', 'x64', 'all'], default='all')
    parser.add_argument('--skip-llama', action='store_true', help='PicoClaw only; no offline model runtime')
    parser.add_argument('--with-model', action='store_true', help='Also download the shared 563 MB GGUF model into models/')
    parser.add_argument('--sign-identity', help='Developer ID Application identity for notarized releases; changes file hashes')
    args = parser.parse_args()
    written = []
    for arch in (['arm64', 'x64'] if args.arch == 'all' else [args.arch]):
        t = TARGETS[arch]
        platform_dir = t['platform'].replace('/', '-')
        filename, sha = t['picoclaw']
        archive = fetch(PICO_URL + filename, filename, sha)
        files = unpack(archive, lambda name, m: posixpath.basename(name) == 'picoclaw')
        rows = install(files, ROOT / 'runtime/picoclaw' / t['dir'], args.sign_identity)
        written.append(write_manifest('picoclaw', platform_dir, {
            'ready': True, 'platform': t['platform'], 'path': rows[0]['path'], 'sha256': rows[0]['sha256'], 'version': PICO_VERSION,
            'archive': {'url': PICO_URL + filename, 'sha256': sha}, 'locally_signed': bool(args.sign_identity)}))
        if args.skip_llama:
            continue
        filename, sha = t['llama']
        archive = fetch(LLAMA_URL + filename, filename, sha)
        with tarfile.open(archive) as tar:
            server = [posixpath.normpath(m.name) for m in tar.getmembers() if posixpath.basename(m.name) == 'llama-server']
        if len(server) != 1:
            raise SystemExit('官方资源包中没有唯一的 llama-server')
        folder = posixpath.dirname(server[0])
        files = linked(unpack(archive, lambda name, m: posixpath.dirname(name) == folder and (
            posixpath.basename(name) == 'llama-server' or name.endswith('.dylib'))), 'llama-server')
        rows = install(files, ROOT / 'runtime' / t['dir'], args.sign_identity)
        written.append(write_manifest('local-ai', platform_dir, {
            'ready': True, 'platform': t['platform'], 'model_name': MODEL['name'], 'model_path': MODEL['path'],
            'model_sha256': MODEL['sha256'], 'model_source': MODEL['source'], 'engine_version': LLAMA_VERSION,
            'engine_path': f"runtime/{t['dir']}/llama-server", 'engine_files': rows, 'gpu_layers': t['gpu_layers'],
            'archive': {'url': LLAMA_URL + filename, 'sha256': sha}, 'locally_signed': bool(args.sign_identity)}))
    if args.with_model:
        fetch(MODEL['url'], posixpath.basename(MODEL['path']), MODEL['sha256'], ROOT / MODEL['path'])
    print(json.dumps({'manifests': written, 'model_ready': (ROOT / MODEL['path']).exists(),
                      'next': '重新运行 scripts/build.sh，让程序嵌入新的校验清单'}, ensure_ascii=False, indent=2))


if __name__ == '__main__':
    main()
