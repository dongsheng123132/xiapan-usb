"""Exercise the prepared exFAT preview. Never print credentials or make payments."""
import json, re, time, urllib.request
from pathlib import Path

PROJECT=Path(__file__).resolve().parent.parent
state=json.loads((PROJECT/'evidence/preview-server.json').read_text(encoding='utf-8-sig'))
BASE=state['url'];ROOT=Path(state['root'])
http=urllib.request.build_opener(urllib.request.ProxyHandler({}))
with http.open(BASE+'/',timeout=10) as r: html=r.read().decode()
TOKEN=re.search(r'name="xiapan-token" content="([a-f0-9]+)"',html)[1]
def run(action,inputs=None):
 req=urllib.request.Request(BASE+'/api/run',data=json.dumps({'action':action,'input':inputs or {}}).encode(),headers={'Content-Type':'application/json','X-Xiapan-Token':TOKEN})
 with http.open(req,timeout=65) as r: result=json.load(r)
 if not result['ok']:raise RuntimeError(action+': '+result.get('error','failed'))
 return result.get('data')

out={'root':str(ROOT),'checks':{},'payments_made':False,'physical_network_disconnection':False}
wallet=run('wallet.refresh');out['checks']['wallet']={'ready':wallet['ready'],'available_tokens':wallet.get('available_tokens'),'note':wallet['note']}
out['checks']['recharge_page']=run('wallet.recharge',{'check_only':True})
models=run('cloud.models');out['checks']['models']={'count':len(models['models']),'selected':models['selected']}
s=run('chat.start')
def send(message,engine='cloud'):
 global s
 s=run('chat.send',{'session_id':s['id'],'expected_version':s['version'],'message':message,'engine':engine})
 return s
def confirm_all():
 global s
 while s.get('pending'):
  p=s['pending'][0]
  s=run('chat.confirm',{'session_id':s['id'],'expected_version':s['version'],'proposal_id':p['id'],'confirmed':True})
send('检查电脑内存和盘里的资源，再安装体检工具并保存报告','guide')
out['checks']['multi_action_ids']=[m['action'] for m in s['messages'] if m['role']=='action']
out['checks']['pending_before_confirm']=[p['action'] for p in s.get('pending',[])]
confirm_all()
out['checks']['writes']=[{'action':m['action'],'data':{k:v for k,v in m['data'].items() if k in ('verified','path','version')}} for m in s['messages'] if m['role']=='action' and m['action'] in ('tools.install','report.create')]
send('/网络');out['checks']['network']=s['messages'][-1]['data']
send('/进程');out['checks']['process_count']=len(s['messages'][-1]['data']['processes'])
persisted=run('chat.read',{'session_id':s['id']});out['checks']['history_saved']=persisted['version']==s['version'] and len(persisted['messages'])==len(s['messages'])
# With this newly created wallet's minimum verification grant, expect a clear
# server rejection rather than a made-up cloud answer. No top-up occurs here.
send('请用一句话回答你好，不要调用任何工具。')
out['checks']['cloud_request']={'roles':[m['role'] for m in s['messages'][-2:]],'reply':s['messages'][-1]['text']}
started=time.monotonic();send('用一句话说明电脑维护为什么要先备份。','local');out['checks']['offline_model']={'seconds':round(time.monotonic()-started,2),'role':s['messages'][-1]['role'],'reply':s['messages'][-1]['text']}
out['checks']['model_stop']=run('models.stop')
(PROJECT/'evidence/preview-results.json').write_text(json.dumps(out,ensure_ascii=False,indent=2),encoding='utf-8')
print(json.dumps(out,ensure_ascii=False,indent=2))
