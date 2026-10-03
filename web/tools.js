/* The list is a view of tools.catalog. Execution still uses tools.launch. */
function renderToolLibrary(data) {
  const root=$('tool-collection');root.replaceChildren();
  const nativeCategories={'task-manager':'启动与进程','resource-monitor':'启动与进程','disk-cleanup':'磁盘与硬件','device-manager':'磁盘与硬件','system-info':'磁盘与硬件','event-viewer':'启动与进程','activity-monitor':'启动与进程','login-items':'启动与进程','console':'启动与进程','storage-settings':'磁盘与硬件','disk-utility':'磁盘与硬件','system-information':'磁盘与硬件','wireless-diagnostics':'网络与连接'};
  // Windows-only entries keep their official link but are not offered as this computer's tools.
  const foreign=t=>system?.os&&system.os!=='windows'&&/windows/i.test(t.platform||'');
  const rows=[
    ...(data.portable_tools||[]).map(t=>({...t,kind:'portable',status:t.ready?'已内置':foreign(t)?'Windows 专用':'未准备'})),
    ...(data.builtin_tools||[]).map(t=>({...t,kind:'system',category:nativeCategories[t.id]||'系统工具',status:t.ready?'系统自带':'此系统不可用',portable_note:`由${t.source||'当前电脑的系统'}提供，不占 U 盘工具包空间。`})),
    ...(data.library_tools||[]).map(t=>({...t,kind:'download',status:foreign(t)?'Windows 专用':'待下载'})),
    ...(data.resources||[]).map(t=>({...t,kind:'download',ready:false,status:'需另行准备',category:t.id==='network-driver'?'网络与连接':'启动与救援',reason:t.status,portable_note:t.description}))
  ];
  const intro=el('p',`${rows.filter(t=>t.kind==='portable'&&t.ready).length} 款已内置 · ${rows.filter(t=>t.kind==='system'&&t.ready).length} 项系统工具 · ${rows.filter(t=>t.kind==='download').length} 项选装资源`,'library-summary');
  const filters=el('div',undefined,'library-filters'),search=el('input'),category=el('select'),state=el('select');
  search.type='search';search.placeholder='搜索名称或用途，如：磁盘、压缩、启动项';search.setAttribute('aria-label','搜索工具');
  category.setAttribute('aria-label','筛选工具分类');state.setAttribute('aria-label','筛选准备状态');
  for(const name of ['全部分类',...new Set(rows.map(t=>t.category))]){const o=el('option',name);o.value=name;category.append(o);}
  for(const [value,name] of [['all','全部工具'],['ready','可直接打开'],['portable','本盘已内置'],['download','待下载 / 选装']]){const o=el('option',name);o.value=value;state.append(o);}
  filters.append(search,category,state);
  const count=el('p',undefined,'library-count'),list=el('div',undefined,'tool-list');count.setAttribute('role','status');list.setAttribute('aria-label','工具列表');
  root.append(intro,filters,count,list);
  function draw(){
    list.replaceChildren();const query=search.value.trim().toLocaleLowerCase();
    const shown=rows.filter(t=>(category.value==='全部分类'||t.category===category.value)&&
      (state.value==='all'||state.value==='ready'&&t.ready||state.value==='portable'&&t.kind==='portable'&&t.ready||state.value==='download'&&!t.ready)&&
      (!query||[t.name,t.description,t.category,t.portable_note].join(' ').toLocaleLowerCase().includes(query)));
    count.textContent=`显示 ${shown.length} / ${rows.length} 项`;
    for(const t of shown){
      const row=el('article',undefined,'tool-row');row.dataset.toolId=t.id;
      const info=el('div',undefined,'tool-info'),title=el('div',undefined,'tool-title');
      title.append(el('h3',t.name),el('span',t.category,'tool-category'));info.append(title,el('p',t.description||'查看官方说明'));
      const details=el('details',undefined,'tool-info-details');details.append(el('summary','便携方式与来源'));
      if(t.portable_note)details.append(el('p',t.portable_note));
      if(t.reason)details.append(el('p',t.reason));
      const metadata=[t.version,t.platform,t.bytes?bytes(t.bytes):'',t.license].filter(Boolean).join(' · ');if(metadata)details.append(el('p',metadata));
      if(t.official_url){const link=el('a','官方说明 ↗');link.href=t.official_url;link.target='_blank';link.rel='noreferrer';details.append(link);}
      info.append(details);
      const actions=el('div',undefined,'tool-row-actions');actions.append(el('span',t.status,t.ready?'tool-status ready':'tool-status'));
      if(t.ready){const b=button('打开 ↗','tools.launch',async()=>{b.disabled=true;try{await openTool(t.id);}finally{b.disabled=false;}},'secondary');actions.append(b);}
      else if(t.official_url){const a=el('a','前往下载 ↗','tool-download');a.href=t.official_url;a.target='_blank';a.rel='noreferrer';actions.append(a);}
      row.append(info,actions);list.append(row);
    }
    if(!shown.length)list.append(el('p','没有匹配的工具，试试其他名称或分类。','tool-empty'));
  }
  search.addEventListener('input',draw);category.addEventListener('change',draw);state.addEventListener('change',draw);draw();
}
