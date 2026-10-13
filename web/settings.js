'use strict';
let modelSettings=null;
async function loadSettings(){modelSettings=await run('settings.model.get');return modelSettings;}

async function modelSettingsView(){
 const current=await loadSettings();
 const c=dialog('模型设置','沿用随盘配置：换台电脑，继续使用自己的模型和对话。');
 const form=el('form',undefined,'model-settings-form');form.onsubmit=e=>e.preventDefault();
 const field=(title,input)=>{const label=el('label',title);label.append(input);form.append(label);return input;};
 const source=el('select');source.id='model-source';attr(source,'aria-label','模型来源');source.append(Object.assign(el('option','虾盘云'),{value:'cloud'}),Object.assign(el('option','自带 API · OpenAI 兼容'),{value:'custom'}));source.value=current.source;field('模型来源',source);
 const base=el('input');base.type='text';base.id='model-base';base.value=current.base_url;base.placeholder='https://api.example.com/v1';base.autocomplete='off';field('API 基础地址',base);
 const key=el('input');key.type='password';key.id='model-key';key.autocomplete='new-password';attr(key,'placeholder',current.has_api_key?'已保存；留空继续使用（换地址需重新填写）':'填写 API Key；本机无密钥服务可留空');field('API Key',key);
 const clear=el('input');clear.type='checkbox';clear.id='model-clear-key';const clearLabel=el('label','移除已保存的 API Key');clearLabel.prepend(clear);form.append(clearLabel);
 const model=el('input');model.type='text';model.id='model-id';model.value=current.model;attr(model,'placeholder','服务商提供的模型 ID');field('模型 ID',model);
 const note=el('p','密钥、历史版本备份和对话保存在本盘。测试会向所选服务发送一次请求；保存不会发送对话。');form.append(note);
 const status=el('p',undefined,'settings-status');status.setAttribute('role','status');
 const payload=()=>({expected_revision:current.revision,source:source.value,base_url:base.value,model:model.value,api_key:key.value,clear_api_key:clear.checked});
 const controls=el('div',undefined,'button-row');
 const test=button('测试连接','settings.model.test',async()=>{setFormBusy(true);ui(status,'正在测试模型与工具调用…');try{const d=await run('settings.model.test',payload());ui(status,d.note);}catch(e){ui(status,e.message);}finally{setFormBusy(false);}},'secondary');test.id='model-test';
 const save=button('保存并使用','settings.model.save',async()=>{setFormBusy(true);try{modelSettings=await run('settings.model.save',payload());key.value='';$('engine-select').value='pico';engineNote();$('dialog').close();toast('模型已保存到本盘');}catch(e){ui(status,e.message);}finally{setFormBusy(false);}});save.id='model-save';
 const cloudWallet=button('打开虾盘云钱包','wallet.ensure',()=>walletView(),'text-button');
 const setFormBusy=value=>{for(const input of form.querySelectorAll('input,select,button'))input.disabled=value;};
 const update=()=>{const custom=source.value==='custom';for(const node of [base.parentElement,key.parentElement,clearLabel])node.classList.toggle('hidden',!custom);cloudWallet.classList.toggle('hidden',custom);};source.onchange=update;
 controls.append(test,save);form.append(controls,status,cloudWallet);c.append(form);update();
}
