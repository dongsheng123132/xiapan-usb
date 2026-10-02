"""Stop only local servers whose inspected portable roots match our test records."""
import json,re,urllib.request
from pathlib import Path
PROJECT=Path(__file__).resolve().parent.parent
http=urllib.request.build_opener(urllib.request.ProxyHandler({}))
allowed=set()
for name in ('preview-target.json','ui-server.json'):
 p=PROJECT/'evidence'/name
 if p.exists():allowed.add(json.loads(p.read_text(encoding='utf-8-sig'))['root'].lower())
for port in (18798,18797):
 base=f'http://127.0.0.1:{port}'
 try:
  with http.open(base+'/',timeout=3) as r:html=r.read().decode()
  match=re.search(r'name="xiapan-token" content="([a-f0-9]+)"',html)
  if not match:continue
  token=match[1];headers={'Content-Type':'application/json','X-Xiapan-Token':token}
  req=urllib.request.Request(base+'/api/run',data=b'{"action":"system.inspect","input":{}}',headers=headers)
  with http.open(req,timeout=5) as r:system=json.load(r)
  if not system.get('ok') or system['data']['portable_root'].lower() not in allowed:continue
  req=urllib.request.Request(base+'/api/quit',data=b'',headers=headers)
  with http.open(req,timeout=5) as r:r.read()
  print(f'Stopped owned preview on {port}')
 except (OSError,ValueError):pass
