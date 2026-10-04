"""Create a fresh Windows USB package from this source tree; never copy user data."""
import hashlib,json,re,shutil,subprocess,zipfile
from pathlib import Path

ROOT=Path(__file__).resolve().parent.parent
exe=ROOT/'dist/windows-x64/虾盘.exe'
pico=ROOT/'runtime/picoclaw/windows-x64/picoclaw.exe'
source_version=re.search(r'^const version = "([^"]+)"$',(ROOT/'main.go').read_text(encoding='utf-8'),re.M).group(1)
build_version=subprocess.check_output([str(exe),'version'],timeout=10).decode().strip()
if build_version!=source_version:raise RuntimeError('Build version mismatch')
source_commit=subprocess.check_output(['git','rev-parse','HEAD'],cwd=ROOT,text=True,timeout=10).strip()
if subprocess.check_output(['git','status','--porcelain'],cwd=ROOT,text=True,timeout=10).strip():
 raise RuntimeError('Release packaging requires a clean source tree')
build_info=subprocess.check_output(['go','version','-m',str(exe)],text=True,timeout=10)
if 'vcs.revision='+source_commit not in build_info or 'vcs.modified=false' not in build_info:
 raise RuntimeError('Executable does not match the clean source commit; rebuild first')
manifest=json.loads((ROOT/'catalog/picoclaw.json').read_text(encoding='utf-8'))
digest=lambda p:hashlib.sha256(p.read_bytes()).hexdigest()
if digest(pico)!=manifest['sha256']:raise RuntimeError('PicoClaw hash mismatch')
name='Xiapan-'+build_version+'-windows-x64'
target=ROOT/'dist'/name
archive=Path(str(target)+'.zip')
if target.exists() or archive.exists():raise RuntimeError('Package already exists; preserve the existing artifact')
target.mkdir()
(target/'data').mkdir()
(target/'app/runtime/picoclaw/windows-x64').mkdir(parents=True)
shutil.copy2(exe,target/'虾盘.exe')
shutil.copy2(pico,target/'app/runtime/picoclaw/windows-x64/picoclaw.exe')
shutil.copytree(ROOT/'licenses',target/'app/licenses')
for filename in ['LICENSE','THIRD_PARTY.md']:
 shutil.copy2(ROOT/filename,target/'app'/filename)
(target/'app/使用说明.txt').write_text(f'虾盘 {build_version} · Windows x64 公司网管预发布版\n双击 虾盘.exe，选择故障并检测。AI 按需配置公司 API；检测无需 AI。\n设置、对话和报告保存在 data/，目前未加密。更新保留已有 data/，不要覆盖为初始模板。\n本版不含第三方工具、离线模型或启动救援。第二台企业电脑、企业网络和 U 盘移动验收尚待完成。\nhttps://github.com/dongsheng123132/xiapan-usb/releases/tag/v{build_version}\n',encoding='utf-8')
files={p.relative_to(target).as_posix():{'bytes':p.stat().st_size,'sha256':digest(p)} for p in target.rglob('*') if p.is_file()}
info={'version':build_version,'source_commit':source_commit,'pico_version':manifest['version'],'source_dirty':False,'files':files,'user_data_included':False}
(target/'app/manifest.json').write_text(json.dumps(info,ensure_ascii=False,indent=2),encoding='utf-8')
with zipfile.ZipFile(archive,'x',zipfile.ZIP_DEFLATED,compresslevel=5) as z:
 for p in target.rglob('*'):
  z.write(p,Path(name)/p.relative_to(target))
archive.with_suffix('.zip.sha256').write_text(digest(archive)+'  '+archive.name+'\n',encoding='utf-8')
result={'root':str(target),'archive':str(archive),'bytes':sum(p.stat().st_size for p in target.rglob('*') if p.is_file()),'zip_bytes':archive.stat().st_size,'sha256':digest(archive)}
(ROOT/'evidence').mkdir(exist_ok=True)
(ROOT/'evidence/genie-package.json').write_text(json.dumps(result,ensure_ascii=False,indent=2),encoding='utf-8')
print(json.dumps(result,ensure_ascii=False,indent=2))
