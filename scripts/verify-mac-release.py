from pathlib import Path
import hashlib, json, os, re, shutil, signal, socket, subprocess, sys, tempfile, time, urllib.request
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from threading import Thread

def check_release_gateway(action, root, report):
    requests = []
    class Model(BaseHTTPRequestHandler):
        def log_message(self, *args): pass
        def reply(self, body):
            encoded=json.dumps(body).encode()
            self.send_response(200); self.send_header('Content-Type','application/json'); self.send_header('Content-Length',str(len(encoded)))
            self.end_headers(); self.wfile.write(encoded)
        def do_GET(self): self.reply({'object':'list','data':[{'id':'fixture-model','object':'model'}]})
        def do_POST(self):
            payload=json.loads(self.rfile.read(int(self.headers['Content-Length'])))
            requests.append({'auth':self.headers.get('Authorization'),'history':'fixture-ok' in json.dumps(payload.get('messages',[])),
                'write_tool':any(t.get('function',{}).get('name')=='write_file' for t in payload.get('tools',[]))})
            message={'role':'assistant','content':'fixture-ok'}; finish='stop'
            if len(requests)==1:
                message={'role':'assistant','content':'','tool_calls':[{'id':'release-write','type':'function','function':{'name':'write_file',
                    'arguments':json.dumps({'path':'release-fixture.txt','content':'release-native-write-ok'})}}]}; finish='tool_calls'
            self.reply({'id':'fixture','object':'chat.completion','model':'fixture-model','choices':[{'index':0,'message':message,'finish_reason':finish}],
                'usage':{'prompt_tokens':10,'completion_tokens':10,'total_tokens':20}})
    model=ThreadingHTTPServer(('127.0.0.1',0),Model)
    thread=Thread(target=model.serve_forever,daemon=True); thread.start()
    try:
        def select(key):
            settings=action('settings.model.get')
            action('settings.model.save',{'expected_revision':settings['revision'],'source':'custom','model':'fixture-model',
                'base_url':f'http://127.0.0.1:{model.server_port}/v1','api_key':key})
        select('fixture-release-1'); session=action('chat.start')
        def send(message):
            nonlocal session
            session=action('chat.send',{'session_id':session['id'],'expected_version':session['version'],'message':message,'engine':'pico','language':'en'})
            last=session['messages'][-1]
            assert last.get('content',last.get('text'))=='fixture-ok',last
        send('Write the fixture file, then confirm.')
        assert (root/'data/pico/workspace/release-fixture.txt').read_text()=='release-native-write-ok'
        old_pid=json.loads((root/'data/pico/.picoclaw.pid').read_text())['pid']
        select('fixture-release-2')
        new_pid=json.loads((root/'data/pico/.picoclaw.pid').read_text())['pid']
        assert old_pid!=new_pid and action('pico.status')['running'] is True
        send('Continue the same conversation after updating the credential.')
        assert requests[-1]['auth']=='Bearer fixture-release-2' and requests[-1]['history']
        assert any(req['write_tool'] for req in requests)
        report['checks'].update({'release_native_write':True,'release_credential_restart':True,'release_shared_history':True})
        report['fixture_model_requests']=len(requests)
    finally:
        model.shutdown(); model.server_close(); thread.join(timeout=3)

def check_file_verifier(root, checks):
    # Exercise the issued verifier on disposable files, including failure paths.
    with tempfile.TemporaryDirectory(prefix='校验 样本 ',dir=root/'acceptance') as directory:
        sample=Path(directory)
        verifier=sample/'Verify Files Mac.command'
        shutil.copyfile(root/'Verify Files Mac.command',verifier)
        document=sample/'中文 文件.txt'; document.write_text('fixture')
        (sample/'data').mkdir(); (sample/'data/private.txt').write_text('excluded fixture')
        sums=''.join(hashlib.sha256(path.read_bytes()).hexdigest()+'  '+path.name+'\n' for path in (verifier,document))
        (sample/'SHA256SUMS').write_text(sums)
        def verify():
            return subprocess.run(['/bin/sh',str(verifier)],cwd='/',capture_output=True,text=True,timeout=15)
        assert verify().returncode==0
        (sample/'SHA256SUMS').write_bytes(sums.replace('\n','\r\n').encode())
        assert verify().returncode==0
        (sample/'data/private.txt').write_text('changed excluded fixture')
        assert verify().returncode==0
        document.write_text('corrupted fixture'); assert verify().returncode!=0
        document.unlink(); assert verify().returncode!=0
    checks.update({'mac_verifier_valid_files':True,'mac_verifier_accepts_crlf':True,'mac_verifier_excludes_user_data':True,
        'mac_verifier_rejects_corruption':True,'mac_verifier_rejects_missing_file':True})

root = Path(sys.argv[1]).resolve()
assert not (root/'data').exists() or not any((root/'data').iterdir()), 'Use an empty, isolated acceptance root'

report = {'root': str(root), 'platform': 'darwin/arm64', 'paid_model_calls': 0,
          'measurement': 'Process creation to complete HTTP homepage; OS file cache not cleared; excludes browser drawing, Gateway and model latency',
          'checks': {}, 'startup_ms': []}
checks = report['checks']
manifest = json.loads((root/'acceptance/manifest.json').read_text())
report['release'] = manifest['release']
for name, expected in manifest['files'].items():
    assert hashlib.sha256((root/name).read_bytes()).hexdigest() == expected, name
checks['transfer_hashes'] = True
check_file_verifier(root,checks)
checks['chinese_space_path'] = ' ' in str(root) and '虾盘' in str(root)
program = root/'app/bin/xiapan-macos-arm64'
program.chmod(0o700)
report['actual_version'] = subprocess.check_output([str(program), '--version'], timeout=15, text=True).strip()
opener = urllib.request.build_opener(urllib.request.ProxyHandler({}))
def process_table():
    out = subprocess.check_output(['ps', '-axo', 'pid=,comm='], timeout=5, text=True)
    result = {}
    for line in out.splitlines():
        items = line.strip().split(None, 1)
        if len(items) == 2:
            result[int(items[0])] = items[1]
    return result

try:
    with tempfile.TemporaryDirectory(prefix='xiapan-mac-acceptance-') as scratch:
        scratch = Path(scratch)
        launcher = root/'acceptance-start.command'
        original = (root/'Start Mac.command').read_text()
        # Keep the issued bootstrap logic and checksum; redirect reproducible cache to a disposable test directory.
        patched = original.replace('CACHE="$HOME/Library/Caches/Xiapan/bootstrap"', f'CACHE="{scratch}/bootstrap"')
        assert patched != original
        env = os.environ.copy()
        env['XIAPAN_RUNTIME_CACHE_DIR'] = str(scratch/'pico-cache')
        for sample in range(3):
            with socket.socket() as sock:
                sock.bind(('127.0.0.1', 0)); port = sock.getsockname()[1]
            launcher.write_text(patched.replace('exec "$DEST" --root "$ROOT"', f'exec "$DEST" --root "$ROOT" --no-open --port {port}'))
            launcher.chmod(0o700)
            base = f'http://127.0.0.1:{port}'
            log = (root/'acceptance'/f'bootstrap-{sample}.log').open('w')
            started = time.perf_counter()
            proc = subprocess.Popen(['/bin/sh', str(launcher)], env=env, stdout=log, stderr=log)
            try:
                deadline = time.monotonic()+30
                while True:
                    try:
                        page = opener.open(base+'/', timeout=.25).read().decode()
                        break
                    except OSError:
                        if proc.poll() is not None or time.monotonic()>deadline:
                            raise RuntimeError(f'Homepage did not start; sample {sample}; process {proc.poll()}')
                        time.sleep(.01)
                report['startup_ms'].append(round((time.perf_counter()-started)*1000, 2))
                assert 'name="xiapan-platform" content="darwin"' in page
                assert 'name="xiapan-arch" content="arm64"' in page
                assert f'name="xiapan-version" content="{report["actual_version"]}"' in page
                checks['native_platform_metadata'] = True
                token = re.search(r'name="xiapan-token" content="([a-f0-9]+)"', page).group(1)
                def action(name, value=None):
                    req = urllib.request.Request(base+'/api/run', data=json.dumps({'action':name,'input':value or {}}).encode(),
                            headers={'Content-Type':'application/json','X-Xiapan-Token':token})
                    with opener.open(req, timeout=30) as response:
                        result = json.load(response)
                    if not result['ok']:
                        raise RuntimeError(f'{name}: {result.get("error")}')
                    return result['data']
                assert action('pico.status')['running'] is False
                checks['lazy_gateway'] = True
                assert action('runtime.cache.status')['status'] == 'not_used'
                assert action('maintenance.plan', {'message':'断网了怎么办'})['mode']=='guide'
                checks['offline_local_guide'] = True
                plan=action('maintenance.plan', {'message':'电脑无法启动'})
                assert plan['suggested_action']=='tools.catalog' and 'macOS 恢复' in plan['summary']
                checks['mac_recovery_guidance'] = True
                network=action('drivers.inspect')
                assert network['platform']=='darwin' and network['interfaces']['interfaces']
                assert '设备管理器' not in ''.join(network['next_steps']) and '硬件 ID' not in ''.join(network['next_steps'])
                checks['mac_network_interfaces'] = True
                report['network_interface_count']=len(network['interfaces']['interfaces'])
                system=action('system.inspect')
                assert system['os']=='darwin' and system['memory_bytes']>0
                assert '未读取可用内存' in ''.join(system['notes'])
                processes=action('processes.inspect')
                assert processes['processes'] and any(row['memory_bytes']>0 for row in processes['processes'])
                checks['mac_readonly_diagnostics'] = True
                catalog = action('tools.catalog')
                tools = catalog['builtin_tools']
                assert len(tools)==5 and all(t['platform']=='darwin' and t['ready'] for t in tools)
                assert not catalog['portable_tools'] and not catalog['library_tools'] and not catalog['resources']
                checks['mac_catalog_excludes_windows'] = True
                report['mac_tools'] = [{k:t[k] for k in ('id','name','ready')} for t in tools]
                checks['five_mac_tools_ready'] = True
                for src in re.findall(r'<script[^>]+src="([^"]+)"', page):
                    if not src.startswith(('http:','https:')):
                        assert opener.open(base+'/'+src.lstrip('/'), timeout=5).status == 200
                checks['homepage_and_scripts_http_200'] = True
                assert not list((root/'data').rglob('device.json')) if (root/'data').exists() else True
                checks['no_wallet_created'] = True
                if sample==2 and os.getenv('XIAPAN_RELEASE_GATEWAY_CHECK')=='1':
                    check_release_gateway(action,root,report)
                if sample==0:
                    before = process_table()
                    opened = action('tools.launch', {'tool_id':'mac-activity-monitor','confirmed':True})
                    assert opened['launch_requested'] and opened['maintenance_performed'] is False
                    deadline = time.monotonic()+8
                    while time.monotonic()<deadline:
                        after = process_table()
                        found = {pid:comm for pid,comm in after.items() if comm.endswith('/Activity Monitor.app/Contents/MacOS/Activity Monitor')}
                        if found: break
                        time.sleep(.1)
                    assert found, 'Activity Monitor did not appear in process table'
                    checks['activity_monitor_actual_launch'] = True
                    for pid in found:
                        if pid not in before:
                            try: os.kill(pid, signal.SIGTERM)
                            except ProcessLookupError: pass
                req = urllib.request.Request(base+'/api/quit', data=b'', headers={'X-Xiapan-Token':token})
                opener.open(req, timeout=5).read()
                proc.wait(timeout=10)
                assert proc.returncode==0
                checks['explicit_exit'] = True
                with socket.socket() as sock:
                    assert sock.connect_ex(('127.0.0.1',port))!=0
                checks['listener_cleanup'] = True
            finally:
                if proc.poll() is None:
                    proc.terminate(); proc.wait(timeout=10)
                log.close()
        launcher.unlink()
    report['ok'] = True
except Exception as error:
    report['ok'] = False
    report['error'] = str(error)
    raise
finally:
    (root/'acceptance/smoke-report.json').write_text(json.dumps(report, ensure_ascii=False, indent=2)+'\n')
    print(json.dumps(report, ensure_ascii=False, indent=2), flush=True)
