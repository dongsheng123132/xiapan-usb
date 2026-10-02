"""Run the pinned PicoClaw binary against a deterministic local API fixture."""
import http.server,json,os,shutil,subprocess,tempfile,threading
from pathlib import Path
PROJECT=Path(__file__).resolve().parent.parent
key='sk-fixture-pico-not-a-real-wallet'
observed=[]
class Fixture(http.server.BaseHTTPRequestHandler):
 def log_message(self,*args):pass
 def do_POST(self):
  request=json.loads(self.rfile.read(int(self.headers['Content-Length'])))
  observed.append({'authenticated':self.headers.get('Authorization')=='Bearer '+key,'model':request.get('model'),'tools':[t.get('function',{}).get('name') for t in request.get('tools',[])]})
  result={'id':'fixture','object':'chat.completion','choices':[{'index':0,'message':{'role':'assistant','content':'这是本地接口回放测试。先备份数据，再读取真实检测结果。'},'finish_reason':'stop'}],'usage':{'prompt_tokens':10,'completion_tokens':20,'total_tokens':30}}
  payload=json.dumps(result,ensure_ascii=False).encode();self.send_response(200);self.send_header('Content-Type','application/json');self.send_header('Content-Length',str(len(payload)));self.end_headers();self.wfile.write(payload)
server=http.server.ThreadingHTTPServer(('127.0.0.1',0),Fixture);threading.Thread(target=server.serve_forever,daemon=True).start()
with tempfile.TemporaryDirectory(prefix='pico-fixture-',dir=PROJECT/'.cache',ignore_cleanup_errors=True) as directory:
 root=Path(directory);(root/'data/wallet').mkdir(parents=True);(root/'data/wallet/device.json').write_text(json.dumps({'apiKey':key}),encoding='utf-8')
 (root/'runtime/picoclaw/windows-x64').mkdir(parents=True);shutil.copy2(PROJECT/'runtime/picoclaw/windows-x64/picoclaw.exe',root/'runtime/picoclaw/windows-x64/picoclaw.exe')
 env=os.environ.copy();env['UCLAW_API_BASE_URL']=f'http://127.0.0.1:{server.server_port}'
 # The adapter must ignore host overrides that would re-enable its tools.
 env['PICOCLAW_TOOLS_EXEC_ENABLED']='true';env['PICOCLAW_TOOLS_WRITE_FILE_ENABLED']='true'
 def run(action,inputs=None):
  args=[str(PROJECT/'dist/windows-x64/虾盘.exe'),'action','run',action,'--root',str(root),'--json','--no-input']
  if inputs is not None:
   p=root/'fixture-input.json';p.write_text(json.dumps(inputs,ensure_ascii=False),encoding='utf-8');args+=['--input-file',str(p)]
  result=subprocess.run(args,env=env,capture_output=True,timeout=62);return json.loads(result.stdout)
 s=run('chat.start')['data'];result=run('chat.send',{'session_id':s['id'],'expected_version':s['version'],'engine':'pico','message':'说明电脑维护的第一步。'})
 messages=result.get('data',{}).get('messages',[]);last=messages[-1] if messages else {}
 if last.get('role')!='assistant':
  debug_env=env.copy();debug_env['PICOCLAW_HOME']=str(root/'data/pico');debug_env['PICOCLAW_CONFIG']=str(root/'data/pico/config.json')
  debug=subprocess.run([str(root/'runtime/picoclaw/windows-x64/picoclaw.exe'),'agent','--no-color','--message','fixture test'],env=debug_env,capture_output=True,timeout=55)
  print(debug.stderr.decode(errors='replace').replace(key,'[fixture]').strip()[:3000])
 forbidden={'exec','write_file','edit_file','append_file','install_skill','subagent','spawn','cron'}
 evidence={'api_fixture':'local deterministic replay, not paid cloud','native_requests':observed,'role':last.get('role'),'reply':last.get('text'),'credential_exposed':key in json.dumps(result)}
 (PROJECT/'evidence/pico-adapter-results.json').write_text(json.dumps(evidence,ensure_ascii=False,indent=2),encoding='utf-8')
 print(json.dumps(evidence,ensure_ascii=False,indent=2))
 if not observed or not all(x['authenticated'] for x in observed) or any(forbidden.intersection(x['tools']) for x in observed) or last.get('role')!='assistant' or evidence['credential_exposed']:raise SystemExit('Pico adapter did not pass')
server.shutdown()
