/* The list is a view of tools.catalog. Execution still uses tools.launch. */
function renderToolLibrary(data) {
  const root=$('tool-collection');root.replaceChildren();
  const nativeCategories={'task-manager':'启动与进程','resource-monitor':'启动与进程','disk-cleanup':'磁盘与硬件','device-manager':'磁盘与硬件','system-info':'磁盘与硬件','event-viewer':'启动与进程','memory-diagnostic':'磁盘与硬件','network-adapters':'网络与连接','network-settings':'网络与连接','mac-activity-monitor':'系统诊断','mac-disk-utility':'磁盘维护','mac-console':'系统诊断','mac-system-information':'系统诊断','mac-wireless-diagnostics':'网络应急'};
  const rows=[
    ...(data.portable_tools||[]).map(t=>({...t,kind:'portable',status:t.ready?'已内置':'未准备'})),
    ...(data.builtin_tools||[]).map(t=>({...t,kind:'system',category:t.category||nativeCategories[t.id]||'系统工具',status:t.ready?'系统自带':'此系统不可用',portable_note:'由当前电脑的系统提供，不占 U 盘工具包空间。'})),
    ...(data.library_tools||[]).map(t=>({...t,kind:'download',status:'待下载'})),
    ...(data.resources||[]).map(t=>({...t,kind:'download',ready:false,status:'需另行准备',category:t.id==='network-driver'?'网络与连接':'启动与救援',reason:t.status,portable_note:t.description}))
  ];
  const intro=el('p',msg('{portable} 款已内置 · {system} 项系统工具 · {optional} 项选装资源',{portable:rows.filter(t=>t.kind==='portable'&&t.ready).length,system:rows.filter(t=>t.kind==='system'&&t.ready).length,optional:rows.filter(t=>t.kind==='download').length}),'library-summary');
  const filters=el('div',undefined,'library-filters'),search=el('input'),category=el('select'),state=el('select');
  search.type='search';attr(search,'placeholder','搜索名称或用途，如：磁盘、压缩、启动项');attr(search,'aria-label','搜索工具');
  attr(category,'aria-label','筛选工具分类');attr(state,'aria-label','筛选准备状态');
  for(const name of ['全部分类',...new Set(rows.map(t=>t.category))]){const o=el('option',t(name));o.value=name;category.append(o);}
  for(const [value,name] of [['all','全部工具'],['ready','可直接打开'],['portable','本盘已内置'],['download','待下载 / 选装']]){const o=el('option',t(name));o.value=value;state.append(o);}
  filters.append(search,category,state);
  const count=el('p',undefined,'library-count'),list=el('div',undefined,'tool-list');count.setAttribute('role','status');attr(list,'aria-label','工具列表');
  root.append(intro,filters,count,list);
  const searchable=tool=>[tool.name,tool.description,tool.category,tool.portable_note].flatMap(value=>[value,t(value)]).join(' ').toLocaleLowerCase();
  function draw(){
    list.replaceChildren();const query=search.value.trim().toLocaleLowerCase();
    const shown=rows.filter(t=>(category.value==='全部分类'||t.category===category.value)&&
      (state.value==='all'||state.value==='ready'&&t.ready||state.value==='portable'&&t.kind==='portable'&&t.ready||state.value==='download'&&!t.ready)&&
      (!query||searchable(t).includes(query)));
    ui(count,msg('显示 {shown} / {total} 项',{shown:shown.length,total:rows.length}));
    for(const tool of shown){
      const row=el('article',undefined,'tool-row');row.dataset.toolId=tool.id;
      const info=el('div',undefined,'tool-info'),title=el('div',undefined,'tool-title');
      title.append(el('h3',tool.name),el('span',t(tool.category),'tool-category'));info.append(title,el('p',tool.description||'查看官方说明'));
      const details=el('details',undefined,'tool-info-details');details.append(el('summary','便携方式与来源'));
      if(tool.portable_note)details.append(el('p',t(tool.portable_note)));
      if(tool.reason)details.append(el('p',t(tool.reason)));
      const metadata=[tool.version,tool.platform,tool.bytes?bytes(tool.bytes):'',tool.license].filter(Boolean).join(' · ');if(metadata)details.append(el('p',metadata,undefined,true));
      if(tool.official_url){const link=el('a','官方说明 ↗');link.href=tool.official_url;link.target='_blank';link.rel='noreferrer';details.append(link);}
      info.append(details);
      const actions=el('div',undefined,'tool-row-actions');actions.append(el('span',t(tool.status),tool.ready?'tool-status ready':'tool-status'));
      if(tool.ready){const b=button('打开 ↗','tools.launch',async()=>{b.disabled=true;try{await openTool(tool.id);}finally{b.disabled=false;}},'secondary');actions.append(b);}
      else if(tool.official_url){const a=el('a','前往下载 ↗','tool-download');a.href=tool.official_url;a.target='_blank';a.rel='noreferrer';actions.append(a);}
      row.append(info,actions);list.append(row);
    }
    if(!shown.length)list.append(el('p','没有匹配的工具，试试其他名称或分类。','tool-empty'));
  }
  search.addEventListener('input',draw);category.addEventListener('change',draw);state.addEventListener('change',draw);draw();
  // Language switch rebuilds the collection so filters and labels are re-rendered.
  if(!renderToolLibrary.bound){renderToolLibrary.bound=true;document.addEventListener('languagechange',()=>{if($('tool-collection').childElementCount)toolCollection().catch(()=>{});});}
}
