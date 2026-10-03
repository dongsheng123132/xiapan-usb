"""Create a fresh Windows USB package from this source tree; never copy user data."""
import argparse,hashlib,json,shutil,subprocess,uuid,zipfile
from pathlib import Path

ROOT=Path(__file__).resolve().parent.parent
parser=argparse.ArgumentParser()
parser.add_argument('--with-tools',action='store_true',help='Include pinned Windows portable tools and upstream sources')
args=parser.parse_args()
name='虾盘 U盘精灵 0.4.3'+('-工具箱' if args.with_tools else '')+'-'+uuid.uuid4().hex[:6]
target=ROOT/'dist'/name
target.mkdir()
(target/'data').mkdir()
(target/'app/runtime/picoclaw/windows-x64').mkdir(parents=True)
exe=ROOT/'dist/windows-x64/虾盘.exe'
pico=ROOT/'runtime/picoclaw/windows-x64/picoclaw.exe'
build_version=subprocess.check_output([str(exe),'version'],timeout=10).decode().strip()
if build_version!='0.4.3-preview':raise RuntimeError('Build version mismatch')
manifest=json.loads((ROOT/'catalog/picoclaw.json').read_text(encoding='utf-8'))
digest=lambda p:hashlib.sha256(p.read_bytes()).hexdigest()
if digest(pico)!=manifest['sha256']:raise RuntimeError('PicoClaw hash mismatch')
shutil.copy2(exe,target/'虾盘.exe')
shutil.copy2(pico,target/'app/runtime/picoclaw/windows-x64/picoclaw.exe')
shutil.copytree(ROOT/'licenses',target/'app/licenses')
for filename in ['LICENSE','THIRD_PARTY.md']:
 shutil.copy2(ROOT/filename,target/'app'/filename)
if args.with_tools:
 tools=json.loads((ROOT/'catalog/portable-tools.json').read_text(encoding='utf-8'))
 for tool in tools:
  source=ROOT/'runtime/tool-library/windows-x64'/tool['id']
  for rel,expected in tool['files'].items():
   if digest(source/rel)!=expected:raise RuntimeError('Portable tool checksum mismatch: '+tool['id'])
  shutil.copytree(source,target/'app/tools/windows-x64'/tool['id'])
 (target/'app/tool-sources').mkdir(parents=True)
 for spec in json.loads((ROOT/'catalog/tool-packages.json').read_text(encoding='utf-8')):
  if 'source_archive' in spec:
   a=spec['source_archive'];source=ROOT/'runtime/tool-library/sources'/a['filename']
   if digest(source)!=a['sha256']:raise RuntimeError('Source archive checksum mismatch')
   shutil.copy2(source,target/'app/tool-sources'/a['filename'])
 shutil.copy2(ROOT/'catalog/tool-packages.json',target/'app/tool-sources/UPSTREAM.json')
lines=['# 虾盘绿色工具清单','', '界面中的工具箱支持搜索、分类和准备状态筛选。此清单由打包时的目录生成。', '', '| 工具 | 用途 | 本包状态 | 官方入口 |', '| --- | --- | --- | --- |']
for t in json.loads((ROOT/'catalog/portable-tools.json').read_text(encoding='utf-8')):
 lines.append(f"| {t['name']} {t['version']} | {t['description']} | {'已内置' if args.with_tools else '未包含工具包'} | [官网]({t['official_url']}) |")
for t in json.loads((ROOT/'catalog/tool-library.json').read_text(encoding='utf-8')):
 lines.append(f"| {t['name']} | {t['description']} | 选装，未内置 | [官网]({t['official_url']}) |")
lines+=['', 'Windows 系统工具：任务管理器、资源监视器、事件查看器、磁盘清理、设备管理器、系统信息。', '', '启动救援和离线驱动在工具箱中单独列出，尚未制作为启动盘。', '', '工具配置保存在相应工具目录内；更新时保留 data/ 和 app/tools/ 内的个人配置。']
(target/'app/工具清单.md').write_text('\n'.join(lines)+'\n',encoding='utf-8')
(target/'app/使用说明.txt').write_text('双击根目录的 虾盘.exe，点顶部工具进入可搜索分类的工具箱。\n模型设置可选择虾盘云或自带 OpenAI 兼容 API。\n设置、密钥及备份、对话保存在 data/，请妥善保管。\n工具清单见本目录 工具清单.md。选装项目只提供官方入口，并未内置。\n退出应用和打开的第三方工具后安全移除 U 盘；此包不含离线模型、驱动、救援镜像，也不是可启动系统盘。\n更新请保留 data/ 和 app/tools/ 内各工具的个人配置；不要覆盖为初始模板。\n',encoding='utf-8')
files={p.relative_to(target).as_posix():{'bytes':p.stat().st_size,'sha256':digest(p)} for p in target.rglob('*') if p.is_file()}
info={'version':build_version,'source_commit':subprocess.check_output(['git','rev-parse','HEAD'],cwd=ROOT,text=True).strip(),'pico_version':manifest['version'],'portable_tools_included':args.with_tools,'files':files,'user_data_included':False}
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
