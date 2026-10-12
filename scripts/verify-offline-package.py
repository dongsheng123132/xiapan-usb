"""Exercise a copied customer master with its real Windows executable and model.

Usage: python scripts/verify-offline-package.py --master <customer-master> [--qa-smoke]
Use repeated --question arguments for custom independent conversations.
Only the disposable copy is launched. The master must have an empty data/.
This is a loopback + invalid-proxy test, NOT a physically disconnected PC test.
No third-party Python modules, cloud API calls, firewall changes or repairs.
"""
from __future__ import annotations

import argparse
import ctypes
from ctypes import wintypes
from datetime import datetime, timezone
import hashlib
from html.parser import HTMLParser
import json
import os
from pathlib import Path
import re
import shutil
import socket
import subprocess
import tempfile
import time
import urllib.error
import urllib.parse
import urllib.request


ROOT = Path(__file__).resolve().parent.parent
QUESTION = '请用简短的话解释 SMART 健康信息能说明什么、不能说明什么。不要给操作步骤。'
QA_QUESTIONS = [
    '现在完全没有网络，插上网线仍不能上网。怎样先判断是网线、路由器还是网卡驱动的问题？请给三条安全检查建议，不要执行修复。',
    '现在电脑硬盘发出咔哒异响，几个重要文件也不见了。电脑没有联网，我应该先做什么？请给三条安全建议，不要执行命令或修复。',
    '只有 Codex 提示“无法加载组织设置”，其他网站都能打开。这等于电脑完全断网吗？我应该先检查什么？请给三条安全建议，不要执行命令、不要重置网络。',
    QUESTION,
]
MODEL_SUFFIX = '本地小模型参考随盘维护知识回答，请结合实际检测核实。'
KNOWLEDGE_MARKER = '维护知识指引（未调用模型）'


def require(condition, message):
    if not condition:
        raise RuntimeError(message)


def digest(path):
    with path.open('rb') as stream:
        return hashlib.file_digest(stream, 'sha256').hexdigest()


def safe_text(value):
    """Do not retain localhost capability tokens, including future log formats."""
    value = re.sub(r'(?i)(token|api[_-]?key)([= :]+)[^\s"&<>]+', r'\1\2[REDACTED]', str(value))
    return re.sub(r'\b[0-9a-fA-F]{64}\b', '[REDACTED_64_HEX]', value)


def read_json(path):
    return json.loads(path.read_text(encoding='utf-8-sig'))


def inside(root, relative):
    path = (root / relative).resolve()
    require(path.is_relative_to(root), 'Manifest path escapes package: ' + str(relative))
    return path


def empty_data(master):
    path = master / 'data'
    return path.is_dir() and not any(path.iterdir())


def resource_path(root, relative):
    direct = inside(root, relative)
    return direct if direct.is_file() else inside(root, 'app/' + relative)


def verify_manifest(root):
    manifest = read_json(root / 'app/manifest.json')
    files = manifest.get('files', {})
    require(files, 'Package manifest has no files')
    checked_bytes = 0
    for relative, expected in files.items():
        path = inside(root, relative)
        require(path.is_file(), 'Missing manifest file: ' + relative)
        require(path.stat().st_size == expected['bytes'], 'Size mismatch: ' + relative)
        require(digest(path) == expected['sha256'], 'SHA256 mismatch: ' + relative)
        checked_bytes += expected['bytes']
    return {'files': len(files), 'bytes': checked_bytes}, manifest


def isolated_env(run_dir):
    env = dict(os.environ)
    # This child must not inherit the developer's provider credentials.
    for key in list(env):
        upper = key.upper()
        if (re.search(r'(API.?KEY|TOKEN|SECRET|PASSWORD|CREDENTIAL)', upper)
                or upper.startswith(('XIAPAN_', 'UCLAW_', 'PICOCLAW_', 'LLAMA_', 'OPENAI_', 'ANTHROPIC_'))):
            env.pop(key, None)
    home = run_dir / 'test-home'
    for name in ('home', 'local', 'roaming', 'temp'):
        (home / name).mkdir(parents=True, exist_ok=True)
    env.update({'HOME': str(home / 'home'), 'USERPROFILE': str(home / 'home'),
                'LOCALAPPDATA': str(home / 'local'), 'APPDATA': str(home / 'roaming'),
                'TEMP': str(home / 'temp'), 'TMP': str(home / 'temp'),
                'HTTP_PROXY': 'http://127.0.0.1:1', 'HTTPS_PROXY': 'http://127.0.0.1:1',
                'ALL_PROXY': 'http://127.0.0.1:1',
                'NO_PROXY': '127.0.0.1,localhost,::1'})
    for key in ('HTTP_PROXY', 'HTTPS_PROXY', 'ALL_PROXY', 'NO_PROXY'):
        env[key.lower()] = env[key]
    return env


class ProcessJob:
    """Kill-on-close Windows job: cleanup is limited to our own process tree."""
    def __init__(self):
        require(os.name == 'nt', 'This verifier requires Windows and the real Windows package')
        self.kernel = ctypes.WinDLL('kernel32', use_last_error=True)
        self.kernel.CreateJobObjectW.argtypes = [ctypes.c_void_p, wintypes.LPCWSTR]
        self.kernel.CreateJobObjectW.restype = wintypes.HANDLE
        self.kernel.SetInformationJobObject.argtypes = [wintypes.HANDLE, ctypes.c_int, ctypes.c_void_p, wintypes.DWORD]
        self.kernel.SetInformationJobObject.restype = wintypes.BOOL
        self.kernel.AssignProcessToJobObject.argtypes = [wintypes.HANDLE, wintypes.HANDLE]
        self.kernel.AssignProcessToJobObject.restype = wintypes.BOOL
        self.kernel.CloseHandle.argtypes = [wintypes.HANDLE]
        self.kernel.CloseHandle.restype = wintypes.BOOL

        class Basic(ctypes.Structure):
            _fields_ = [('process_time', ctypes.c_int64), ('job_time', ctypes.c_int64),
                        ('flags', wintypes.DWORD), ('min_ws', ctypes.c_size_t),
                        ('max_ws', ctypes.c_size_t), ('active_limit', wintypes.DWORD),
                        ('affinity', ctypes.c_size_t), ('priority', wintypes.DWORD),
                        ('scheduling', wintypes.DWORD)]

        class IO(ctypes.Structure):
            _fields_ = [(name, ctypes.c_uint64) for name in
                        ('read_ops', 'write_ops', 'other_ops', 'read_bytes', 'write_bytes', 'other_bytes')]

        class Extended(ctypes.Structure):
            _fields_ = [('basic', Basic), ('io', IO), ('process_memory', ctypes.c_size_t),
                        ('job_memory', ctypes.c_size_t), ('peak_process', ctypes.c_size_t),
                        ('peak_job', ctypes.c_size_t)]

        self.handle = self.kernel.CreateJobObjectW(None, None)
        require(self.handle, 'Cannot create cleanup job: ' + str(ctypes.get_last_error()))
        limits = Extended()
        limits.basic.flags = 0x2000  # JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE
        if not self.kernel.SetInformationJobObject(self.handle, 9, ctypes.byref(limits), ctypes.sizeof(limits)):
            self.close()
            raise RuntimeError('Cannot set cleanup job limits: ' + str(ctypes.get_last_error()))

    def attach(self, process):
        require(self.kernel.AssignProcessToJobObject(self.handle, wintypes.HANDLE(process._handle)),
                'Cannot attach test process to cleanup job: ' + str(ctypes.get_last_error()))

    def close(self):
        if self.handle:
            self.kernel.CloseHandle(self.handle)
            self.handle = None


class TokenParser(HTMLParser):
    token = None

    def handle_starttag(self, tag, attrs):
        attrs = dict(attrs)
        if tag == 'meta' and attrs.get('name') == 'xiapan-token':
            self.token = attrs.get('content')


class Server:
    def __init__(self, package, run_dir, env, label):
        self.package, self.run_dir, self.env, self.label = package, run_dir, env, label
        self.process = self.job = self.log = None
        self.base = self.token = None
        self.log_path = run_dir / (label + '-server.log')
        self.opener = urllib.request.build_opener(urllib.request.ProxyHandler({}))
        self.calls = []
        self.cleanup = {}

    def start(self):
        self.job = ProcessJob()
        self.log = self.log_path.open('wb')
        self.process = subprocess.Popen([str(self.package / '虾盘.exe'), 'serve', '--root',
                                         str(self.package), '--port', '0', '--no-open'],
                                        cwd=self.package, env=self.env, stdout=self.log, stderr=self.log,
                                        creationflags=subprocess.CREATE_NO_WINDOW)
        self.job.attach(self.process)
        deadline = time.monotonic() + 25
        while time.monotonic() < deadline:
            text = self.log_path.read_text('utf-8', errors='replace')
            match = re.search(r'http://127\.0\.0\.1:\d+', text)
            if match:
                self.base = match.group(0)
                page = self.request('/', timeout=10, raw=True)
                parser = TokenParser()
                parser.feed(page)
                require(parser.token and re.fullmatch(r'[0-9a-f]{64}', parser.token),
                        'Homepage did not provide a valid local session token')
                self.token = parser.token
                return
            require(self.process.poll() is None, 'Server exited: ' + safe_text(text[-3000:]))
            time.sleep(0.1)
        raise RuntimeError('Timed out waiting for local server URL')

    def request(self, path, payload=None, timeout=70, raw=False):
        url = urllib.parse.urlparse(self.base + path)
        require(url.scheme == 'http' and url.hostname == '127.0.0.1', 'Non-loopback test request rejected')
        headers = {'Content-Type': 'application/json'}
        if self.token:
            headers['X-Xiapan-Token'] = self.token
        request = urllib.request.Request(url.geturl(),
                                        None if payload is None else json.dumps(payload).encode('utf-8'),
                                        headers=headers)
        with self.opener.open(request, timeout=timeout) as response:
            content = response.read(4 * 1024 * 1024).decode('utf-8')
        return content if raw else json.loads(content)

    def action(self, name, inputs=None):
        start = time.monotonic()
        timeout = 260 if name in ('models.start', 'chat.send') else 70
        reply = self.request('/api/run', {'action': name, 'input': inputs or {}}, timeout=timeout)
        self.calls.append({'action': name, 'seconds': round(time.monotonic() - start, 3),
                           'ok': reply.get('ok'), 'error': safe_text(reply.get('error', ''))})
        require(reply.get('ok') is True, name + ': ' + safe_text(reply.get('error', 'missing success flag')))
        return reply.get('data')

    def close(self):
        # An API failure still closes our job, including any child model process.
        if self.base and self.token and self.process and self.process.poll() is None:
            try:
                self.cleanup['models_stop'] = self.action('models.stop')
            except Exception as error:
                self.cleanup['models_stop_error'] = safe_text(error)
            try:
                self.cleanup['quit'] = self.request('/api/quit', {})
                self.process.wait(timeout=8)
            except Exception as error:
                self.cleanup['quit_error'] = safe_text(error)
        if self.job:
            self.job.close()
        if self.process:
            if self.process.poll() is None:
                # Only the Popen-owned service; never kill by executable name.
                self.process.kill()
            try:
                self.process.wait(timeout=8)
                self.cleanup['server_exited'] = True
            except subprocess.TimeoutExpired:
                self.cleanup['server_exited'] = False
        if self.log:
            self.log.close()
        if self.log_path.exists():
            text = self.log_path.read_text('utf-8', errors='replace')
            if self.token:
                text = text.replace(self.token, '[REDACTED_TOKEN]')
            self.log_path.write_text(safe_text(text), encoding='utf-8')


def is_listening(port):
    try:
        with socket.create_connection(('127.0.0.1', port), timeout=0.5):
            return True
    except OSError:
        return False


def generation_stats(package):
    logs = sorted((package / 'data/logs').glob('model-*.log'))
    counts = []
    for path in logs:
        log = path.read_text('utf-8', errors='replace')
        counts.extend(int(count) for count in re.findall(
            r'(?m)^.*\|\s*eval time\s*=.*?/\s*(\d+)\s+tokens', log))
    return {'completed_generations': len(counts), 'generated_tokens': sum(counts)}


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--master', required=True, type=Path)
    parser.add_argument('--test-parent', type=Path, help='Put the isolated test copy on this drive/directory; never overwrites existing files')
    parser.add_argument('--skip-relocation', action='store_true', help='Skip optional Chinese + spaces relocation check')
    questions_group = parser.add_mutually_exclusive_group()
    questions_group.add_argument('--question', action='append', help='Real model question; repeat for independent conversations')
    questions_group.add_argument('--qa-smoke', action='store_true', help='Verify three fixed references plus one real model answer; record all for manual review')
    args = parser.parse_args()
    questions = args.question or (QA_QUESTIONS if args.qa_smoke else [QUESTION])
    expected_kinds = ['reference', 'reference', 'reference', 'model'] if args.qa_smoke else ['model'] * len(questions)
    if any(not question.strip() for question in questions):
        parser.error('--question must not be empty')
    stamp = datetime.now(timezone.utc).strftime('%Y%m%dT%H%M%SZ')
    cache = args.test_parent.resolve() if args.test_parent else ROOT / '.cache'
    cache.mkdir(parents=True, exist_ok=True)
    run_dir = Path(tempfile.mkdtemp(prefix='verify-offline-' + stamp + '-', dir=cache)).resolve()
    evidence_dir = ROOT / 'evidence'
    evidence_dir.mkdir(exist_ok=True)
    evidence_path = evidence_dir / (run_dir.name + '.json')
    master = args.master.resolve()
    report = {'schema_version': 3, 'started_utc': datetime.now(timezone.utc).isoformat(),
              'status': 'failed', 'master': str(master), 'test_directory': str(run_dir),
              'evidence': str(evidence_path), 'questions': questions, 'expected_answer_kinds': expected_kinds,
              'answer_quality': {'status': 'manual_review_required',
                                 'boundary': 'Chinese output and actual inference are functional checks, not factual correctness or safe advice certification.'},
              'validation_scope': 'Functional delivery and actual model inference only. A pass does not certify factual correctness, repair competence, every tool, or physical offline hardware compatibility.',
              'network_test': {'test_requests': 'http://127.0.0.1 only',
                               'child_proxy': 'http://127.0.0.1:1', 'loopback_bypasses_proxy': True,
                               'real_network_disconnected': False, 'firewall_changed': False,
                               'boundary': 'No packet capture or physical disconnection. Invalid proxy cannot block clients that ignore proxy variables.'},
              'privacy': 'API session tokens are excluded; host health details are not copied into evidence.',
              'steps': []}
    servers = []
    model_port = None
    qa_failures = []
    try:
        require(os.name == 'nt', 'Windows x64 package verification must run on Windows')
        require(master.is_dir(), 'Master does not exist')
        require(not run_dir.is_relative_to(master), 'Master may not contain the test directory')
        require(empty_data(master), 'Master data/ must exist and be empty before testing')
        require((master / 'app/OFFLINE_ONLY').is_file(), 'Missing app/OFFLINE_ONLY marker')
        for path in master.rglob('*'):
            require(not path.is_symlink() and not (getattr(path.stat(), 'st_file_attributes', 0) & 0x400),
                    'Reparse point or symlink not allowed in master: ' + str(path))
        report['master_manifest_before'] = digest(master / 'app/manifest.json')
        report['steps'].append({'name': 'master_hashes', 'result': verify_manifest(master)[0]})
        package = run_dir / 'package'
        shutil.copytree(master, package)
        report['steps'].append({'name': 'copy_hashes', 'result': verify_manifest(package)[0]})
        env = isolated_env(run_dir)
        pico = read_json(package / 'app/catalog/picoclaw.json')
        pico_exe = resource_path(package, pico['path'])
        require(digest(pico_exe) == pico['sha256'], 'PicoClaw SHA256 mismatch')
        pico_env = dict(env, PICOCLAW_HOME=str(run_dir / 'pico-home'),
                        PICOCLAW_CONFIG=str(run_dir / 'pico-home/config.json'))
        result = subprocess.run([str(pico_exe), 'version'], cwd=run_dir, env=pico_env,
                                capture_output=True, timeout=20, creationflags=subprocess.CREATE_NO_WINDOW)
        output = (result.stdout + result.stderr).decode('utf-8', errors='replace').strip()
        require(result.returncode == 0 and pico['version'] in output,
                'PicoClaw version failed: ' + safe_text(output))
        report['steps'].append({'name': 'picoclaw_version', 'output': safe_text(output[:2000]),
                                'scope': 'CLI version only; PicoClaw model integration not exercised'})

        server = Server(package, run_dir, env, 'original')
        servers.append(server)
        server.start()
        catalog = server.action('tools.catalog')
        portable = catalog.get('portable_tools', [])
        require(len(portable) == 9 and all(tool.get('ready') is True for tool in portable),
                'Expected 9 ready portable tools: ' + safe_text([
                    {'id': tool.get('id'), 'ready': tool.get('ready')} for tool in portable]))
        report['steps'].append({'name': 'portable_tools_catalog', 'count': len(portable),
                                'tools': [{'id': tool.get('id'), 'name': tool.get('name'),
                                           'ready': tool['ready']} for tool in portable],
                                'scope': 'Presence and complete manifest hashes; tools are not launched'})
        state = server.action('models.inspect')
        require(state.get('bundled') is True, 'models.inspect did not find bundled resources')
        report['steps'].append({'name': 'models_inspect_before', 'bundled': state['bundled'],
                                'model_files': [item.get('path') for item in state.get('files', [])]})
        started = server.action('models.start', {'confirmed': True})
        require(started.get('ready') is True and started.get('backend') == 'bundled' and started.get('pid'),
                'A real bundled model process was not started: ' + safe_text(started))
        model_port = started['port']
        report['steps'].append({'name': 'models_start', 'result': started})
        for case_index, (question, expected_kind) in enumerate(zip(questions, expected_kinds), 1):
            case = {'name': 'reference_reply' if expected_kind == 'reference' else 'real_chinese_model_reply',
                    'case_index': case_index, 'expected_answer_kind': expected_kind,
                    'question': question, 'inference_status': 'failed',
                    'answer_quality': 'manual_review_required'}
            report['steps'].append(case)
            try:
                # Independent sessions prevent earlier answers from contaminating later scenarios.
                session = server.action('chat.start')
                require(session.get('engine') == 'local' and session.get('offline_only') is True,
                        'New conversation did not default to the offline local engine')
                case['default_engine'] = session['engine']
                case['offline_only'] = session['offline_only']
                before = generation_stats(package)
                sent = server.action('chat.send', {'session_id': session['id'],
                                                 'expected_version': session['version'],
                                                 'message': question, 'locale': 'zh-CN'})
                notices = [item.get('text', '') for item in sent.get('messages', []) if item.get('role') == 'notice']
                require(not notices, 'AI returned failure notice: ' + safe_text(' | '.join(notices)))
                require(sent.get('engine') == 'local', 'Chat changed away from the local engine')
                answers = [item.get('text', '') for item in sent.get('messages', []) if item.get('role') == 'assistant']
                require(answers, 'No assistant answer')
                answer = answers[-1].strip()
                case['answer'] = safe_text(answer)
                case['characters'] = len(answer)
                case['actions'] = [item.get('action') for item in sent.get('messages', []) if item.get('role') == 'action']
                require(len(re.findall(r'[\u4e00-\u9fff]', answer)) >= 12, 'No substantive Chinese answer')
                after = generation_stats(package)
                # A runtime may finish flushing timings just after the HTTP response.
                deadline = time.monotonic() + 2
                while expected_kind == 'model' and after['generated_tokens'] == before['generated_tokens'] and time.monotonic() < deadline:
                    time.sleep(0.1)
                    after = generation_stats(package)
                case['runtime_generation_delta'] = {key: after[key] - before[key] for key in before}
                if expected_kind == 'reference':
                    require(KNOWLEDGE_MARKER in answer, 'Fixed reference answer lacks its explicit non-model marker')
                    require(MODEL_SUFFIX not in answer, 'Fixed reference incorrectly claims model generation')
                    require(not case['actions'], 'Fixed reference unexpectedly ran an inspection or other action')
                    require(case['runtime_generation_delta']['generated_tokens'] == 0
                            and case['runtime_generation_delta']['completed_generations'] == 0,
                            'A fixed-reference scenario unexpectedly invoked model generation')
                    case['inference_status'] = 'no_generation_observed'
                    case['answer_kind'] = 'reference'
                else:
                    require(MODEL_SUFFIX in answer, 'No real local-model answer marker')
                    require(KNOWLEDGE_MARKER not in answer and '维护向导（未调用大模型）' not in answer,
                            'Fixed reference / guide fallback is not a model answer')
                    require(case['runtime_generation_delta']['generated_tokens'] > 0,
                            'Model answer lacks actual per-question generation evidence')
                    case['inference_status'] = 'observed'
                    case['answer_kind'] = 'model'
            except Exception as error:
                case['error'] = safe_text(type(error).__name__ + ': ' + str(error))
                qa_failures.append(case_index)
        stopped = server.action('models.stop')
        require(stopped.get('stopped') is True, 'models.stop did not stop the owned model')
        require(not is_listening(model_port), 'Owned model port is still listening after stop')
        model_logs = sorted((package / 'data/logs').glob('model-*.log'))
        require(model_logs, 'No real model runtime log')
        log = '\n'.join(path.read_text('utf-8', errors='replace') for path in model_logs)
        # b11146 does not emit HTTP access logs at its default verbosity. Require
        # completed token generation from the actual child, in addition to the
        # HTTP action result and answer marker; a started model alone is insufficient.
        evidence_lines = [safe_text(line) for line in log.splitlines()
                          if ('chat/completions' in line or 'model loaded' in line or 'stop processing' in line
                              or re.search(r'(?<!prompt )eval time|predicted_per_second|tokens per second', line))]
        generated = [int(count) for count in re.findall(r'(?m)^.*\|\s*eval time\s*=.*?/\s*(\d+)\s+tokens', log)]
        require('model loaded' in log and 'stop processing' in log and sum(generated) > 0,
                'Runtime log lacks completed real token generation; refusing unproven pass')
        successful_model_cases = sum(step.get('answer_kind') == 'model' for step in report['steps'])
        require(successful_model_cases > 0, 'No successful real-model scenario; refusing a reference-only pass')
        require(len(generated) >= successful_model_cases,
                'Runtime log has fewer completed generations than successful question responses')
        report['steps'].append({'name': 'model_runtime_evidence', 'lines': evidence_lines[-16:],
                                'generated_tokens': sum(generated),
                                'scope': 'Real child inference timings + successful chat.send; default runtime does not log HTTP status.',
                                'port_closed_after_stop': True})
        server.close()
        require(server.cleanup.get('server_exited'), 'Original service did not stop')

        if not args.skip_relocation:
            moved = run_dir / '客户测试 中文 带空格' / '虾盘 离线维护'
            moved.parent.mkdir()
            package.rename(moved)
            relocated = Server(moved, run_dir, env, 'relocated')
            servers.append(relocated)
            relocated.start()
            health = relocated.action('system.inspect')
            require(isinstance(health, dict) and health, 'Relocated system.inspect returned no data')
            state = relocated.action('models.inspect')
            require(state.get('bundled') is True, 'Relocated package cannot find bundled model resources')
            report['steps'].append({'name': 'relocation_read_only_health', 'path': str(moved),
                                    'system_fields': sorted(health), 'bundled': state['bundled'],
                                    'scope': 'Real read-only health + resource inspection; model is not restarted'})
            relocated.close()
            require(relocated.cleanup.get('server_exited'), 'Relocated service did not stop')
        report['status'] = 'failed' if qa_failures else 'passed'
        if qa_failures:
            report['error'] = 'Actual model inference failed for question indices: ' + ', '.join(map(str, qa_failures))
    except Exception as error:
        report['error'] = safe_text(type(error).__name__ + ': ' + str(error))
    finally:
        for server in servers:
            try:
                server.close()
            except Exception as error:
                server.cleanup['cleanup_error'] = safe_text(error)
                report['status'] = 'failed'
        report['services'] = [{'label': server.label, 'calls': server.calls, 'cleanup': server.cleanup}
                              for server in servers]
        report['master_data_empty_after'] = empty_data(master)
        if not report['master_data_empty_after']:
            report['status'] = 'failed'
            report['master_preservation_error'] = 'Master data/ is missing or no longer empty'
        if (master / 'app/manifest.json').exists():
            report['master_manifest_unchanged'] = digest(master / 'app/manifest.json') == report.get('master_manifest_before')
            if not report['master_manifest_unchanged']:
                report['status'] = 'failed'
        if model_port:
            report['model_port_closed_at_end'] = not is_listening(model_port)
            if not report['model_port_closed_at_end']:
                report['status'] = 'failed'
        report['finished_utc'] = datetime.now(timezone.utc).isoformat()
        temporary = evidence_path.with_suffix('.json.tmp')
        temporary.write_text(json.dumps(report, ensure_ascii=False, indent=2) + '\n', encoding='utf-8')
        temporary.replace(evidence_path)
        print(json.dumps({'status': report['status'], 'evidence': str(evidence_path),
                          'test_directory': str(run_dir), 'error': report.get('error')}, ensure_ascii=False))
    return 0 if report['status'] == 'passed' else 1


if __name__ == '__main__':
    raise SystemExit(main())
