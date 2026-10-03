"""Create a fresh Windows USB package from this source tree; never copy user data."""
import hashlib,json,shutil,subprocess,uuid,zipfile
from pathlib import Path

ROOT=Path(__file__).resolve().parent.parent
name='虾盘 U盘精灵 0.4-'+uuid.uuid4().hex[:6]
target=ROOT/'dist'/name
target.mkdir()
(target/'data').mkdir()
(target/'app/runtime/picoclaw/windows-x64').mkdir(parents=True)
exe=ROOT/'dist/windows-x64/虾盘.exe'
pico=ROOT/'runtime/picoclaw/windows-x64/picoclaw.exe'
if subprocess.check_output([str(exe),'version'],timeout=10).decode().strip()!='0.4.0-preview':raise RuntimeError('Build version mismatch')
manifest=json.loads((ROOT/'catalog/picoclaw.json').read_text(encoding='utf-8'))
digest=lambda p:hashlib.sha256(p.read_bytes()).hexdigest()
if digest(pico)!=manifest['sha256']:raise RuntimeError('PicoClaw hash mismatch')
shutil.copy2(exe,target/'虾盘.exe')
shutil.copy2(pico,target/'app/runtime/picoclaw/windows-x64/picoclaw.exe')
shutil.copytree(ROOT/'licenses',target/'app/licenses')
for filename in ['LICENSE','THIRD_PARTY.md']:
 shutil.copy2(ROOT/filename,target/'app'/filename)
(target/'app/使用说明.txt').write_text('双击根目录的 虾盘.exe。模型设置可选择虾盘云或自带 OpenAI 兼容 API。\n虾盘云钱包可从模型设置打开；自带 API 无需钱包。\n设置、密钥及备份、对话保存在 data/，请妥善保管。\n退出应用后安全移除 U 盘；此包不含离线模型、驱动、救援镜像，也不是可启动系统盘。\n完整更新时替换程序文件，保留自己的 data/。\n',encoding='utf-8')
files={p.relative_to(target).as_posix():{'bytes':p.stat().st_size,'sha256':digest(p)} for p in target.rglob('*') if p.is_file()}
info={'version':'0.4.0-preview','source_commit':subprocess.check_output(['git','rev-parse','HEAD'],cwd=ROOT,text=True).strip(),'pico_version':manifest['version'],'files':files,'user_data_included':False}
(target/'app/manifest.json').write_text(json.dumps(info,ensure_ascii=False,indent=2),encoding='utf-8')
archive=Path(str(target)+'.zip')
with zipfile.ZipFile(archive,'x',zipfile.ZIP_DEFLATED,compresslevel=5) as z:
 for p in target.rglob('*'):
  z.write(p,Path(name)/p.relative_to(target))
archive.with_suffix('.zip.sha256').write_text(digest(archive)+'  '+archive.name+'\n',encoding='utf-8')
result={'root':str(target),'archive':str(archive),'bytes':sum(p.stat().st_size for p in target.rglob('*') if p.is_file()),'zip_bytes':archive.stat().st_size,'sha256':digest(archive)}
(ROOT/'evidence').mkdir(exist_ok=True)
(ROOT/'evidence/genie-package.json').write_text(json.dumps(result,ensure_ascii=False,indent=2),encoding='utf-8')
print(json.dumps(result,ensure_ascii=False,indent=2))
