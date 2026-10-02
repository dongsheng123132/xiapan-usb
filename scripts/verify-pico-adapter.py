"""Run the pinned PicoClaw binary against a deterministic local API fixture."""
import http.server,json,os,shutil,subprocess,tempfile,threading
from pathlib import Path
PROJECT=Path(__file__).resolve().parent.parent
key='sk-fixture-pico-not-a-real-wallet'
observed=[];tool_evidence=[]
class Fixture(http.server.BaseHTTPRequestHandler):
 def log_message(self,*args):pass
 def do_POST(self):
  request=json.loads(self.rfile.read(int(self.headers['Content-Length'])))
  tools=[t.get('function',{}).get('name') for t in request.get('tools',[])]
  observed.append({'authenticated':self.headers.get('Authorization')=='Bearer '+key,'model':request.get('model'),'tools':tools})
  if len(observed)==1:
   name='mcp_xiapan_maintenance_action'
   if name not in tools: raise RuntimeError('Native PicoClaw did not discover maintenance MCP')
   calls=[]
   for i,args in enumerate([{'action':'system.inspect'},{'action':'tools.catalog'},{'action':'report.create'},{'action':'tools.launch','tool_id':'device-manager'},{'action':'wallet.rotate','confirmed':True},{'action':'report.create','confirmed':True}]):
    calls.append({'id':f'test-{i}','type':'function','function':{'name':name,'arguments':json.dumps(args)}})
   message={'role':'assistant','content':'','tool_calls':calls};finish='tool_calls'
  else:
   tool_evidence.extend(m.get('content','') for m in request.get('messages',[]) if m.get('role')=='tool')
   message={'role':'assistant','content':'这是原生 PicoClaw 的接口回放：已实际读取体检与工具集合，保存报告和打开设备管理器都等待确认。'};finish='stop'
  result={'id':'fixture','object':'chat.completion','choices':[{'index':0,'message':message,'finish_reason':finish}],'usage':{'prompt_tokens':10,'completion_tokens':20,'total_tokens':30}}
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
 s=run('chat.start')['data'];assert s['engine']=='pico'
 # This text intentionally contains no old diagnostic keywords. Only native
 # model-selected MCP calls may produce the evidence and pending operations.
 result=run('chat.send',{'session_id':s['id'],'expected_version':s['version'],'message':'请按照本轮验证任务处理。'})
 messages=result.get('data',{}).get('messages',[]);last=messages[-1] if messages else {}
 if last.get('role')!='assistant':
  debug_env=env.copy();debug_env['PICOCLAW_HOME']=str(root/'data/pico');debug_env['PICOCLAW_CONFIG']=str(root/'data/pico/config.json')
  debug=subprocess.run([str(root/'runtime/picoclaw/windows-x64/picoclaw.exe'),'agent','--no-color','--message','fixture test'],env=debug_env,capture_output=True,timeout=55)
  print(debug.stderr.decode(errors='replace').replace(key,'[fixture]').strip()[:3000])
 forbidden={'exec','write_file','edit_file','append_file','install_skill','subagent','spawn','cron'}
 pending=result.get('data',{}).get('pending',[])
 completed=[m.get('action') for m in messages if m.get('role')=='action']
 reports=run('reports.list')['data']
 evidence={'api_fixture':'local deterministic replay, not paid cloud','native_requests':observed,'role':last.get('role'),'reply':last.get('text'),'credential_exposed':key in json.dumps(result),'actual_actions':completed,'pending_actions':[p['action'] for p in pending],'reports_before_confirmation':len(reports),'ephemeral_credentials_cleaned':not list((root/'data/pico/runs').glob('*/.security.yml')),'real_evidence_returned_to_model':any('memory_bytes' in str(t) for t in tool_evidence),'confirmation_returned_to_model':any('requires_user_confirmation' in str(t) for t in tool_evidence)}
 (PROJECT/'evidence/pico-adapter-results.json').write_text(json.dumps(evidence,ensure_ascii=False,indent=2),encoding='utf-8')
 print(json.dumps(evidence,ensure_ascii=False,indent=2))
 if len(observed)!=2 or not all(x['authenticated'] for x in observed) or any(forbidden.intersection(x['tools']) for x in observed) or last.get('role')!='assistant' or evidence['credential_exposed'] or completed!=['system.inspect','tools.catalog'] or set(evidence['pending_actions'])!={'report.create','tools.launch'} or reports or not evidence['ephemeral_credentials_cleaned'] or not evidence['real_evidence_returned_to_model'] or not evidence['confirmation_returned_to_model']:raise SystemExit('Pico adapter did not pass')
server.shutdown()
