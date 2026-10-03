'use strict';
const token=document.querySelector('meta[name="xiapan-token"]').content;
const $=id=>document.getElementById(id),el=(tag,text,cls,raw=false)=>{const n=document.createElement(tag);if(text!==undefined){if(raw||tag==='pre'||tag==='code')n.textContent=text;else ui(n,text);}if(cls)n.className=cls;return n;};
const labels={windows:'Windows',darwin:'macOS',linux:'Linux',amd64:'x64',arm64:'ARM64'};const names={'startup.inspect':'开机启动项','system.inspect':'系统体检','processes.inspect':'进程内存','network.rescue':'断网急救诊断','network.inspect':'网卡信息','drivers.inspect':'网络设备与驱动','tools.catalog':'维护工具集合','tools.launch':'系统工具启动','report.create':'保存体检报告','reports.list':'历史报告'};
let session=null,system=null,busy=false,expert=false;
const gib=n=>Number.isFinite(n)?`${(n/1073741824).toFixed(1)} GB`:t('未获得'),bytes=n=>n>=1073741824?gib(n):`${(n/1048576).toFixed(1)} MB`;
function toast(t){ui($('toast'),t);$('toast').classList.remove('hidden');setTimeout(()=>$('toast').classList.add('hidden'),4200);}
async function run(action,input={}){const resp=await fetch('/api/run',{method:'POST',headers:{'Content-Type':'application/json','X-Xiapan-Token':token},body:JSON.stringify({action,input:{...input,locale:I18n.language}})});if(!resp.ok)throw Error(t('请求未完成 ({status})',{status:resp.status}));const r=await resp.json();if(!r.ok)throw Error(t(r.error||'操作未完成'));return r.data;}
function dialog(title,note){const c=$('dialog-content');c.replaceChildren(el('div','XIAPAN / MAINTENANCE','eyebrow'),el('h2',title));if(note)c.append(el('p',note));if(!$('dialog').open)$('dialog').showModal();return c;}
function fail(e){toast(e.message||String(e));}
function button(text,action,fn,cls='primary'){const b=el('button',text,cls);if(action)b.dataset.actionId=action;b.onclick=async()=>{b.disabled=true;try{await fn();}catch(e){fail(e);}finally{b.disabled=false;}};return b;}
function detail(parent,data){const d=el('details',undefined,'result-detail');d.append(el('summary','查看真实结果 JSON'),el('pre',JSON.stringify(data,null,2)));parent.append(d);}
function table(parent,head,rows){const t=el('table'),th=el('tr');for(const h of head)th.append(el('th',h));const thead=el('thead');thead.append(th);const tb=el('tbody');for(const row of rows){const tr=el('tr');for(const cell of row)tr.append(el('td',String(cell??''),undefined,true));tb.append(tr);}t.append(thead,tb);const wrap=el('div',undefined,'table-wrap');wrap.append(t);parent.append(wrap);}
const networkTests=[['gateway_reachable','网关（路由器）可达'],['internet_reachable','外网 IP 可达'],['dns_works','DNS 域名解析'],['web_direct','直连能打开网页'],['web_via_proxy','经系统代理能打开网页']];
function networkCard(parent,d){
 const verdict=el('p');verdict.append(el('strong',d.verdict));parent.append(verdict);
 const list=el('ul',undefined,'network-tests');
 for(const [key,label] of networkTests){const ok=d.tests?.[key]===true,li=el('li');li.append(el('span',ok?'✓':'✗',ok?'test-ok':'test-bad',true),el('span',label));list.append(li);}
 parent.append(list,el('p',d.proxy?.enabled?msg('系统代理：已开启 → {server}',{server:d.proxy.server||''}):'系统代理：未开启'));
 if(d.hosts?.suspicious)parent.append(el('p',msg('hosts 提醒：系统 hosts 文件里有 {count} 条对主流网站的自定义指向，请核对公司批准的配置和变更记录。',{count:(d.hosts.hits||[]).length})),el('pre',(d.hosts.hits||[]).join('\n')));
 if((d.adapters||[]).length)table(parent,['网卡','IPv4','网关','DNS'],d.adapters.map(a=>[a.name,a.ipv4,a.gateway,a.dns]));
 // Opening the tool is the existing tools.launch path; this app changes no network setting itself.
 if(d.next_step)parent.append(el('p',d.next_step.reason),button(msg('打开 {name}',{name:t(d.next_step.name)}),'tools.launch',()=>openTool(d.next_step.tool_id),'secondary'));
}
function result(parent,id,d){
 if(id==='system.inspect'){parent.append(el('p',msg('{system} · {count} 个逻辑处理器',{system:`${labels[d.os]||d.os}${d.os_version?' '+d.os_version:''} ${labels[d.arch]||d.arch}${d.cpu_model?' · '+d.cpu_model:''}`,count:d.cpu_threads})));const stats=el('div',undefined,'stats');stats.append(el('div',msg('总内存 {size}',{size:gib(d.memory_bytes)})),el('div',msg('可用内存 {size}',{size:gib(d.available_memory_bytes)})));parent.append(stats);table(parent,['磁盘','文件系统','可用 / 总量'],(d.volumes||[]).map(v=>[`${v.path} ${v.label||''}`,v.file_system,`${gib(v.free_bytes)} / ${gib(v.size_bytes)}`]));}
 else if(id==='startup.inspect'){table(parent,['名称','来源','状态'],(d.items||[]).map(x=>[x.name,t(x.source),t(({enabled:'已启用',disabled:'已禁用',unknown:'待核实'})[x.state]||'待核实')]));parent.append(el('p',d.note));for(const w of d.warnings||[])parent.append(el('p',w));const [label,tool]=['打开任务管理器','task-manager'];parent.append(button(label,'tools.launch',()=>openTool(tool),'secondary'));}
 else if(id==='processes.inspect'){table(parent,['进程','PID','工作集内存'],(d.processes||[]).slice(0,10).map(p=>[p.name,p.pid,bytes(p.memory_bytes)]));parent.append(el('p',d.note||'只读查看，没有结束进程。'));}
 else if(id==='drivers.inspect'){table(parent,['设备','故障码','驱动 / INF'],(d.devices||[]).map(n=>[n.name,n.error_code,`${n.driver_version||t('未读到')} / ${n.inf_name||t('未读到')}`]));for(const n of d.devices||[]){const info=el('details');info.append(el('summary',msg('{name} · 硬件 ID',{name:n.name||t('未知设备')})),el('pre',(n.hardware_ids||[]).join('\n')||t('未读到硬件 ID')));parent.append(info);}parent.append(el('p',d.note));}
 else if(id==='tools.catalog'){parent.append(el('p',msg('Windows 系统工具 {count} 项可用',{count:(d.builtin_tools||[]).filter(t=>t.ready).length})));for(const tool of (d.builtin_tools||[])){parent.append(el('p',`${t(tool.name)}${tool.version?' '+tool.version:''} · ${t(tool.ready?tool.source:'未包含当前平台的工具包')}`));if(tool.ready)parent.append(button(msg('打开 {name}',{name:t(tool.name)}),'tools.launch',()=>openTool(tool.id),'secondary'));}parent.append(el('p',d.note));}
 else if(id==='tools.launch'){parent.append(el('p',`${t(d.name)}: ${t(d.note)}`));}
 else if(id==='report.create'){parent.append(el('p','新的报告已保存到本盘 data/reports。'));}
 else if(id==='reports.list'){parent.append(el('p',msg('本盘有 {count} 份体检报告。',{count:(d||[]).length})));}
 else if(id==='network.rescue'){networkCard(parent,d);}
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
function setBusy(value,text='● 正在处理当前任务…'){busy=value;$('send').disabled=value;$('new-chat').disabled=value;$('question').disabled=value;$('busy-status').classList.toggle('hidden',!value);ui($('busy-status'),text);$('language-select').disabled=value;for(const b of document.querySelectorAll('.proposal-card button'))b.disabled=value;}
async function listSessions(){const rows=await run('chat.list'),c=$('session-list');c.replaceChildren();for(const row of rows){const b=el('button',undefined,'session-item');ui(b,row.untitled?'新对话':row.title);if(session&&row.id===session.id)b.classList.add('active');b.onclick=async()=>{if(busy)return;try{session=await run('chat.read',{session_id:row.id});showView('chat');render();await listSessions();closeSidebarOnNarrow();}catch(e){fail(e);}};c.append(b);}return rows;}
async function newChat(){if(busy)return;session=await run('chat.start');showView('chat');render();await listSessions();$('question').focus();}
async function send(message){if(busy||!message.trim())return;if(!session)await newChat();showView('chat');setBusy(true,'● 正在检测与回答…');
 const progressTimer=setInterval(async()=>{try{const p=await run('chat.progress',{session_id:session.id});if(p){const saved=session;session=p;render();session=saved;}}catch{}},800);
 const local=el('article',undefined,'message user');local.append(el('div','你','message-label'),el('div',message,'message-text',true));$('messages').append(local);$('welcome').classList.add('hidden');local.scrollIntoView({block:'nearest'});$('question').value='';
 try{session=await run('chat.send',{session_id:session.id,expected_version:session.version,message});render();await listSessions();}catch(e){fail(e);try{session=await run('chat.read',{session_id:session.id});render();}catch{} }finally{clearInterval(progressTimer);setBusy(false);$('question').focus();}
}
async function confirm(proposalID){if(busy)return;setBusy(true,'● 正在执行已确认的操作并验证…');try{session=await run('chat.confirm',{session_id:session.id,expected_version:session.version,proposal_id:proposalID,confirmed:true});render();}finally{setBusy(false);}}
function engineNote(){ui($('engine-note'),modelSettings?.model?'AI 分析使用公司 API；检测按钮无需联网模型。':'检测按钮直接可用；需要 AI 分析时再配置公司 API。');}
async function openTool(id){if(busy)return;const d=await run('tools.launch',{tool_id:id,confirmed:true});toast(msg('{name} 已提交启动',{name:t(d.name)}));}
async function inspect(){system=await run('system.inspect');$('computer-tag').textContent=`${labels[system.os]||system.os} ${labels[system.arch]||system.arch} · ${gib(system.memory_bytes)}`;$('drive-status').textContent=system.portable_root;$('drive-status').title=system.portable_root;}
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
function closeSidebarOnNarrow(){if(narrow()){document.body.classList.remove('history-open');$('history-open').setAttribute('aria-expanded','false');syncSidebarInert();}}
function showView(view){closeSidebarOnNarrow();for(const n of document.querySelectorAll('.view'))n.classList.toggle('hidden',n.id!==`${view}-view`);for(const b of document.querySelectorAll('.nav-item'))b.classList.toggle('active',b.dataset.view===view);if(view==='tools')toolCollection().catch(fail);}
const commands=[['/体检','系统与内存、磁盘'],['/进程','进程工作集内存'],['/启动项','开机自启清单与状态'],['/断网','网卡、网关、DNS、网页与代理'],['/驱动','网卡硬件 ID 与故障码'],['/工具','Windows 系统工具'],['/报告','确认后保存当前排障记录']];
for(const [cmd,description] of commands){const b=el('button',undefined,'palette-command');b.append(el('code',t(cmd)),el('span',t(description)));b.onclick=()=>{$('palette').close();send(cmd).catch(fail);};$('palette-commands').append(b);}
document.addEventListener('click',e=>{const b=e.target.closest('button');if(!b||b.disabled)return;if(b.dataset.view)showView(b.dataset.view);if(b.dataset.prompt){$('question').value=b.dataset.prompt;$('question').focus();}if(b.dataset.command)send(b.dataset.command).catch(fail);});
$('chat-form').onsubmit=e=>{e.preventDefault();send($('question').value).catch(fail);};$('question').onkeydown=e=>{if(e.key==='Enter'&&!e.shiftKey&&!e.isComposing){e.preventDefault();send($('question').value).catch(fail);}};
$('model-settings').onclick=()=>modelSettingsView().catch(fail);$('history-open').onclick=()=>{const open=document.body.classList.toggle('history-open');$('history-open').setAttribute('aria-expanded',String(open));syncSidebarInert();};$('sidebar-toggle').onclick=()=>{setSidebarCollapsed($('sidebar-toggle').getAttribute('aria-expanded')!=='false');};window.addEventListener('resize',applySidebar);window.addEventListener('resize',syncSidebarInert);$('new-chat').onclick=()=>newChat().catch(fail);$('expert-toggle').onclick=()=>{expert=!expert;$('trace-panel').classList.toggle('hidden',!expert);ui($('expert-toggle'),expert?'关闭专业模式':'专业模式');};$('dialog-close').onclick=()=>{$('dialog').close();$('dialog-content').replaceChildren();};$('dialog').addEventListener('close',()=>{$('dialog-content').replaceChildren();});$('palette-open').onclick=()=>$('palette').showModal();$('palette-close').onclick=()=>$('palette').close();
$('reports-open').onclick=async()=>{try{const d=await run('reports.list'),c=dialog('本盘维护记录',msg('共 {count} 份报告。',{count:d.length}));for(const r of d){const a=el('article',undefined,'action-card');a.append(el('h3',new Date(r.created_at).toLocaleString(I18n.language)));detail(a,r);c.append(a);}}catch(e){fail(e);}};
$('skills-open').onclick=async()=>{try{const r=await (await fetch('/api/skills')).json();if(!r.ok)throw Error(r.error);const c=dialog('维护技能','技能规定维护步骤，执行能力由真实工具提供。');for(const s of r.data){const d=el('details',undefined,'skill-card');d.append(el('summary',s.id),el('pre',s.content));c.append(d);}}catch(e){fail(e);}};
document.addEventListener('keydown',e=>{if((e.ctrlKey||e.metaKey)&&e.key.toLowerCase()==='k'){e.preventDefault();$('palette').showModal();}if((e.ctrlKey||e.metaKey)&&e.shiftKey&&e.key.toLowerCase()==='n'){e.preventDefault();newChat().catch(fail);}});

$('quit').onclick=async()=>{try{const r=await fetch('/api/quit',{method:'POST',headers:{'X-Xiapan-Token':token}});if(!r.ok)throw Error('退出未完成');dialog('虾盘已退出','可以关闭页面，退出已打开的系统工具后安全移除 U 盘。');}catch(e){fail(e);}};
document.addEventListener('languagechange',()=>{render();engineNote();if(session)listSessions().catch(fail);for(const [index,[cmd]] of commands.entries()){const code=$('palette-commands').children[index]?.querySelector('code');if(code)code.textContent=t(cmd);}});
// The sidebar ships with `inert` so it is not focusable before scripts run.
// Once JS owns it, desktop keeps it interactive and narrow screens use the drawer.
function syncSidebarInert(){const bar=document.querySelector('.sidebar');if(!bar)return;const drawer=narrow()&&!document.body.classList.contains('history-open');bar.inert=drawer;}
window.addEventListener('resize',syncSidebarInert);
(async()=>{try{await I18n.init();}catch(e){fail(e);}try{await loadSettings();}catch(e){fail(e);}try{const saved=localStorage.getItem(SIDEBAR_KEY);setSidebarCollapsed(saved==='1',false);}catch{setSidebarCollapsed(false,false);}syncSidebarInert();engineNote();await inspect();const rows=await listSessions();session=rows.length?await run('chat.read',{session_id:rows[0].id}):await run('chat.start');render();await listSessions();})().catch(fail);
