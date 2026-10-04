/* System tools are registered once in the action core. */
function renderToolLibrary(data){
 const root=$('tool-collection');root.replaceChildren(el('p',data.note));
 const list=el('div',undefined,'tools-grid');
 for(const tool of data.builtin_tools||[]){
  const card=el('article',undefined,'tool-tile');
  card.append(el('h3',tool.name),el('p',tool.description));
  const open=button(tool.ready?'打开工具':'此电脑不可用','tools.launch',()=>openTool(tool.id),'secondary');
  open.disabled=!tool.ready;card.append(open);list.append(card);
 }
 root.append(list);
}
