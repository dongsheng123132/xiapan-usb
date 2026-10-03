"""Prepare pinned upstream portable tools. Never install into the host OS."""
import hashlib
import json
from pathlib import Path
import shutil
import urllib.request
import zipfile

ROOT = Path(__file__).resolve().parent.parent
CACHE = ROOT / '.cache/portable-tool-downloads'
DEST = ROOT / 'runtime/tools/windows-x64'
SOURCES = ROOT / 'runtime/tool-sources'
ASSETS = {
    'WinDirStat.zip': ('https://github.com/windirstat/windirstat/releases/download/release/v2.9.0/WinDirStat.zip', '0d04d9f716b70502658595956ac3af5192f591c6d2636662eea170655546b891'),
    'WinDirStat-2.9.0-source.zip': ('https://api.github.com/repos/windirstat/windirstat/zipball/release/v2.9.0', '8818e4d402e64466f3e6d7cd5c89301f67048d62b89120044f49be8fb42f072b'),
    'peazip_portable-11.3.0.WIN64.zip': ('https://github.com/peazip/PeaZip/releases/download/11.3.0/peazip_portable-11.3.0.WIN64.zip', '3fb15b9852c94ba5ee8d1af5568fad0cad0999854225b4432b9c98b315a4f768'),
    'peazip-11.3.0.src.zip': ('https://github.com/peazip/PeaZip/releases/download/11.3.0/peazip-11.3.0.src.zip', '17e85e62a3b526a03347186465ff4a79f8117311e9efe0c432e968bd98e612cb'),
}


def digest(path):
    with path.open('rb') as stream:
        return hashlib.file_digest(stream, 'sha256').hexdigest()


def extract(archive, target, prefix):
    with zipfile.ZipFile(archive) as z:
        assert sum(i.file_size for i in z.infolist()) < 180 * 1024 * 1024
        for item in z.infolist():
            if not item.filename.startswith(prefix) or item.is_dir():
                continue
            relative = item.filename[len(prefix):]
            path = (target / relative).resolve()
            assert path.is_relative_to(target.resolve()) and path != target.resolve()
            assert (item.external_attr >> 16) & 0o170000 != 0o120000
            path.parent.mkdir(parents=True, exist_ok=True)
            with path.open('xb') as f:
                f.write(z.read(item))


def main():
    CACHE.mkdir(parents=True, exist_ok=True)
    if DEST.exists() or SOURCES.exists():
        raise SystemExit('Prepared directory already exists; use a fresh staging directory, never overwrite tool preferences.')
    for name, (url, expected) in ASSETS.items():
        path = CACHE / name
        if not path.exists():
            request = urllib.request.Request(url, headers={'User-Agent': 'Xiapan-portable-tools'})
            with urllib.request.urlopen(request, timeout=45) as response:
                content = response.read(100 * 1024 * 1024 + 1)
            assert len(content) <= 100 * 1024 * 1024 and hashlib.sha256(content).hexdigest() == expected
            path.write_bytes(content)
        assert digest(path) == expected, name
    DEST.mkdir(parents=True)
    SOURCES.mkdir(parents=True)
    extract(CACHE / 'WinDirStat.zip', DEST / 'windirstat', 'x64/')
    # Upstream checks for this file before loading settings; absent means registry.
    (DEST / 'windirstat/WinDirStat.ini').write_text('', encoding='utf-8')
    with zipfile.ZipFile(CACHE / 'WinDirStat-2.9.0-source.zip') as z:
        license_name = next(n for n in z.namelist() if n.endswith('/LICENSE.md'))
        (DEST / 'windirstat/LICENSE.md').write_bytes(z.read(license_name))
    extract(CACHE / 'peazip_portable-11.3.0.WIN64.zip', DEST / 'peazip', 'peazip_portable-11.3.0.WIN64/')
    for name in ('WinDirStat-2.9.0-source.zip', 'peazip-11.3.0.src.zip'):
        shutil.copy2(CACHE / name, SOURCES / name)
    rows = []
    for tool_id, name, version, executable, license_name, url in (
        ('windirstat', 'WinDirStat 磁盘空间分析', '2.9.0', 'WinDirStat.exe', 'GPL-2.0', 'https://github.com/windirstat/windirstat'),
        ('peazip', 'PeaZip 压缩与解压', '11.3.0', 'peazip.exe', 'LGPL-3.0 / bundled component notices', 'https://github.com/peazip/PeaZip'),
    ):
        directory = DEST / tool_id
        files = {}
        for path in sorted(directory.rglob('*')):
            relative = path.relative_to(directory).as_posix()
            if path.is_file() and relative != 'WinDirStat.ini' and not relative.startswith('res/conf/'):
                files[relative] = digest(path)
        rows.append({'id': tool_id, 'name': name, 'version': version, 'platform': 'windows/amd64',
                     'directory': 'app/tools/windows-x64/' + tool_id, 'executable': executable,
                     'license': license_name, 'official_url': url, 'files': files})
    (ROOT / 'catalog/portable-tools.json').write_text(json.dumps(rows, ensure_ascii=False, indent=2) + '\n', encoding='utf-8')
    (SOURCES / 'UPSTREAM.json').write_text(json.dumps(ASSETS, ensure_ascii=False, indent=2), encoding='utf-8')
    print(json.dumps({'prepared': [r['name'] for r in rows], 'bytes': sum(p.stat().st_size for p in DEST.rglob('*') if p.is_file())}, ensure_ascii=False))


if __name__ == '__main__':
    main()
