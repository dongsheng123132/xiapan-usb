"""Stage pinned upstream maintenance tools without changing public catalogs.

Only writes .cache/customer-maintenance-assets. Never executes a maintenance
program, and never overwrites an existing ready directory or preferences.
"""
import argparse
import gzip
import hashlib
import io
import json
import lzma
from pathlib import Path
import shutil
import struct
import subprocess
import tarfile
import zipfile

ROOT = Path(__file__).resolve().parent.parent
BASE = ROOT / '.cache/customer-maintenance-assets'
DOWNLOADS = BASE / 'downloads'
DEST = BASE / 'ready'
ASSETS = [
    ('CrystalDiskInfo9_9_3.zip', 'https://downloads.sourceforge.net/project/crystaldiskinfo/9.9.3/CrystalDiskInfo9_9_3.zip', '364f40d9161175a4d8b797edc21f9e57c1671dbfb689aadf9d91449ca814015c'),
    ('CrystalDiskInfo9_9_3Src.zip', 'https://downloads.sourceforge.net/project/crystaldiskinfo/9.9.3/CrystalDiskInfo9_9_3Src.zip', '272b54323335e4fcb3871ccc21146935e9990c1516cfaa79a50bb0b3e553f921'),
    ('testdisk-7.2.win64.zip', 'https://www.cgsecurity.org/testdisk-7.2.win64.zip', 'e97e203ce77b6b1a3a37d01beccf069dc6c4632b579ffbb82ae739cdda229f38'),
    ('testdisk-7.2.tar.bz2', 'https://www.cgsecurity.org/testdisk-7.2.tar.bz2', 'f8343be20cb4001c5d91a2e3bcd918398f00ae6d8310894a5a9f2feb813c283f'),
]
CLI_KEEP = {
    'testdisk_win.exe', 'photorec_win.exe', 'fidentify_win.exe',
    'cyggcc_s-seh-1.dll', 'cygwin1.dll', 'cygewf-2.dll', 'cygiconv-2.dll',
    'cygjpeg-8.dll', 'cygncursesw-10.dll', 'cygz.dll',
    'readme.txt', 'VERSION.txt', 'COPYING.txt', 'NEWS.txt', 'THANKS.txt',
    'AUTHORS.txt', 'INFO', 'testdisk.pdf', 'documentation.html', '63/cygwin',
}
RECIPES = [
    ('cygwin', '79d6a849cd302ef1b7d73a4eaa42e60059c2e537'),
    ('cygwin-gcc', 'b74008dc56407c41260a147f899caaedb9214b7a'),
]
# Complete source packages, with versions verified from DLL resources/debug data.
DEPENDENCY_ASSETS = [
    ('cygwin64-libewf-20140608-1.fc33.src.rpm', 'https://download.copr.fedorainfracloud.org/results/grenier/cygwin-testdisk/fedora-42-x86_64/01353637-cygwin64-libewf/cygwin64-libewf-20140608-1.fc33.src.rpm', 'a84eaadb7aa13d9afbeeb7a94a14c678a447ec4fef0828a3e1669bfd6fbd7c2a'),
    ('cygwin-libjpeg-turbo-1.5.3-5.fc33.src.rpm', 'https://download.copr.fedorainfracloud.org/results/grenier/cygwin-testdisk/fedora-42-x86_64/01353621-cygwin-libjpeg-turbo/cygwin-libjpeg-turbo-1.5.3-5.fc33.src.rpm', 'faf0d82f4e2009765336ab5e91f70edee3bb125cfb1f5f2631763bdb44210f54'),
    ('cygwin-ncurses-6.1-1.20190727.fc33.src.rpm', 'https://download.copr.fedorainfracloud.org/results/yselkowitz/cygwin/fedora-42-x86_64/01351354-cygwin-ncurses/cygwin-ncurses-6.1-1.20190727.fc33.src.rpm', 'd76df621c82458ff2a6ed16913322c26e8a0a6b56b12edb0bc71e90bc7caa498'),
    ('cygwin-libiconv-1.16-1.fc36.src.rpm', 'https://download.copr.fedorainfracloud.org/results/yselkowitz/cygwin/fedora-42-x86_64/03137071-cygwin-libiconv/cygwin-libiconv-1.16-1.fc36.src.rpm', '759e5e704826ae0daf2ccefa0f16143483b4dce2129894983ff35111d57574d3'),
    ('cygwin-e2fsprogs-1.45.3-0.fc33.src.rpm', 'https://download.copr.fedorainfracloud.org/results/grenier/cygwin-testdisk/fedora-42-x86_64/01353624-cygwin-e2fsprogs/cygwin-e2fsprogs-1.45.3-0.fc33.src.rpm', '4d6e6dcee9f06140c8563dbd2170a2a6f0ad57632549216c486696f54ccd01da'),
    ('cygwin-ntfsprogs-2.0.0-0.fc33.src.rpm', 'https://download.copr.fedorainfracloud.org/results/grenier/cygwin-testdisk/fedora-42-x86_64/01353604-cygwin-ntfsprogs/cygwin-ntfsprogs-2.0.0-0.fc33.src.rpm', '547c83836cbb6d533ea6e12bf0b9694bba83f2d9c2dae34cab468f8ea3ee1033'),
    ('cygwin-zlib-1.2.11-2.fc36.src.rpm', 'https://download.copr.fedorainfracloud.org/results/yselkowitz/cygwin/fedora-42-x86_64/03139503-cygwin-zlib/cygwin-zlib-1.2.11-2.fc36.src.rpm', 'd6e00e615e9d1b7f28f46e638c5a3d44f955c942bbdbce750a8dfd6b61176e68'),
    ('newlib-cygwin-3.1.4.tar.bz2', 'https://copr-dist-git.fedorainfracloud.org/repo/pkgs/yselkowitz/cygwin/cygwin/newlib-cygwin-3.1.4.tar.bz2/md5/7eb65a47a09b4af757c5f66d318eceb0/newlib-cygwin-3.1.4.tar.bz2', 'a6c1ad3a6baef58ff438c59c60aa488304e1719fb20fbc1be27439be045185b4'),
    ('gcc-11.2.0.tar.xz', 'https://mirrors.aliyun.com/gnu/gcc/gcc-11.2.0/gcc-11.2.0.tar.xz', 'd08edc536b54c372a1010ff6619dd274c0f1603aa49212ba20f7aa2cda36fa8b'),
]


def digest(path):
    with path.open('rb') as stream:
        return hashlib.file_digest(stream, 'sha256').hexdigest()


def fetch(name, url, expected):
    target = DOWNLOADS / name
    if target.exists():
        if digest(target) != expected:
            raise ValueError(f'Existing archive failed SHA256: {target}')
        return target
    partial = target.with_suffix(target.suffix + '.part')
    subprocess.run(['curl.exe', '--fail', '--location', '--silent', '--show-error',
                    '--connect-timeout', '20', '--max-time', '300', '--retry', '1',
                    '--output', str(partial), url], check=True, timeout=640)
    if digest(partial) != expected:
        raise ValueError(f'Download failed SHA256: {partial}')
    partial.replace(target)
    return target


def extract(archive, target, prefix='', selected=None):
    with zipfile.ZipFile(archive) as z:
        if z.testzip() is not None:
            raise ValueError(f'Corrupt ZIP: {archive}')
        if sum(i.file_size for i in z.infolist()) > 300 * 1024 * 1024:
            raise ValueError('Unexpected uncompressed size')
        for item in z.infolist():
            if item.is_dir() or not item.filename.startswith(prefix):
                continue
            if selected is not None and item.filename[len(prefix):] not in selected:
                continue
            output = (target / item.filename[len(prefix):]).resolve()
            if not output.is_relative_to(target.resolve()) or output == target.resolve():
                raise ValueError('Archive path traversal')
            if (item.external_attr >> 16) & 0o170000 == 0o120000:
                raise ValueError('Archive symlink rejected')
            output.parent.mkdir(parents=True, exist_ok=True)
            with output.open('xb') as stream:
                stream.write(z.read(item))


def rpm_members(path):
    """Read source RPM payload without installing or running any package."""
    blob = path.read_bytes()
    def header_end(offset):
        if blob[offset:offset + 3] != b'\x8e\xad\xe8':
            raise ValueError('Invalid RPM header')
        count, length = struct.unpack_from('>II', blob, offset + 8)
        return offset + 16 + count * 16 + length
    offset = (header_end(96) + 7) // 8 * 8
    payload = blob[header_end(offset):]
    if payload.startswith(b'\x1f\x8b'):
        payload = gzip.decompress(payload)
    elif payload.startswith(b'\xfd7zXZ'):
        payload = lzma.decompress(payload)
    else:
        raise ValueError('Unsupported source RPM compression')
    offset = 0
    while offset < len(payload):
        if payload[offset:offset + 6] not in (b'070701', b'070702'):
            raise ValueError('Invalid source RPM CPIO')
        fields = [int(payload[offset + 6 + i * 8:offset + 14 + i * 8], 16) for i in range(13)]
        name = payload[offset + 110:offset + 110 + fields[11] - 1].decode()
        start = (offset + 110 + fields[11] + 3) // 4 * 4
        size = fields[6]
        if name == 'TRAILER!!!':
            break
        yield name, payload[start:start + size]
        offset = (start + size + 3) // 4 * 4


def source_notices(name, body, target):
    """Extract readable license notices; complete source archives remain intact."""
    try:
        with tarfile.open(fileobj=io.BytesIO(body), mode='r:*') as archive:
            for item in archive:
                leaf = Path(item.name).name.upper()
                if not item.isfile() or len(Path(item.name).parts) > 3:
                    continue
                if not (leaf.startswith(('COPYING', 'LICENSE', 'COPYRIGHT')) or leaf in ('README', 'README.TXT')):
                    continue
                output = (target / item.name).resolve()
                if not output.is_relative_to(target.resolve()):
                    raise ValueError('Unsafe notice path')
                output.parent.mkdir(parents=True, exist_ok=True)
                output.write_bytes(archive.extractfile(item).read())
    except tarfile.ReadError:
        raise ValueError(f'Invalid source archive: {name}') from None


def prepare_sources(sources, licenses):
    deps = sources / 'testdisk-dependencies'
    deps.mkdir()
    for name, url, expected in DEPENDENCY_ASSETS:
        archive = fetch(name, url, expected)
        shutil.copy2(archive, deps / name)
        notice_dir = licenses / 'testdisk-dependencies' / name
        if name.endswith('.rpm'):
            for member, body in rpm_members(archive):
                if member.endswith(('.gz', '.tgz', '.bz2', '.xz')):
                    source_notices(member, body, notice_dir)
        else:
            source_notices(name, archive.read_bytes(), notice_dir)
    for package, revision in RECIPES:
        checkout = BASE / 'source-history' / package
        if not checkout.exists():
            checkout.mkdir(parents=True)
            subprocess.run(['git', 'init', str(checkout)], check=True, timeout=20, capture_output=True)
            subprocess.run(['git', '-C', str(checkout), 'fetch', '--depth', '1',
                            f'https://copr-dist-git.fedorainfracloud.org/git/yselkowitz/cygwin/{package}.git',
                            revision], check=True, timeout=120, capture_output=True)
        archive = deps / (package + '-build-recipes-' + revision[:12] + '.zip')
        subprocess.run(['git', '-C', str(checkout), 'archive', '--format=zip',
                        '--output=' + str(archive.resolve()), revision], check=True, timeout=30)


def pe_imports(path):
    """Read PE import names without loading a DLL or executing a binary."""
    blob = path.read_bytes()
    pe = struct.unpack_from('<I', blob, 60)[0]
    count = struct.unpack_from('<H', blob, pe + 6)[0]
    optional_size = struct.unpack_from('<H', blob, pe + 20)[0]
    optional = pe + 24
    magic = struct.unpack_from('<H', blob, optional)[0]
    imports_rva = struct.unpack_from('<I', blob, optional + (112 if magic == 523 else 96) + 8)[0]
    sections = [struct.unpack_from('<8sIIII', blob, optional + optional_size + i * 40) for i in range(count)]
    def offset(rva):
        for _, virtual_size, address, raw_size, raw in sections:
            if address <= rva < address + max(virtual_size, raw_size):
                return raw + rva - address
        return rva
    ptr = offset(imports_rva)
    names = []
    while any(struct.unpack_from('<IIIII', blob, ptr)):
        name = offset(struct.unpack_from('<I', blob, ptr + 12)[0])
        names.append(blob[name:blob.index(b'\0', name)].decode())
        ptr += 20
    return names


def main():
    global BASE, DOWNLOADS, DEST
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--asset-root', type=Path, default=BASE)
    args = parser.parse_args()
    BASE = args.asset_root.resolve()
    DOWNLOADS = BASE / 'downloads'
    DEST = BASE / 'ready'
    if DEST.exists():
        raise SystemExit('ready already exists; refusing to overwrite tool settings')
    DOWNLOADS.mkdir(parents=True, exist_ok=True)
    for asset in [*ASSETS, *DEPENDENCY_ASSETS]:
        fetch(*asset)
    DEST.mkdir(parents=True)
    tools = DEST / 'tools'
    extract(DOWNLOADS / ASSETS[0][0], tools / 'crystaldiskinfo')
    extract(DOWNLOADS / ASSETS[2][0], tools / 'testdisk', 'testdisk-7.2/', CLI_KEEP)
    directory = tools / 'testdisk'
    imports = {p.name: pe_imports(p) for p in sorted(directory.iterdir()) if p.suffix in ('.exe', '.dll')}
    available = {p.name.lower() for p in directory.iterdir() if p.is_file()}
    for name, dependencies in imports.items():
        missing = [d for d in dependencies if d.lower() not in available | {'kernel32.dll', 'ntdll.dll'}]
        if missing:
            raise ValueError(f'Missing DLLs for {name}: {missing}')
    (DEST / 'IMPORT-CLOSURE.json').write_text(json.dumps({'imports': imports,
        'system_libraries': ['KERNEL32.dll', 'ntdll.dll'], 'missing_dependencies': []}, indent=2) + '\n', encoding='utf-8')
    sources = DEST / 'sources'
    sources.mkdir()
    for name in [ASSETS[1][0], ASSETS[3][0]]:
        shutil.copy2(DOWNLOADS / name, sources / name)
    licenses = DEST / 'licenses'
    shutil.copytree(tools / 'crystaldiskinfo/License', licenses / 'crystaldiskinfo')
    (licenses / 'testdisk').mkdir()
    for name in ['COPYING.txt', 'AUTHORS.txt', 'THANKS.txt', 'INFO']:
        shutil.copy2(tools / 'testdisk' / name, licenses / 'testdisk' / name)
    prepare_sources(sources, licenses)
    readme = '''离线附加维护工具（Windows）

一、硬盘健康：tools\\crystaldiskinfo\\DiskInfo64.exe
  查看硬盘/SSD 的 SMART 健康、温度、通电时间。先用它做只读检查。
  Windows 32 位用 DiskInfo32.exe；ARM64 用 DiskInfoA64.exe。
  不支持某个 USB 硬盘盒或 RAID 时，无法读取不等于硬盘健康。

二、文件恢复：tools\\testdisk\\photorec_win.exe
  键盘界面按文件特征恢复，结果通常不保留原文件名/目录。
  源盘是丢失数据的盘；恢复目的地必须选另一块有足够空间的健康盘。
  不要把恢复文件写回源盘，不要在源盘下载安装任何软件。

三、分区与文件系统分析：tools\\testdisk\\testdisk_win.exe
  需要管理员权限。先记录盘符、容量、型号和故障现象。
  不熟悉分区结构时不要使用 Write、修复引导或重建分区表。
  重要资料、异响、频繁掉盘：停止写入并交专业人员评估。

四、英文离线操作手册：tools\\testdisk\\testdisk.pdf
  全部工具本身不需要网络；系统更新、厂家专用驱动不在此目录。
  这些是 Windows 下运行的便携工具，不是启动盘/WinPE。

五、来源与开源材料
  UPSTREAM.json 记录官方下载地址和版本；SHA256SUMS.txt 是随盘文件校验。
  licenses 保留原版权声明；sources 带同版主程序、依赖源码与历史编译补丁。
  本次是 TestDisk / PhotoRec 命令行子集，不含 QPhotoRec 图形版。
  软件按原作者许可提供，不包含额外授权或效果保证。
'''
    (DEST / 'README.txt').write_text(readme, encoding='utf-8-sig')
    (sources / 'SOURCE-COVERAGE.txt').write_text('''Source coverage and redistribution notes

CrystalDiskInfo9_9_3Src.zip is the exact official 9.9.3 source archive.
testdisk-7.2.tar.bz2 is the exact official TestDisk / PhotoRec 7.2 source archive,
including its build scripts and GNU GPL text.

This distribution selects the unchanged TestDisk, PhotoRec and fidentify
command-line binaries and their seven Cygwin DLLs from the upstream ZIP.
QPhotoRec, Qt and the separate MinGW DLL chain are deliberately omitted.
Original application notices and the upstream PDF manual are retained.

The dependency sources in testdisk-dependencies include:
- Cygwin 3.1.4 original source and exact Fedora cross-build patches/recipe;
- GCC 11.2.0 original source and exact Fedora Cygwin 11.2.0-2 patches/recipe;
- libewf 20140608, libiconv 1.16, libjpeg-turbo 1.5.3,
  ncurses 6.1-20190727, zlib 1.2.11 complete source RPMs with build patches;
- e2fsprogs 1.45.3 and ntfsprogs 2.0.0 source RPMs for the application's
  filesystem libraries, from the author's documented cross-build repository.
Source RPMs are archives, not Windows installers. Their compressed CPIO
payload contains the original source tarball, .spec recipe, and patches.

The two recipe ZIPs are fixed historical commits of the official COPR
dist-git repositories. Their 'sources' files identify the original tarballs;
the downloaded GCC/Cygwin tarballs match those recorded digests.

This is a source-material inventory, not a claim of bit-for-bit rebuild
reproducibility. Preserve sources and license notices when copying the kit.

Upstream application source: https://www.cgsecurity.org/testdisk-7.2.tar.bz2
Build dependencies: https://www.cgsecurity.org/testdisk_doc/compilation_env.html
Official cross-build repositories: https://www.cgsecurity.org/testdisk_doc/crosscompilation_env.html
''', encoding='utf-8')
    upstream = {
        'prepared_on': '2026-10-12',
        'assets': [{'filename': n, 'url': u, 'sha256': h, 'bytes': (DOWNLOADS / n).stat().st_size}
                   for n, u, h in [*ASSETS, *DEPENDENCY_ASSETS]],
        'source_recipes': [{'package': p, 'revision': r,
                            'repository': f'https://copr-dist-git.fedorainfracloud.org/git/yselkowitz/cygwin/{p}.git'}
                           for p, r in RECIPES],
        'source_materials': [{'path': p.relative_to(DEST).as_posix(), 'bytes': p.stat().st_size, 'sha256': digest(p)}
                             for p in sorted(sources.rglob('*')) if p.is_file()],
        'tools': [
            {'name': 'CrystalDiskInfo', 'version': '9.9.3', 'license': 'MIT (preserve bundled third-party notices)',
             'official_url': 'https://crystalmark.info/en/download/', 'directory': 'tools/crystaldiskinfo',
             'entry': 'DiskInfo64.exe', 'authenticode_checked': 'Valid / CrystalMark Inc.',
             'distribution_status': 'official portable package and official application source included'},
            {'name': 'TestDisk / PhotoRec CLI', 'version': '7.2', 'license': 'GPL-2.0-or-later; bundled libraries have additional licenses',
             'official_url': 'https://www.cgsecurity.org/wiki/TestDisk_Download', 'directory': 'tools/testdisk',
             'entry': 'photorec_win.exe',
             'included_files': sorted(CLI_KEEP),
             'selection': 'Unmodified CLI executables, seven Cygwin DLLs, original notices, terminfo and PDF; Qt/QPhotoRec/MinGW chain omitted',
             'distribution_status': 'application and dependency sources, exact historical Cygwin/GCC patches, and notices included',
             'runtime_test': 'version invocation requires administrator elevation; no disk operations performed'},
        ],
        'verification': ['All application and dependency archives checked against pinned SHA256', 'Both binary ZIP archives passed CRC checks',
                         'Selected CLI binary files retain upstream hashes; original notices retained',
                         'No maintenance or disk-write actions executed'],
    }
    (DEST / 'UPSTREAM.json').write_text(json.dumps(upstream, ensure_ascii=False, indent=2) + '\n', encoding='utf-8')
    inventories = {}
    for key in ['crystaldiskinfo', 'testdisk']:
        directory = tools / key
        immutable = [p for p in sorted(directory.rglob('*')) if p.is_file()
                     and p.suffix.lower() not in ('.ini', '.log')]
        inventories[key] = {'bytes': sum(p.stat().st_size for p in immutable),
                            'files': {p.relative_to(directory).as_posix(): digest(p) for p in immutable},
                            'mutable_excluded': ['**/*.ini', '**/*.log']}
    (DEST / 'TOOL-FILES.json').write_text(json.dumps(inventories, ensure_ascii=False, indent=2) + '\n', encoding='utf-8')
    files = sorted(p for p in DEST.rglob('*') if p.is_file())
    (DEST / 'SHA256SUMS.txt').write_text(''.join(f'{digest(p)}  {p.relative_to(DEST).as_posix()}\n' for p in files), encoding='utf-8')
    print(json.dumps({'path': str(DEST), 'files': len(files) + 1, 'bytes': sum(p.stat().st_size for p in DEST.rglob('*') if p.is_file())}))


if __name__ == '__main__':
    main()
