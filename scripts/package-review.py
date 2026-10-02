"""Build a fresh, credential-free Windows review candidate from this source tree."""
import hashlib,json,re,shutil,subprocess,zipfile
from datetime import datetime
from pathlib import Path

ROOT=Path(__file__).resolve().parent.parent
def sha(p):
 h=hashlib.sha256()
 with p.open('rb') as f:
  for b in iter(lambda:f.read(1024*1024),b''):h.update(b)
 return h.hexdigest()
def git(*args):return subprocess.check_output(['git',*args],cwd=ROOT,text=True,timeout=10).strip()

commit=git('rev-parse','HEAD')
if git('status','--porcelain'):raise SystemExit('Commit the reviewed source changes before packaging.')
pwsh=shutil.which('pwsh')
if not pwsh:raise SystemExit('PowerShell 7 is required for UTF-8 build scripts.')
subprocess.run([pwsh,'-NoProfile','-File',str(ROOT/'scripts/build.ps1')],cwd=ROOT,check=True,timeout=180)
version=re.search(r'const version = "([^"]+)"',(ROOT/'main.go').read_text(encoding='utf-8'))[1]
out=ROOT/'dist'/('GOSIM-review-'+datetime.now().strftime('%Y%m%d-%H%M%S'))
out.mkdir(parents=True,exist_ok=False)
pack=out/'Xiapan-Windows';pack.mkdir()
pico=json.loads((ROOT/'catalog/picoclaw.json').read_text(encoding='utf-8'))
if sha(ROOT/pico['path'])!=pico['sha256']:raise SystemExit('PicoClaw checksum mismatch')
files={
 '虾盘.exe':ROOT/'dist/windows-x64/虾盘.exe',
 'runtime/picoclaw/windows-x64/picoclaw.exe':ROOT/pico['path'],
 'LICENSE':ROOT/'LICENSE','THIRD_PARTY.md':ROOT/'THIRD_PARTY.md',
 'licenses/PicoClaw-MIT.txt':ROOT/'licenses/PicoClaw-MIT.txt',
 '终端维护菜单.cmd':ROOT/'launchers/终端维护菜单.cmd',
 '终端维护菜单.ps1':ROOT/'launchers/终端维护菜单.ps1',
}
for dest,src in files.items():
 target=pack/dest;target.parent.mkdir(parents=True,exist_ok=True);shutil.copy2(src,target)
for name in ['models','drivers','packages','images']:(pack/name).mkdir()
(pack/'先读我.txt').write_text('虾盘 '+version+' · Windows x64 轻量评审包\n\n解压整个目录到 U 盘或普通目录，再双击 虾盘.exe。\n默认 PicoClaw 连接虾盘云。此包不带用户钱包或预付额度；可在设备钱包中取得/填入自己的凭证并自行充值。\n无需云端即可使用本地体检、网络/驱动识别、资源扫描、工具集合和维护向导。\n本包不含离线大模型、驱动包或启动镜像。不是开机启动盘。\n本版已在 Windows 实机和 exFAT U 盘验证；自动驱动安装、系统重装与跨电脑接续尚未完成验收。\n',encoding='utf-8-sig')
manifest={'version':version,'source_commit':commit,'platform':'windows/amd64','cloud_ai_requires_own_wallet':True,'offline_model_included':False,'files':{p.relative_to(pack).as_posix():{'sha256':sha(p),'bytes':p.stat().st_size} for p in sorted(pack.rglob('*')) if p.is_file()}}
(pack/'manifest.json').write_text(json.dumps(manifest,ensure_ascii=False,indent=2),encoding='utf-8')
archive=out/'Xiapan-PicoClaw-Windows-review.zip'
with zipfile.ZipFile(archive,'w',zipfile.ZIP_DEFLATED,compresslevel=6) as z:
 for p in sorted(pack.rglob('*')):
  z.write(p,p.relative_to(out))
with zipfile.ZipFile(archive) as z:
 bad=z.testzip()
 if bad:raise SystemExit('Corrupt archive entry: '+bad)
 if any('/data/' in n.replace('\\','/').lower() or '.security' in n or 'device.json' in n for n in z.namelist()):raise SystemExit('Private state in candidate')
receipt={'directory':str(out),'archive':str(archive),'sha256':sha(archive),'bytes':archive.stat().st_size,'source_commit':commit,'version':version,'published':False}
shutil.copy2(ROOT/'docs/GOSIM原型页.html',out/'index.html')
shutil.copy2(ROOT/'evidence/PicoClaw-真实云端维护-16秒.mp4',out/'PicoClaw-demo.mp4')
for name in ['GOSIM报名稿-2026-10-02.md','GOSIM演示分镜-2026-10-02.md']:
 shutil.copy2(ROOT/'docs'/name,out/name)
(out/'release.json').write_text(json.dumps(receipt,ensure_ascii=False,indent=2),encoding='utf-8')
(ROOT/'evidence/review-candidate.json').write_text(json.dumps(receipt,ensure_ascii=False,indent=2),encoding='utf-8')
print(json.dumps(receipt,ensure_ascii=False,indent=2))
