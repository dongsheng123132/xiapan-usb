"""Test freshly staged physical USB packages with a localhost model fixture.

Input: evidence/genie-usb-targets.json from identity-checked staging. Every data/
directory must be empty. No formatting, registry writes, or original-data edits.
The caller must reset only these newly generated test data directories afterward.
"""
import argparse
import hashlib
import http.server
import json
import os
from pathlib import Path
import re
import shutil
import subprocess
import threading
import time
import urllib.request

ROOT = Path(__file__).resolve().parent.parent
HTTP = urllib.request.build_opener(urllib.request.ProxyHandler({}))
KEY = 'sk-usb-fixture-not-a-real-key'
OBSERVED = []


class Fixture(http.server.BaseHTTPRequestHandler):
    def log_message(self, *args):
        pass

    def do_POST(self):
        body = json.loads(self.rfile.read(int(self.headers['Content-Length'])))
        names = [t.get('function', {}).get('name') for t in body.get('tools', [])]
        OBSERVED.append({'model': body.get('model'), 'path': self.path,
                         'custom_key_used': self.headers.get('Authorization') == 'Bearer ' + KEY,
                         'tools': names})
        if 'connection_check' in names:
            function = {'name': 'connection_check', 'arguments': '{}'}
        elif not any(m.get('role') == 'tool' for m in body.get('messages', [])):
            function = {'name': 'mcp_xiapan_maintenance_action',
                        'arguments': json.dumps({'action': 'startup.inspect'})}
        else:
            function = None
        message = {'role': 'assistant'}
        if function:
            message['tool_calls'] = [{'id': 'usb-check', 'type': 'function', 'function': function}]
        else:
            message['content'] = 'U 盘原生 PicoClaw 已读取启动项，未修改系统。' + KEY
        payload = json.dumps({'id': 'usb-fixture', 'object': 'chat.completion',
                              'choices': [{'index': 0, 'message': message,
                                           'finish_reason': 'tool_calls' if function else 'stop'}],
                              'usage': {'prompt_tokens': 8, 'completion_tokens': 8, 'total_tokens': 16}}).encode()
        self.send_response(200)
        self.send_header('Content-Type', 'application/json')
        self.send_header('Content-Length', str(len(payload)))
        self.end_headers()
        self.wfile.write(payload)


class App:
    def __init__(self, target):
        self.target, self.root = target, Path(target['root']).resolve()
        self.base = self.token = None
        self.log = (ROOT / 'evidence' / f"genie-usb-{target['drive']}.log").open('w', encoding='utf-8')
        env = os.environ.copy()
        for name in list(env):
            if name.startswith('PICOCLAW_') or name == 'UCLAW_API_BASE_URL':
                env.pop(name)
        # Deliberately launch from the development directory with no --root.
        self.proc = subprocess.Popen([str(self.root / '虾盘.exe'), 'serve', '--no-open'],
                                     cwd=ROOT, env=env, stdout=self.log, stderr=self.log,
                                     creationflags=subprocess.CREATE_NO_WINDOW)
        started = time.perf_counter()
        for _ in range(160):
            text = Path(self.log.name).read_text(encoding='utf-8', errors='replace')
            match = re.search(r'http://127\.0\.0\.1:\d+', text)
            if match:
                self.base = match.group()
                try:
                    with HTTP.open(self.base, timeout=2) as response:
                        page = response.read().decode()
                    self.token = re.search(r'name="xiapan-token" content="([a-f0-9]+)"', page).group(1)
                    self.startup_seconds = round(time.perf_counter() - started, 3)
                    return
                except Exception:
                    pass
            if self.proc.poll() is not None:
                raise RuntimeError('USB executable exited before serving')
            time.sleep(.25)
        self.stop()
        raise RuntimeError('USB server startup timed out')

    def run(self, action, inputs=None):
        request = urllib.request.Request(self.base + '/api/run',
            data=json.dumps({'action': action, 'input': inputs or {}}).encode(),
            headers={'Content-Type': 'application/json', 'X-Xiapan-Token': self.token})
        with HTTP.open(request, timeout=75) as response:
            result = json.load(response)
        if not result['ok']:
            raise RuntimeError(action + ': ' + result.get('error', 'failed'))
        return result['data']

    def stop(self):
        if self.proc.poll() is None:
            try:
                req = urllib.request.Request(self.base + '/api/quit', data=b'',
                                             headers={'X-Xiapan-Token': self.token})
                HTTP.open(req, timeout=5).close()
                self.proc.wait(timeout=10)
            except Exception:
                self.proc.terminate()
                self.proc.wait(timeout=10)
        self.log.close()


def send(app, session, message):
    return app.run('chat.send', {'session_id': session['id'],
        'expected_version': session['version'], 'message': message, 'engine': 'pico'})


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument('--hold-for-ui', action='store_true')
    args = parser.parse_args()
    targets = json.loads((ROOT / 'evidence/genie-usb-targets.json').read_text(encoding='utf-8-sig'))
    assert len(targets) == 2 and len({Path(t['root']).drive for t in targets}) == 2
    for target in targets:
        root = Path(target['root']).resolve()
        assert root.drive == target['drive'] + ':' and root.name.startswith('虾盘 U盘精灵 0.4-')
        assert (root / 'data').is_dir() and not list((root / 'data').iterdir()), 'Requires fresh test data'
        manifest = json.loads((root / 'app/manifest.json').read_text(encoding='utf-8'))
        for name, item in manifest['files'].items():
            path = (root / name).resolve()
            assert path.is_relative_to(root)
            assert hashlib.sha256(path.read_bytes()).hexdigest() == item['sha256']
    server = http.server.ThreadingHTTPServer(('127.0.0.1', 0), Fixture)
    threading.Thread(target=server.serve_forever, daemon=True).start()
    apps, results, carry = [], [], None
    complete = ROOT / '.cache/genie-usb-ui-done.json'
    assert not complete.exists(), 'Remove stale UI completion marker first'
    try:
        for target in targets:
            root = Path(target['root'])
            if carry:
                shutil.copytree(carry[0] / 'data', root / 'data', dirs_exist_ok=True)
            app = App(target)
            apps.append(app)
            system = app.run('system.inspect')
            assert Path(system['portable_root']).resolve() == root.resolve()
            resources = app.run('resources.scan')
            assert any('app/runtime/picoclaw/' in f['path'].replace('\\', '/') for f in resources['files'])
            startup = app.run('startup.inspect')
            cli = subprocess.run([str(root / '虾盘.exe'), 'action', 'run', 'startup.inspect', '--json', '--no-input'],
                                 cwd=ROOT, capture_output=True, timeout=35)
            assert json.loads(cli.stdout)['data']['items'] == startup['items']
            row = {**target, 'startup_seconds': app.startup_seconds,
                   'exe_root_independent_of_cwd': True, 'startup_items': len(startup['items']),
                   'startup_warnings': startup['warnings'], 'cli_gui_same_startup_results': True}
            if carry:
                restored = app.run('chat.read', {'session_id': carry[1]['id']})
                assert restored == carry[1], 'History changed during physical copy'
                assert app.run('settings.model.get')['model'] == 'fixture/usb-model'
                row['history_and_model_carried_from_E_to_F'] = True
                app.run('settings.model.save', {'expected_revision': app.run('settings.model.get')['revision'],
                                               'source': 'cloud', 'model': 'deepseek-v4-flash', 'clear_api_key': True})
            current = app.run('settings.model.get')
            inputs = {'expected_revision': current['revision'], 'source': 'custom',
                      'base_url': f'http://127.0.0.1:{server.server_port}/v1',
                      'model': 'fixture/usb-model', 'api_key': KEY}
            assert app.run('settings.model.test', inputs)['tools_supported']
            assert app.run('settings.model.get')['revision'] == current['revision']
            app.run('settings.model.save', inputs)
            assert KEY not in json.dumps(app.run('settings.model.get'))
            session = restored if carry else app.run('chat.start')
            previous_message_count = len(session['messages'])
            session = send(app, session, '请执行本轮启动项只读验证。')
            assert any(m.get('action') == 'startup.inspect' for m in session['messages'][previous_message_count:])
            assert session['messages'][-1]['role'] == 'assistant'
            assert KEY not in json.dumps(session)
            assert not list((root / 'data/pico/runs').glob('*/.security.yml'))
            app.stop()
            restarted = App(target)
            apps.append(restarted)
            assert restarted.run('chat.read', {'session_id': session['id']}) == session
            assert restarted.run('settings.model.get')['model'] == 'fixture/usb-model'
            row.update({'custom_api_passed': True, 'native_startup_tool_call_passed': True,
                        'restart_preserves_settings_history': True, 'secret_redacted': True,
                        'ephemeral_secrets_cleaned': True, 'url': restarted.base,
                        'session_id': session['id'], 'pid': restarted.proc.pid})
            results.append(row)
            restarted.stop()
            carry = (root, session)
            print(target['drive'] + ': startup / custom API / native tools / restart passed', flush=True)
        assert all(r['custom_key_used'] and r['model'] == 'fixture/usb-model' for r in OBSERVED)
        evidence = {'same_windows_computer': True, 'real_usb_drives': True,
                    'no_format_performed': True, 'fixture_requests': len(OBSERVED), 'results': results}
        (ROOT / 'evidence/genie-usb-results.json').write_text(json.dumps(evidence, ensure_ascii=False, indent=2), encoding='utf-8')
        if args.hold_for_ui:
            for target, row in zip(targets, results):
                app = App(target)
                apps.append(app)
                row['url'], row['pid'] = app.base, app.proc.pid
            (ROOT / 'evidence/genie-usb-results.json').write_text(json.dumps(evidence, ensure_ascii=False, indent=2), encoding='utf-8')
            print(json.dumps({'ready_for_browser': [{k: r[k] for k in ('drive','url','pid')} for r in results]}, ensure_ascii=False), flush=True)
            deadline = time.monotonic() + 900
            while not complete.exists() and time.monotonic() < deadline:
                time.sleep(.5)
            assert complete.exists(), 'Browser verification completion timed out'
        print('Two physical USB packages passed.', flush=True)
    finally:
        for app in reversed(apps):
            app.stop()
        server.shutdown()
        server.server_close()


if __name__ == '__main__':
    main()
