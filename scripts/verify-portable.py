"""Verify a shipping master without a real wallet, model request, or system repair."""
import argparse
import hashlib
import json
from pathlib import Path
import re
import socket
import statistics
import subprocess
import tempfile
import time
import urllib.error
import urllib.request


def digest(path):
    with path.open('rb') as stream:
        return hashlib.file_digest(stream, 'sha256').hexdigest()


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument('master', type=Path)
    parser.add_argument('--output', type=Path, required=True)
    parser.add_argument('--samples', type=int, default=7)
    parser.add_argument('--storage-note', default='Storage not classified; record the tested machine and medium separately')
    args = parser.parse_args()
    master = args.master.resolve()
    if args.samples < 1:
        raise ValueError('At least one startup sample is required')
    if args.output.resolve().is_relative_to(master):
        raise ValueError('Verification evidence must stay outside the shipping master')
    sums = (master / 'SHA256SUMS').read_text(encoding='utf-8').splitlines()
    listed = set()
    for line in sums:
        expected, relative = line.split('  ', 1)
        path = master / relative
        assert path.resolve().is_relative_to(master) and path.relative_to(master).as_posix() == relative, relative
        assert relative.casefold() not in listed, f'Duplicate checksum entry: {relative}'
        listed.add(relative.casefold())
        assert not path.is_symlink(), relative
        assert digest(path) == expected, relative
    actual = set()
    for path in master.rglob('*'):
        assert not path.is_symlink(), f'Linked shipping resource: {path}'
        if path.is_file():
            actual.add(path.relative_to(master).as_posix().casefold())
    assert actual == listed | {'sha256sums'}, 'Shipping file set differs from the checksum manifest'
    assert not list((master / 'data').iterdir()), 'Shipping data directory must be empty'
    info = json.loads((master / 'app/manifest.json').read_text(encoding='utf-8'))
    assert not info['customer_data_included'] and not info['local_model_included']
    assert not info['automatic_program_updates']
    durations = []
    opener = urllib.request.build_opener(urllib.request.ProxyHandler({}))
    checks = {'checksums': len(sums), 'empty_shipping_data': True}
    with tempfile.TemporaryDirectory(prefix='xiapan-verify-') as scratch:
        for index in range(args.samples):
            root = Path(scratch) / str(index)
            root.mkdir()
            with socket.socket() as sock:
                sock.bind(('127.0.0.1', 0))
                port = sock.getsockname()[1]
            base = f'http://127.0.0.1:{port}'
            started = time.perf_counter()
            proc = subprocess.Popen([str(master / '虾盘.exe'), 'serve', '--root', str(root), '--port', str(port), '--no-open'],
                                    stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL,
                                    creationflags=getattr(subprocess, 'CREATE_NO_WINDOW', 0))
            try:
                deadline = time.monotonic() + 15
                while True:
                    try:
                        page = opener.open(base + '/', timeout=.1).read().decode()
                        break
                    except (OSError, urllib.error.URLError):
                        if time.monotonic() > deadline or proc.poll() is not None:
                            raise RuntimeError('Application did not serve its local page')
                        time.sleep(.005)
                durations.append(round((time.perf_counter() - started) * 1000, 2))
                token = re.search(r'name="xiapan-token" content="([a-f0-9]+)"', page).group(1)

                def action(action_id, value=None):
                    body = json.dumps({'action': action_id, 'input': value or {}}).encode()
                    req = urllib.request.Request(base + '/api/run', data=body, headers={'Content-Type': 'application/json', 'X-Xiapan-Token': token})
                    with opener.open(req, timeout=15) as response:
                        return json.load(response)

                status = action('pico.status')
                assert status['ok'] and status['data']['running'] is False
                assert action('runtime.cache.status')['ok']
                assert action('maintenance.plan', {'message': '断网了怎么办'})['data']['mode'] == 'guide'
                assert not action('models.start', {'confirmed': True})['ok']
                catalog = action('tools.catalog')
                assert catalog['ok'] and len(catalog['data']['portable_tools']) == 9
                assert not list(root.rglob('device.json')), 'Read-only startup created a wallet'
                checks.update({'lazy_gateway': True, 'offline_guide': True, 'retired_local_model': True,
                               'nine_portable_tools_registered': True, 'no_wallet_created': True})
                req = urllib.request.Request(base + '/api/quit', data=b'', headers={'X-Xiapan-Token': token})
                opener.open(req, timeout=10).read()
                proc.wait(timeout=12)
                checks['explicit_exit'] = True
            finally:
                if proc.poll() is None:
                    proc.terminate()
                    proc.wait(timeout=10)
    result = {'version': info['version'], 'source_commit': info['source_commit'], 'master': str(master),
              'measurement': 'process creation to complete HTTP homepage; excludes browser rendering, Gateway and model latency',
              'storage_note': args.storage_note,
              'cache_note': 'Checksums read before timing; OS file caches are not cleared. This is not a cold-boot benchmark.',
              'startup_ms': durations, 'median_ms': statistics.median(durations), 'max_ms': max(durations),
              'paid_model_calls': 0, 'checks': checks}
    args.output.parent.mkdir(parents=True, exist_ok=True)
    args.output.write_text(json.dumps(result, ensure_ascii=False, indent=2)+'\n', encoding='utf-8')
    print(json.dumps(result, ensure_ascii=False, indent=2))


if __name__ == '__main__':
    main()
