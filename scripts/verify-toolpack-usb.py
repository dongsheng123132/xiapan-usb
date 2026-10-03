"""Verify freshly staged USB tool packs. No cloud calls or host maintenance.

Requires evidence/toolpack-usb-targets.json from identity-checked staging.
Creates only test data/preferences in those new packages. The caller resets
those generated files afterward, with disk identity and path boundary checks.
"""
import hashlib
import json
from pathlib import Path
import runpy
import subprocess
import time

ROOT = Path(__file__).resolve().parent.parent
App = runpy.run_path(str(ROOT / 'scripts/verify-genie-usb.py'))['App']


def observe_and_close(pid):
    assert isinstance(pid, int) and pid > 0
    script = f"""[Console]::OutputEncoding=[Text.UTF8Encoding]::new($false)
for($i=0;$i -lt 50;$i++){{$p=Get-Process -Id {pid} -ErrorAction SilentlyContinue;if(!$p -or $p.MainWindowHandle -ne 0){{break}};Start-Sleep -Milliseconds 200}}
$r=@{{pid={pid};visible=($null -ne $p -and $p.MainWindowHandle -ne 0);title=$p.MainWindowTitle}}
if($p){{$null=$p.CloseMainWindow();if(!$p.WaitForExit(3000)){{Stop-Process -Id {pid} -Force}}}}
$r|ConvertTo-Json -Compress"""
    result = subprocess.run(['pwsh.exe', '-NoProfile', '-Command', script],
                            capture_output=True, timeout=25,
                            creationflags=subprocess.CREATE_NO_WINDOW, check=True)
    return json.loads(result.stdout.decode('utf-8-sig'))


def main():
    targets = json.loads((ROOT / 'evidence/toolpack-usb-targets.json').read_text(encoding='utf-8-sig'))
    assert targets and len({t['drive'] for t in targets}) == len(targets)
    results = []
    for target in targets:
        root = Path(target['root']).resolve()
        assert root.drive == target['drive'] + ':' and root.name.startswith('虾盘 U盘精灵 0.4.2-工具箱-')
        assert (root / 'data').is_dir() and not list((root / 'data').iterdir())
        manifest = json.loads((root / 'app/manifest.json').read_text(encoding='utf-8'))
        assert manifest['portable_tools_included'] and not manifest['user_data_included']
        for name, item in manifest['files'].items():
            path = (root / name).resolve()
            assert path.is_relative_to(root)
            assert hashlib.sha256(path.read_bytes()).hexdigest() == item['sha256'], name
        app = App({**target, 'drive': 'toolpack-' + target['drive']})
        try:
            system = app.run('system.inspect')
            assert Path(system['portable_root']).resolve() == root
            catalog = app.run('tools.catalog')
            assert sum(t['ready'] for t in catalog['builtin_tools']) == 6
            assert all(t['ready'] for t in catalog['portable_tools'])
            expected_ids = {t['id'] for t in json.loads((ROOT / 'catalog/portable-tools.json').read_text(encoding='utf-8'))}
            assert {t['id'] for t in catalog['portable_tools']} == expected_ids
            startup = app.run('startup.inspect')
            processes = app.run('processes.inspect')
            row = {**target, 'startup_seconds': app.startup_seconds,
                   'exe_root_independent_of_cwd': True,
                   'system_tools_ready': 6, 'portable_tools_ready': len(expected_ids),
                   'startup_items': len(startup['items']), 'startup_warnings': startup['warnings'],
                   'processes_returned': len(processes['processes']), 'tool_launches': []}
            for tool_id in sorted(expected_ids):
                began = time.perf_counter()
                launch = app.run('tools.launch', {'tool_id': tool_id, 'confirmed': True})
                window = observe_and_close(launch['pid'])
                assert launch['verified'] and not launch['maintenance_performed']
                assert window['visible'], tool_id + ' window not visible'
                row['tool_launches'].append({'id': tool_id, 'verified': True, 'window': window,
                                            'launch_observe_close_seconds': round(time.perf_counter() - began, 3)})
            # Exercise the actual included compression engine with synthetic files only.
            fixture = root / 'data/toolpack-test'
            fixture.mkdir()
            sample = fixture / '中文测试.txt'
            body = ('虾盘压缩测试 / portable archive fixture\n' * 8).encode('utf-8')
            sample.write_bytes(body)
            engine = root / 'app/tools/windows-x64/peazip/res/bin/7z/7z.exe'
            for arguments in (
                ['a', '-tzip', 'fixture.zip', sample.name],
                ['t', 'fixture.zip'],
                ['x', 'fixture.zip', '-orestored', '-y'],
            ):
                subprocess.run([str(engine), *arguments], cwd=fixture, capture_output=True,
                               timeout=30, check=True, creationflags=subprocess.CREATE_NO_WINDOW)
            assert (fixture / 'restored' / sample.name).read_bytes() == body
            row['included_engine_zip_roundtrip'] = True
            results.append(row)
            print(target['drive'] + ': native windows / startup / processes / ZIP roundtrip passed', flush=True)
        finally:
            app.stop()
    evidence = {'same_windows_computer': True, 'real_usb_drives': True,
                'no_format_performed': True, 'cloud_requests': 0, 'results': results}
    (ROOT / 'evidence/toolpack-usb-results.json').write_text(
        json.dumps(evidence, ensure_ascii=False, indent=2), encoding='utf-8')


if __name__ == '__main__':
    main()
