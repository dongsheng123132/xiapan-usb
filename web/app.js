'use strict';
const token=document.querySelector('meta[name="xiapan-token"]').content;
const $=id=>document.getElementById(id),el=(tag,text,cls,raw=false)=>{const n=document.createElement(tag);if(text!==undefined){if(raw||tag==='pre'||tag==='code')n.textContent=text;else ui(n,text);}if(cls)n.className=cls;return n;};
const labels={windows:'Windows',darwin:'macOS',linux:'Linux',amd64:'x64',arm64:'ARM64'},kinds={packages:'软件包',images:'镜像',drivers:'驱动',runtime:'运行时'};const names={'startup.inspect':'开机启动项','system.inspect':'系统体检','processes.inspect':'进程内存','network.diagnose':'网络诊断','network.inspect':'网卡信息','drivers.inspect':'网络设备与驱动','tools.catalog':'电脑维护工具包','tools.launch':'系统工具启动','resources.scan':'盘内资源','tools.install':'便携工具安装','report.create':'保存体检报告','reports.list':'历史报告'};
let session=null,wallet=null,busy=false,expert=false;
const runtimeInfo={pico:null,cache:null};
const gib=n=>Number.isFinite(n)?`${(n/1073741824).toFixed(1)} GB`:t('未获得'),bytes=n=>n>=1073741824?gib(n):`${(n/1048576).toFixed(1)} MB`;
function toast(t){ui($('toast'),t);$('toast').classList.remove('hidden');setTimeout(()=>$('toast').classList.add('hidden'),4200);}
async function run(action,input={}){const resp=await fetch('/api/run',{method:'POST',headers:{'Content-Type':'application/json','X-Xiapan-Token':token},body:JSON.stringify({action,input:{...input,locale:I18n.language}})});if(!resp.ok)throw Error(t('请求未完成 ({status})',{status:resp.status}));const r=await resp.json();if(!r.ok)throw Error(t(r.error||'操作未完成'));return r.data;}
function dialog(title,note){const c=$('dialog-content');c.replaceChildren(el('div','XIAPAN / MAINTENANCE','eyebrow'),el('h2',title));if(note)c.append(el('p',note));if(!$('dialog').open)$('dialog').showModal();return c;}
function fail(e){toast(e.message||String(e));}
function button(text,action,fn,cls='primary'){const b=el('button',text,cls);if(action)b.dataset.actionId=action;b.onclick=async()=>{b.disabled=true;try{await fn();}catch(e){fail(e);}finally{b.disabled=false;}};return b;}
function detail(parent,data){const d=el('details',undefined,'result-detail');d.append(el('summary','查看真实结果 JSON'),el('pre',JSON.stringify(data,null,2)));parent.append(d);}
function table(parent,head,rows){const t=el('table'),th=el('tr');for(const h of head)th.append(el('th',h));const thead=el('thead');thead.append(th);const tb=el('tbody');for(const row of rows){const tr=el('tr');for(const cell of row)tr.append(el('td',String(cell??''),undefined,true));tb.append(tr);}t.append(thead,tb);const wrap=el('div',undefined,'table-wrap');wrap.append(t);parent.append(wrap);}
function result(parent,id,d){
 if(id==='system.inspect'){parent.append(el('p',msg('{system} · {count} 个逻辑处理器',{system:`${labels[d.os]||d.os} ${labels[d.arch]||d.arch}`,count:d.cpu_threads})));const stats=el('div',undefined,'stats');stats.append(el('div',msg('总内存 {size}',{size:gib(d.memory_bytes)})),el('div',msg('可用内存 {size}',{size:gib(d.available_memory_bytes)})));parent.append(stats);table(parent,['磁盘','文件系统','可用 / 总量'],(d.volumes||[]).map(v=>[`${v.path} ${v.label||''}`,v.file_system,`${gib(v.free_bytes)} / ${gib(v.size_bytes)}`]));}
 else if(id==='startup.inspect'){table(parent,['名称','来源','状态'],(d.items||[]).map(x=>[x.name,t(x.source),t(({enabled:'已启用',disabled:'已禁用',unknown:'待核实'})[x.state]||'待核实')]));parent.append(el('p',d.note));for(const w of d.warnings||[])parent.append(el('p',w));parent.append(button('打开任务管理器','tools.launch',()=>openTool('task-manager'),'secondary'));}
 else if(id==='processes.inspect'){table(parent,['进程','PID','工作集内存'],(d.processes||[]).slice(0,10).map(p=>[p.name,p.pid,bytes(p.memory_bytes)]));parent.append(el('p',d.note||'只读查看，没有结束进程。'));}
 else if(id==='network.diagnose'){parent.append(el('p',msg('DNS：{dns} · 云端 HTTPS：{https}',{dns:t((d.checks||[])[0]?.ok?'解析成功':'未成功'),https:t((d.checks||[])[1]?.ok?'可达':'未确认')})),el('p',d.note||'仅检测，不修改网络配置。'));}
 else if(id==='drivers.inspect'){table(parent,['设备','故障码','驱动 / INF'],(d.devices||[]).map(n=>[n.name,n.error_code,`${n.driver_version||t('未读到')} / ${n.inf_name||t('未读到')}`]));parent.append(el('p',msg('本盘驱动文件 {count} 个，尚未自动匹配或安装。',{count:(d.offline_files||[]).length})));for(const n of d.devices||[]){const info=el('details');info.append(el('summary',msg('{name} · 硬件 ID',{name:n.name||t('未知设备')})),el('pre',(n.hardware_ids||[]).join('\n')||t('未读到硬件 ID')));parent.append(info);}parent.append(el('p',d.note));}
 else if(id==='tools.catalog'){parent.append(el('p',msg('系统工具 {system} 项可用 · 本盘工具 {portable} 项已准备',{system:(d.builtin_tools||[]).filter(t=>t.ready).length,portable:(d.portable_tools||[]).filter(t=>t.ready).length})));for(const tool of [...(d.portable_tools||[]),...(d.builtin_tools||[])]){parent.append(el('p',`${t(tool.name)}${tool.version?' '+tool.version:''} · ${t(tool.ready?tool.source:'未包含当前平台的工具包')}`));if(tool.ready)parent.append(button(msg('打开 {name}',{name:t(tool.name)}),'tools.launch',()=>openTool(tool.id),'secondary'));}parent.append(el('p',d.note));}
 else if(id==='tools.launch'){parent.append(el('p',`${t(d.name)}: ${t(d.note)}`));}
 else if(id==='resources.scan'){const stats=el('div',undefined,'stats');for(const [key,name] of Object.entries(kinds)){const count=(d.files||[]).filter(f=>f.kind===key).length;if(count)stats.append(el('div',msg('{name} {count} 项',{name:t(name),count})));}parent.append(stats);for(const f of (d.files||[]).slice(0,8)){const row=el('div',undefined,'file-row');row.append(el('span',f.path,undefined,true),el('span',bytes(f.size_bytes)));parent.append(row);}if(!(d.files||[]).length)parent.append(el('p','没有发现资源文件；请按标准目录提前放入。'));}
 else if(id==='tools.install'){parent.append(el('p','文件校验与实际启动验证已通过。'));}
 else if(id==='report.create'){parent.append(el('p','新的报告已保存到本盘 data/reports。'));}
 else if(id==='reports.list'){parent.append(el('p',msg('本盘有 {count} 份体检报告。',{count:(d||[]).length})));}
 detail(parent,d);
}
function render(){if(!session)return;$('task-title').textContent=session.messages.length?session.title:t('新对话');$('welcome').classList.toggle('hidden',session.messages.length>0);const box=$('messages');box.replaceChildren();const traces=[];
 for(const m of session.messages){const c=el('article',undefined,`message ${m.role}`);c.dataset.messageId=m.id;
  if(m.role==='user'||m.role==='assistant'||m.role==='notice'){c.append(el('div',m.role==='user'?'你':m.role==='notice'?'提示':'虾盘','message-label'),el('div',m.text,'message-text',true));}
  else if(m.role==='action'){c.className='action-card';c.append(el('div','✓ '+(names[m.action]||m.action),'action-heading'));result(c,m.action,m.data);traces.push(m);}
  else if(m.role==='proposal'){c.className='proposal-card';c.append(el('div',m.completed?'✓ 已完成':'等待你确认','action-heading'),el('p',m.text));if(!m.completed&&session.pending.some(p=>p.id===m.proposal_id)){const b=button('确认执行',m.action,()=>confirm(m.proposal_id));b.disabled=busy;c.append(b);} }
  box.append(c);
 }
 const trace=$('trace-content');trace.replaceChildren();for(const m of traces){const c=el('details',undefined,'trace-item');c.append(el('summary',m.action),el('pre',JSON.stringify(m.data,null,2)));trace.append(c);}ui($('trace-count'),'{count} 项动作',{count:traces.length});box.lastElementChild?.scrollIntoView({block:'nearest'});
}
function setBusy(value,text='● 正在处理当前任务…'){busy=value;$('send').disabled=value;$('new-chat').disabled=value;$('question').disabled=value;$('engine-select').disabled=value;$('busy-status').classList.toggle('hidden',!value);ui($('busy-status'),text);$('language-select').disabled=value;for(const b of document.querySelectorAll('.proposal-card button'))b.disabled=value;}
async function listSessions(){const rows=await run('chat.list'),c=$('session-list');c.replaceChildren();for(const row of rows){const b=el('button',undefined,'session-item');ui(b,row.untitled?'新对话':row.title);if(session&&row.id===session.id)b.classList.add('active');b.onclick=async()=>{if(busy)return;try{session=await run('chat.read',{session_id:row.id});showView('chat');render();await listSessions();closeSidebarOnNarrow();}catch(e){fail(e);}};c.append(b);}return rows;}
async function newChat(){if(busy)return;session=await run('chat.start');showView('chat');render();await listSessions();$('question').focus();}
async function send(message){if(busy||!message.trim())return;if(!session)await newChat();if(busy)return;showView('chat');setBusy(true,'● 正在检测与回答…');
 const progressTimer=setInterval(async()=>{try{const p=await run('chat.progress',{session_id:session.id});if(p){const saved=session;session=p;render();session=saved;}}catch{}},800);
 const local=el('article',undefined,'message user');local.append(el('div','你','message-label'),el('div',message,'message-text',true));$('messages').append(local);$('welcome').classList.add('hidden');local.scrollIntoView({block:'nearest'});$('question').value='';
 try{session=await run('chat.send',{session_id:session.id,expected_version:session.version,message,engine:$('engine-select').value,model:modelSettings?.model||''});render();await listSessions();}catch(e){fail(e);try{session=await run('chat.read',{session_id:session.id});render();}catch{} }finally{clearInterval(progressTimer);setBusy(false);$('question').focus();refreshRuntimeInfo().catch(()=>{});}
}
async function confirm(proposalID){if(busy)return;setBusy(true,'● 正在执行已确认的操作并验证…');try{session=await run('chat.confirm',{session_id:session.id,expected_version:session.version,proposal_id:proposalID,confirmed:true});render();}finally{setBusy(false);}}
function engineNote(){const value=$('engine-select').value;ui($('engine-note'),value==='pico'?msg('{model} · {source} · 设置与对话随盘保存',{model:modelSettings?.model||t('选择模型'),source:t(modelSettings?.source==='custom'?'自带 API':'虾盘云')}):'本地工具 · 无需模型');}
async function openTool(id){const d=await run('tools.launch',{tool_id:id,confirmed:true});toast(msg('{name} 已提交启动',{name:t(d.name)}));}
function walletMini(){if(!wallet)return;ui($('wallet-mini-note'),wallet.ready?(wallet.available_tokens!==undefined?msg('余额 {count} token',{count:Number(wallet.available_tokens).toLocaleString(I18n.language)}):'钱包已保存 · 余额待查询'):'联网获取 · 或填入已有钱包');}
async function walletView(){if(!wallet){try{wallet=await run('wallet.ensure');walletMini();}catch(e){wallet={ready:false,note:e.message};}}
 const c=dialog('设备钱包','免注册，凭证就是钱包。请备份凭证，随盘丢失也会失去访问入口。');c.append(el('div',wallet.ready?wallet.masked_key:'尚未获取钱包','wallet-key'),el('div',wallet.available_tokens!==undefined?`${Number(wallet.available_tokens).toLocaleString()} token`:'余额未获得','wallet-amount'),el('p',wallet.note||''));
 const row=el('div',undefined,'button-row');row.append(button('充值 ↗','wallet.recharge',async()=>{await run('wallet.recharge');toast('已在系统浏览器打开充值页，请自行选择与付款');}),button('刷新余额','wallet.refresh',async()=>{wallet=await run(wallet.ready?'wallet.refresh':'wallet.ensure');walletMini();await walletView();},'secondary'));if(wallet.ready)row.append(button('备份凭证','wallet.copy',async()=>{const d=await run('wallet.copy',{confirmed:true});try{await navigator.clipboard.writeText(d.api_key);toast('凭证已复制，请保存到你自己的安全位置');}catch{const reveal=el('input');reveal.value=d.api_key;reveal.readOnly=true;reveal.type='text';attr(reveal,'aria-label','待备份的钱包凭证');c.append(reveal);reveal.select();toast('请手动复制；关闭窗口后不再展示');}},'secondary'));c.append(row);
 const adopt=el('details',undefined,'wallet-adopt');adopt.append(el('summary','填入已有钱包'));const input=el('input');input.type='password';input.autocomplete='off';attr(input,'placeholder','粘贴 sk- 开头的钱包凭证');attr(input,'aria-label','已有钱包凭证');adopt.append(input,button('验证并使用','wallet.adopt',async()=>{wallet=await run('wallet.adopt',{api_key:input.value});input.value='';walletMini();await walletView();}));c.append(adopt);
 if(wallet.ready){const danger=el('details',undefined,'wallet-danger');danger.append(el('summary','钱包管理'));danger.append(button('换一把凭证','wallet.rotate',async()=>{const p=dialog('确认换一把凭证','余额保留在同一钱包，完成后旧凭证会失效。请重新备份新凭证。');p.append(button('确认换一把','wallet.rotate',async()=>{wallet=await run('wallet.rotate',{confirmed:true});walletMini();await walletView();}));},'text-button'));danger.append(button('从本盘移除钱包','wallet.reset',async()=>{const p=dialog('移除本盘钱包','旧钱包和余额保留在服务端。创建的新钱包不会带入旧余额。');const label=el('label'),check=el('input');check.type='checkbox';label.append(check,el('span','我已备份旧钱包凭证'));p.append(label,button('确认移除','wallet.reset',async()=>{if(!check.checked)throw Error('请先备份并勾选确认');wallet=await run('wallet.reset',{confirmed:true,backed_up:true});walletMini();await walletView();}));},'text-button'));c.append(danger);}
}
async function maintenanceAction(action){
 const c=dialog(names[action]||action),status=el('div');status.append(el('p','正在读取当前电脑的信息…'));c.append(status);
 try{const data=await run(action);if(!status.isConnected||!$('dialog').open)return;status.replaceChildren();result(status,action,data);}catch(e){if(status.isConnected){status.replaceChildren(el('p',e.message));}}
}
async function offline(){
 const c=dialog('断网急救','无需模型、API Key 或钱包额度，直接检查电脑并打开维护工具。');
 const row=el('div',undefined,'button-row');
 for(const [label,action] of [['系统体检','system.inspect'],['识别网卡与驱动','drivers.inspect'],['检查网络','network.diagnose']])row.append(button(label,action,()=>maintenanceAction(action),'secondary'));
 row.append(button('查看电脑维护工具包','tools.catalog',()=>{$('dialog').close();showView('tools');},'secondary'));
 c.append(row,el('p','工具按当前系统显示。Windows 的维护程序不能在 Mac 上运行；未准备的工具会标明状态。'),el('p','试试有线连接或手机 USB 共享网络。需要驱动时，在有网电脑从整机/网卡厂商官网按硬件 ID 准备对应系统的包，放入 drivers/；当前不会自动匹配或安装。'));
 c.append(button('使用维护向导','chat.send',()=>{$('engine-select').value='guide';engineNote();$('dialog').close();showView('chat');},'secondary'));
}
const cacheLabels={not_used:'缓存按需准备',copied:'程序已缓存到电脑',hit:'正在复用电脑缓存',usb_fallback:'从 U 盘运行',unavailable:'缓存暂不可用'};
function picoRunning(pico){return !!(pico&&(pico.running===true||pico.gateway_running===true||pico.gateway?.running===true||['running','ready'].includes(pico.status)));}
function runtimeLabels(){
 const pico=runtimeInfo.pico,cache=runtimeInfo.cache;
 ui($('pico-runtime-state'),picoRunning(pico)?'PicoClaw 已运行 · 退出后停止':'完整功能按需启动');
 if(cache)ui($('cache-open'),msg('{state} · 查看状态',{state:t(cacheLabels[cache.status]||'程序缓存')}));
 if(cache?.portable_root){$('drive-status').textContent=cache.portable_root;$('drive-status').title=cache.portable_root;}
}
async function refreshRuntimeInfo(){
 const results=await Promise.allSettled([run('pico.status'),run('runtime.cache.status')]);
 if(results[0].status==='fulfilled')runtimeInfo.pico=results[0].value;
 if(results[1].status==='fulfilled')runtimeInfo.cache=results[1].value;
 runtimeLabels();return results;
}
async function runtimeView(){
 const c=dialog('运行与缓存','程序可在这台电脑的本地缓存运行；API Key、钱包与工作内容随 U 盘保存。');
 c.append(el('p','首次复制需要读取 U 盘；后续复用缓存。首次可用时间与 AI 回答速度分别计算。'),el('p','关闭网页不会退出 PicoClaw。完成任务后请点右上角“退出”，再安全移除 U 盘。'));
 const status=el('div');c.append(status);await refreshRuntimeInfo();
 if(!status.isConnected)return;
 const pico=el('section');pico.append(el('h3','PicoClaw 运行状态'),el('p',runtimeInfo.pico?(picoRunning(runtimeInfo.pico)?'正在运行，可继续使用完整功能':'尚未运行，聊天或打开完整界面时启动'):'状态暂不可用；应急维护工具仍可打开。'));status.append(pico);
 const cache=el('section'),data=runtimeInfo.cache;cache.append(el('h3','程序缓存状态'));
 if(data){cache.append(el('p',cacheLabels[data.status]||'缓存按需准备'));if(data.reason)cache.append(el('p',data.reason));if(data.directory){cache.append(el('p','这台电脑的缓存位置'),el('code',data.directory));}if(data.note)cache.append(el('p',data.note));}else cache.append(el('p','状态暂不可用；应急维护工具仍可打开。'));status.append(cache);
}
async function openPicoUI(){
 const b=$('pico-open');b.disabled=true;ui(b,'正在启动完整 PicoClaw…');
 try{const data=await run('pico.ui.open');if(data.opened)toast('完整 PicoClaw 已在浏览器打开；当前对话保留');else if(data.url){const url=new URL(data.url);if(!['http:','https:'].includes(url.protocol)||!['localhost','127.0.0.1','[::1]'].includes(url.hostname))throw Error(t('完整 PicoClaw 返回了无效的本地地址'));const c=dialog('完整 PicoClaw','工作台已启动，点击下方入口继续。');const a=el('a','打开完整工作台 ↗','secondary');a.href=url.href;a.target='_blank';a.rel='noreferrer';c.append(a);}else throw Error(data.note||t('完整 PicoClaw 尚未准备好'));await refreshRuntimeInfo();}catch(e){fail(e);}finally{b.disabled=false;ui(b,'完整 PicoClaw ↗');}
}
async function resources(){const d=await run('resources.scan'),c=$('resources-content');c.replaceChildren();for(const [kind,name] of Object.entries(kinds)){const files=d.files.filter(f=>f.kind===kind);if(!files.length)continue;const card=el('article',undefined,'resource-group');card.append(el('h3',msg('{name} · {count} 项',{name:t(name),count:files.length})));for(const f of files){const row=el('div',undefined,'file-row');row.append(el('span',f.path,undefined,true),el('span',bytes(f.size_bytes)));card.append(row);}if(!files.length)card.append(el('p',msg('请把{name}文件提前放进 {directory} 目录。',{name:t(name),directory:kind+'/'})));c.append(card);}for(const w of d.warnings)c.append(el('p',w));}
async function toolCollection(){renderToolLibrary(await run('tools.catalog'));}
const SIDEBAR_KEY='xiapan.sidebar.collapsed';
const narrow=()=>window.matchMedia('(max-width:780px)').matches;
function applySidebar(){
  const collapsed=$('sidebar-toggle').getAttribute('aria-expanded')==='false';
  document.body.classList.toggle('sidebar-collapsed',collapsed&&!narrow());
}
function setSidebarCollapsed(collapsed,remember=true){
  $('sidebar-toggle').setAttribute('aria-expanded',String(!collapsed));
  applySidebar();
  if(remember){try{localStorage.setItem(SIDEBAR_KEY,collapsed?'1':'0');}catch{}}
}
function closeSidebarOnNarrow(){if(narrow()){document.body.classList.remove('history-open');$('history-open').setAttribute('aria-expanded','false');}}
function showView(view){closeSidebarOnNarrow();for(const n of document.querySelectorAll('.view'))n.classList.toggle('hidden',n.id!==`${view}-view`);for(const b of document.querySelectorAll('.nav-item'))b.classList.toggle('active',b.dataset.view===view);if(view==='resources')resources().catch(fail);if(view==='tools')toolCollection().catch(fail);}
const commands=[['/启动项','开机自启清单与状态'],['/体检','系统与内存、磁盘'],['/进程','进程工作集内存'],['/网络','网卡、DNS、云端 HTTPS'],['/驱动','硬件 ID、故障码、盘内驱动'],['/工具','可打开的系统工具与资源集合'],['/资源','盘内软件、镜像与驱动'],['/安装','便携体检工具 · 确认后安装'],['/报告','新的体检报告 · 确认后保存'],['/救援','无法启动时的准备方案']];
for(const [cmd,description] of commands){const b=el('button',undefined,'palette-command');b.append(el('code',t(cmd)),el('span',t(description)));b.onclick=()=>{$('palette').close();send(cmd).catch(fail);};$('palette-commands').append(b);}
document.addEventListener('click',e=>{const b=e.target.closest('button');if(!b||b.disabled)return;if(b.dataset.view)showView(b.dataset.view);if(b.dataset.prompt){$('question').value=t(b.dataset.prompt);$('question').focus();}if(b.dataset.command)send(b.dataset.command).catch(fail);});
$('chat-form').onsubmit=e=>{e.preventDefault();send($('question').value).catch(fail);};$('question').onkeydown=e=>{if(e.key==='Enter'&&!e.shiftKey&&!e.isComposing){e.preventDefault();send($('question').value).catch(fail);}};
$('engine-select').onchange=engineNote;$('model-settings').onclick=()=>modelSettingsView().catch(fail);$('history-open').onclick=()=>{const open=document.body.classList.toggle('history-open');$('history-open').setAttribute('aria-expanded',String(open));syncSidebarInert();};$('sidebar-toggle').onclick=()=>{setSidebarCollapsed($('sidebar-toggle').getAttribute('aria-expanded')!=='false');};window.addEventListener('resize',applySidebar);window.addEventListener('resize',syncSidebarInert);$('new-chat').onclick=()=>newChat().catch(fail);$('expert-toggle').onclick=()=>{expert=!expert;$('trace-panel').classList.toggle('hidden',!expert);ui($('expert-toggle'),expert?'关闭专业模式':'专业模式');};$('wallet-open').onclick=()=>walletView().catch(fail);$('wallet-top-open').onclick=()=>walletView().catch(fail);$('offline-open').onclick=()=>offline().catch(fail);$('pico-open').onclick=openPicoUI;$('cache-open').onclick=()=>runtimeView().catch(fail);$('dialog-close').onclick=()=>{$('dialog').close();$('dialog-content').replaceChildren();};$('dialog').addEventListener('close',()=>{$('dialog-content').replaceChildren();});$('palette-open').onclick=()=>$('palette').showModal();$('palette-close').onclick=()=>$('palette').close();
$('reports-open').onclick=async()=>{try{const d=await run('reports.list'),c=dialog('本盘维护记录',msg('共 {count} 份报告。',{count:d.length}));for(const r of d){const a=el('article',undefined,'action-card');a.append(el('h3',new Date(r.created_at).toLocaleString(I18n.language)));detail(a,r);c.append(a);}}catch(e){fail(e);}};
$('skills-open').onclick=async()=>{try{const r=await (await fetch('/api/skills')).json();if(!r.ok)throw Error(r.error);const c=dialog('维护技能','技能规定维护步骤，执行能力由真实工具提供。');for(const s of r.data){const d=el('details',undefined,'skill-card');d.append(el('summary',s.id),el('pre',s.content));c.append(d);}}catch(e){fail(e);}};
document.addEventListener('keydown',e=>{if((e.ctrlKey||e.metaKey)&&e.key.toLowerCase()==='k'){e.preventDefault();$('palette').showModal();}if((e.ctrlKey||e.metaKey)&&e.shiftKey&&e.key.toLowerCase()==='n'){e.preventDefault();newChat().catch(fail);}});
$('quit').onclick=async()=>{try{const r=await fetch('/api/quit',{method:'POST',headers:{'X-Xiapan-Token':token}});if(!r.ok)throw Error('退出未完成');dialog('虾盘已退出','本次启动的 PicoClaw 已停止。请关闭维护工具，再安全移除 U 盘。');}catch(e){fail(e);}};
document.addEventListener('languagechange',()=>{render();engineNote();walletMini();runtimeLabels();if(session)listSessions().catch(fail);for(const [index,[cmd]] of commands.entries()){const code=$('palette-commands').children[index]?.querySelector('code');if(code)code.textContent=t(cmd);}});
// The sidebar ships with `inert` so it is not focusable before scripts run.
// Once JS owns it, desktop keeps it interactive and narrow screens use the drawer.
function syncSidebarInert(){const bar=document.querySelector('.sidebar');if(!bar)return;const drawer=narrow()&&!document.body.classList.contains('history-open');bar.inert=drawer;}
window.addEventListener('resize',syncSidebarInert);
// Paint and wire the portable interface before loading optional local state.
// No system inspection, wallet/network call, hashing or Gateway start on this path.
try{const saved=localStorage.getItem(SIDEBAR_KEY);setSidebarCollapsed(saved==='1',false);}catch{setSidebarCollapsed(false,false);}
syncSidebarInert();engineNote();
I18n.init().catch(fail);
loadSettings().then(engineNote).catch(fail);
listSessions().catch(fail);
refreshRuntimeInfo().catch(()=>{});
// Sessions and wallets are created only after an explicit user action.
