"""Prepare catalog/tool-packages.json into a fresh, reproducible portable library."""
import fnmatch
import hashlib
import json
from pathlib import Path
import shutil
import urllib.request
import zipfile

ROOT = Path(__file__).resolve().parent.parent
CACHE = ROOT / '.cache/portable-tool-downloads'
DEST = ROOT / 'runtime/tool-library/windows-x64'
SOURCES = ROOT / 'runtime/tool-library/sources'


def digest(path):
    with path.open('rb') as stream:
        return hashlib.file_digest(stream, 'sha256').hexdigest()


def fetch(asset):
    path = CACHE / asset['filename']
    assert path.parent == CACHE and path.name == asset['filename']
    if not path.exists():
        req = urllib.request.Request(asset['url'], headers={'User-Agent': 'Xiapan-portable-tools'})
        with urllib.request.urlopen(req, timeout=45) as response:
            body = response.read(100 * 1024 * 1024 + 1)
        assert len(body) <= 100 * 1024 * 1024
        assert hashlib.sha256(body).hexdigest() == asset['sha256']
        path.write_bytes(body)
    assert digest(path) == asset['sha256'], asset['filename']
    return path


def extract(archive, target, prefix):
    with zipfile.ZipFile(archive) as z:
        assert sum(i.file_size for i in z.infolist()) < 180 * 1024 * 1024
        for item in z.infolist():
            if not item.filename.startswith(prefix) or item.is_dir():
                continue
            path = (target / item.filename[len(prefix):]).resolve()
            assert path.is_relative_to(target.resolve()) and path != target.resolve()
            assert (item.external_attr >> 16) & 0o170000 != 0o120000
            path.parent.mkdir(parents=True, exist_ok=True)
            with path.open('xb') as f:
                f.write(z.read(item))


def main():
    specs = json.loads((ROOT / 'catalog/tool-packages.json').read_text(encoding='utf-8'))
    CACHE.mkdir(parents=True, exist_ok=True)
    if DEST.exists() or SOURCES.exists():
        raise SystemExit('Use fresh staging; refusing to overwrite existing portable preferences.')
    for spec in specs:
        for asset in [spec['archive'], *spec.get('extra_files', []), *([spec['source_archive']] if 'source_archive' in spec else [])]:
            fetch(asset)
    DEST.mkdir(parents=True)
    SOURCES.mkdir(parents=True)
    rows = []
    for spec in specs:
        directory = DEST / spec['id']
        extract(fetch(spec['archive']), directory, spec.get('prefix', ''))
        for name, body in spec.get('init_files', {}).items():
            path = (directory / name).resolve()
            assert path.is_relative_to(directory.resolve())
            path.parent.mkdir(parents=True, exist_ok=True)
            if not path.exists():
                path.write_text(body, encoding='utf-8')
        for asset in spec.get('extra_files', []):
            shutil.copy2(fetch(asset), directory / asset['filename'])
        if 'source_archive' in spec:
            source = fetch(spec['source_archive'])
            shutil.copy2(source, SOURCES / source.name)
            with zipfile.ZipFile(source) as z:
                for name in spec.get('license_from_source', []):
                    member = next(n for n in z.namelist() if n.count('/') == 1 and n.endswith('/' + name))
                    (directory / name).write_bytes(z.read(member))
        assert (directory / spec['executable']).is_file()
        row = {k: spec[k] for k in ('id', 'name', 'version', 'platform', 'executable', 'category',
                                   'description', 'license', 'official_url', 'portable_note', 'init_files', 'args')}
        row['directory'] = 'app/tools/windows-x64/' + spec['id']
        row['files'] = {p.relative_to(directory).as_posix(): digest(p) for p in sorted(directory.rglob('*'))
                        if p.is_file() and not any(fnmatch.fnmatchcase(p.relative_to(directory).as_posix(), pat)
                                                  for pat in spec['mutable'])}
        row['bytes'] = sum(p.stat().st_size for p in directory.rglob('*') if p.is_file())
        rows.append(row)
    (ROOT / 'catalog/portable-tools.json').write_text(json.dumps(rows, ensure_ascii=False, indent=2) + '\n', encoding='utf-8')
    (SOURCES / 'UPSTREAM.json').write_text(json.dumps(specs, ensure_ascii=False, indent=2), encoding='utf-8')
    print(json.dumps({'prepared': [r['name'] for r in rows], 'bytes': sum(r['bytes'] for r in rows)}, ensure_ascii=False))


if __name__ == '__main__':
    main()
