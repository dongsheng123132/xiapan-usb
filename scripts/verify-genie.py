"""Exercise the actual pinned PicoClaw with custom settings and relocated data.
Only a localhost model fixture is used. No paid API or system changes.
"""
import http.server,json,os,shutil,subprocess,tempfile,threading
from pathlib import Path

ROOT=Path(__file__).resolve().parent.parent
key='sk-genie-fixture-custom'
observed=[]
class Fixture(http.server.BaseHTTPRequestHandler):
 def log_message(self,*args):pass
 def do_POST(self):
  request=json.loads(self.rfile.read(int(self.headers['Content-Length'])))
  tool_names=[t.get('function',{}).get('name') for t in request.get('tools',[])]
  observed.append({'model':request.get('model'),'path':self.path,'custom_key_used':self.headers.get('Authorization')=='Bearer '+key,'tools':tool_names})
  if 'connection_check' in tool_names:
   message={'role':'assistant','tool_calls':[{'id':'probe','type':'function','function':{'name':'connection_check','arguments':'{}'}}]}
  elif not any(m.get('role')=='tool' for m in request.get('messages',[])):
   message={'role':'assistant','tool_calls':[{'id':'inspect','type':'function','function':{'name':'mcp_xiapan_maintenance_action','arguments':json.dumps({'action':'startup.inspect'})}}]}
  else:
   message={'role':'assistant','content':'已读取常见启动项，未修改系统。fixture redaction: '+key}
  data=json.dumps({'id':'fixture','object':'chat.completion','choices':[{'index':0,'message':message,'finish_reason':'tool_calls' if 'tool_calls' in message else 'stop'}],'usage':{'prompt_tokens':8,'completion_tokens':8,'total_tokens':16}}).encode()
  self.send_response(200);self.send_header('Content-Type','application/json');self.send_header('Content-Length',str(len(data)));self.end_headers();self.wfile.write(data)

server=http.server.ThreadingHTTPServer(('127.0.0.1',0),Fixture)
threading.Thread(target=server.serve_forever,daemon=True).start()
try:
 with tempfile.TemporaryDirectory(prefix='genie-native-',dir=ROOT/'.cache',ignore_cleanup_errors=True) as directory:
  parent=Path(directory);root=parent/'原目录';root.mkdir()
  (root/'app/runtime/picoclaw/windows-x64').mkdir(parents=True)
  shutil.copy2(ROOT/'runtime/picoclaw/windows-x64/picoclaw.exe',root/'app/runtime/picoclaw/windows-x64/picoclaw.exe')
  (root/'data/wallet').mkdir(parents=True)
  (root/'data/wallet/device.json').write_text(json.dumps({'apiKey':'sk-wallet-must-not-be-used'}),encoding='utf-8')
  env=os.environ.copy();env.pop('UCLAW_API_BASE_URL',None)
  env['PICOCLAW_TOOLS_EXEC_ENABLED']='true'
  def run(action,inputs=None):
   args=[str(ROOT/'dist/windows-x64/虾盘.exe'),'action','run',action,'--root',str(root),'--json','--no-input']
   if inputs is not None:
    file=root/'input-fixture.json';file.write_text(json.dumps(inputs,ensure_ascii=False),encoding='utf-8');args+=['--input-file',str(file)]
   process=subprocess.run(args,env=env,capture_output=True,timeout=70)
   result=json.loads(process.stdout)
   assert result['ok'],result.get('error')
   return result['data']
  settings=run('settings.model.get')
  inputs={'expected_revision':settings['revision'],'source':'custom','base_url':f'http://127.0.0.1:{server.server_port}/v1','api_key':key,'model':'fixture/custom-model'}
  assert run('settings.model.test',inputs)['tools_supported']
  assert run('settings.model.get')['revision']=='initial'
  run('settings.model.save',inputs)
  assert key not in json.dumps(run('settings.model.get'))
  session=run('chat.start')
  def converse(session):
   result=run('chat.send',{'session_id':session['id'],'expected_version':session['version'],'message':'请执行本轮本地验证。'})
   assert result['messages'][-1]['role']=='assistant',result['messages'][-1]
   assert key not in json.dumps(result)
   actions=[m for m in result['messages'] if m['role']=='action']
   assert actions[-1]['action']=='startup.inspect'
   return result,actions[-1]['data']
  session,startup=converse(session)
  dest=parent/'换盘符模拟 新位置';root.rename(dest);root=dest
  restored=run('chat.read',{'session_id':session['id']})
  assert restored['version']==session['version']
  assert run('settings.model.get')['model']=='fixture/custom-model'
  session,_=converse(restored)
  assert all(r['custom_key_used'] and r['model']=='fixture/custom-model' and r['path']=='/v1/chat/completions' for r in observed),observed
  assert all('exec' not in r['tools'] for r in observed)
  assert not list((root/'data/pico/runs').glob('*/.security.yml'))
  evidence={'actual_pico_version':'0.3.1','model_service':'localhost fixture only','requests':observed,'startup_item_count':len(startup['items']),'startup_warnings':startup['warnings'],'custom_api_without_cloud':True,'settings_and_history_survive_relocation':True,'secret_redacted':True,'ephemeral_secrets_cleaned':True,'system_writes':False}
  (ROOT/'evidence/genie-native-results.json').write_text(json.dumps(evidence,ensure_ascii=False,indent=2),encoding='utf-8')
  print(json.dumps(evidence,ensure_ascii=False,indent=2))
finally:
 server.shutdown();server.server_close()
