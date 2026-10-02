"""Read-only hardware checks and native MCP evidence; no system repairs/payments."""
import json,re,time,urllib.request
from pathlib import Path
PROJECT=Path(__file__).resolve().parent.parent
state=json.loads((PROJECT/'evidence/preview-server.json').read_text(encoding='utf-8-sig'))
BASE=state['url'];http=urllib.request.build_opener(urllib.request.ProxyHandler({}))
with http.open(BASE+'/',timeout=10) as r:html=r.read().decode()
TOKEN=re.search(r'name="xiapan-token" content="([a-f0-9]+)"',html)[1]
def run(action,inputs=None):
 req=urllib.request.Request(BASE+'/api/run',data=json.dumps({'action':action,'input':inputs or {}}).encode(),headers={'Content-Type':'application/json','X-Xiapan-Token':TOKEN})
 with http.open(req,timeout=65) as r: result=json.load(r)
 if not result['ok']:raise RuntimeError(action+': '+result.get('error','failed'))
 return result.get('data')
out={'version':'0.3.0-preview','root':state['root'],'payments_made':False,'repairs_performed':False}
wallet=run('wallet.refresh');out['wallet']={k:wallet.get(k) for k in ['ready','available_tokens','note']}
started=time.monotonic();drivers=run('drivers.inspect')
out['drivers']={'seconds':round(time.monotonic()-started,2),'devices':len(drivers['devices']),'hardware_ids_present':sum(bool(n.get('hardware_ids')) for n in drivers['devices']),'fault_devices':[{'name':n.get('name'),'error_code':n.get('error_code')} for n in drivers['devices'] if n.get('error_code')],'offline_files':len(drivers['offline_files']),'internet_required':drivers['internet_required'],'driver_install_supported':drivers['driver_install_supported']}
catalog=run('tools.catalog');out['tools']=[{'id':t['id'],'ready':t['ready']} for t in catalog['builtin_tools']];out['resource_entries']=len(catalog['resources'])
s=run('chat.start');out['default_engine']=s['engine']
s=run('chat.send',{'session_id':s['id'],'expected_version':s['version'],'message':'/工具'})
s=run('chat.send',{'session_id':s['id'],'expected_version':s['version'],'message':'/打开 device-manager'})
out['tool_proposal']=[p['action'] for p in s['pending']]
out['tool_launch_executed']=any(m.get('action')=='tools.launch' and m['role']=='action' for m in s['messages'])
out['history_saved']=run('chat.read',{'session_id':s['id']})['version']==s['version']
assert out['default_engine']=='pico' and out['tool_proposal']==['tools.launch'] and not out['tool_launch_executed']
# A funded call is a small, explicitly authorized verification. This stays
# separate from old long diagnostics and asks for one read-only action only.
if wallet.get('available_tokens',0)>10000:
 s=run('chat.start');started=time.monotonic()
 s=run('chat.send',{'session_id':s['id'],'expected_version':s['version'],'message':'请只调用一次 system.inspect，告诉我当前系统和可用内存，用两句话回答，不要运行其他检测或打开工具。'})
 out['real_pico']={'session_id':s['id'],'seconds':round(time.monotonic()-started,2),'actions':[m.get('action') for m in s['messages'] if m['role']=='action'],'role':s['messages'][-1]['role'],'reply':s['messages'][-1]['text']}
 after=run('wallet.refresh');out['wallet_after']={k:after.get(k) for k in ['ready','available_tokens']}
else:out['real_pico']={'status':'awaiting_funded_wallet'}
(PROJECT/'evidence/preview-03-results.json').write_text(json.dumps(out,ensure_ascii=False,indent=2),encoding='utf-8')
print(json.dumps(out,ensure_ascii=False,indent=2))
