// Real Chromium checks for platform-specific labels and their existing actions.
// Set PLAYWRIGHT_MODULE to an installed Playwright module when necessary.
const fs=require('node:fs'),path=require('node:path'),http=require('node:http'),assert=require('node:assert/strict');
const {chromium}=require(process.env.PLAYWRIGHT_MODULE||'playwright');
const root=path.resolve(__dirname,'..'),dictionary=JSON.parse(fs.readFileSync(path.join(root,'web/locales.json'),'utf8'));
const version=fs.readFileSync(path.join(root,'main.go'),'utf8').match(/const version = "([^"]+)"/)[1];
const results=[];
async function check(platform,arch){
  let language='auto';const calls=[],errors=[];
  const macTools=[['mac-activity-monitor','macOS 活动监视器'],['mac-disk-utility','macOS 磁盘工具'],['mac-console','macOS 控制台'],['mac-system-information','macOS 系统信息'],['mac-wireless-diagnostics','macOS 无线诊断']];
  const server=http.createServer(async(req,res)=>{
    if(req.url==='/api/run'){
      let body='';for await(const part of req)body+=part;
      const {action,input}=JSON.parse(body);calls.push({action,input});
      const fixtures={'settings.ui.get':{language,revision:1},'settings.model.get':{source:'cloud',revision:'fixture',has_api_key:false},'chat.list':[],
        'pico.status':{running:false},'runtime.cache.status':{status:'not_used'},
        'drivers.inspect':platform==='darwin'?{platform,interfaces:{interfaces:[{name:'en0',up:true,addresses:['192.0.2.1/24']}]},
          note:'此检查提供 macOS 网络接口与地址；不读取 Windows 硬件 ID、INF 或设备故障码，也不会安装驱动。',next_steps:[]}
          :{platform,devices:[{name:'Fixture adapter',error_code:0,driver_version:'fixture',inf_name:'fixture.inf'}],offline_files:[],note:'Fixture'},
        'tools.catalog':{portable_tools:[],library_tools:[],resources:[],builtin_tools:(platform==='darwin'?macTools:[['task-manager','任务管理器']]).map(([id,name])=>({id,name,ready:true,platform}))}};
      const data=action==='settings.ui.save'?{language:language=input.language,revision:2}:fixtures[action];
      res.setHeader('Content-Type','application/json');res.end(JSON.stringify(data===undefined?{ok:false,error:'Unexpected action '+action}:{ok:true,data}));return;
    }
    const name=req.url==='/'?'index.html':req.url.slice(1);
    if(!/^(?:index\.html|style\.css|[a-z0-9]+\.js|locales\.json)$/.test(name)){res.statusCode=404;res.end();return;}
    res.setHeader('Content-Type',name.endsWith('.js')?'application/javascript':name.endsWith('.json')?'application/json':name.endsWith('.css')?'text/css':'text/html');
    res.end(fs.readFileSync(path.join(root,'web',name),'utf8').replaceAll('__XIAPAN_TOKEN__','fixture').replaceAll('__XIAPAN_PLATFORM__',platform).replaceAll('__XIAPAN_ARCH__',arch).replaceAll('__XIAPAN_VERSION__',version));
  });
  await new Promise(resolve=>server.listen(0,'127.0.0.1',resolve));const base=`http://127.0.0.1:${server.address().port}`;
  const browser=await chromium.launch({headless:true});
  try{
    const page=await browser.newPage({locale:'en-US',viewport:{width:1366,height:900}});page.on('pageerror',e=>errors.push(e.message));
    await page.route('**/*',route=>route.request().url().startsWith(base)?route.continue():route.abort());
    await page.goto(base);await page.waitForFunction(()=>document.documentElement.lang==='en');
    assert.equal(await page.locator('#app-version').innerText(),version.replace('-portable-preview','')+' portable preview');
    assert.equal(calls.filter(c=>c.action.startsWith('wallet.')||c.action.endsWith('.inspect')||c.action==='chat.start'||c.action==='pico.ui.open').length,0);
    await page.locator('#starter-system').click();
    assert.equal(await page.locator('#question').inputValue(),dictionary[platform==='darwin'?'查看这台 Mac 的系统信息和内存情况':'看看这台电脑的启动项，哪些可以考虑精简？']);
    assert.equal(await page.locator('#new-chat kbd').innerText(),platform==='darwin'?'⌘ ⇧ N':'Ctrl ⇧ N');
    await page.locator('.nav-item[data-view=tools]').click();await page.waitForSelector('.tool-row');
    await page.locator('.maintenance-shortcuts summary').first().click();
    assert.equal(await page.locator('[data-platform=windows]').isVisible(),platform==='windows');
    await page.locator('#palette-open').click();
    const palette=await page.locator('#palette-commands').innerText();assert.equal(palette.includes('/启动项'),platform==='windows');
    await page.locator('#palette-close').click();
    await page.locator('.nav-item[data-view=rescue]').click();
    const rescue=await page.locator('#rescue-view').innerText();
    assert.ok(!/[\u3400-\u9fff]/.test(rescue),'recovery view fully translated');
    if(platform==='darwin'){
      assert.ok(!rescue.includes('Ventoy')&&!rescue.includes('Windows PE'));
      assert.ok(rescue.includes(arch==='arm64'?'power button':'Command-R'));
      assert.equal(await page.locator('#rescue-view a').first().getAttribute('href'),'https://support.apple.com/zh-cn/102518');
    }else assert.ok(rescue.includes('Ventoy'));
    await page.locator('.nav-item[data-view=chat]').click();
    await page.locator('#offline-open').click();
    await page.locator('#dialog [data-action-id="drivers.inspect"]').click();
    await page.waitForSelector('#dialog table');const network=await page.locator('#dialog-content').innerText();
    if(platform==='darwin'){
      assert.ok(network.includes('en0')&&network.includes('192.0.2.1/24'));assert.ok(!network.includes('Driver / INF'));
      assert.equal(await page.locator('#dialog [data-action-id="tools.launch"]').innerText(),'Open Wireless Diagnostics');
    }else assert.ok(network.includes('fixture.inf'));
    assert.ok(!/[\u3400-\u9fff]/.test(network),'network results fully translated');
    await page.evaluate(async()=>{const select=document.querySelector('#language-select');select.value='zh-CN';await select.onchange();});
    await page.waitForFunction(()=>document.documentElement.lang==='zh-CN');
    assert.equal(await page.locator('#app-version').innerText(),version.replace('-portable-preview','')+' 便携预览');
    if(platform==='darwin')assert.ok((await page.locator('#dialog-content').innerText()).includes('网络接口'));
    await page.locator('#dialog-close').click();
    await page.setViewportSize({width:390,height:844});assert.ok(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth));
    assert.deepEqual(errors,[]);results.push({platform,arch,ok:true,checks:['lightweight startup','version','native shortcut','starter prompt','available maintenance actions','quick commands','recovery instructions','network action binding','language switching','narrow layout','no script errors']});
  }finally{await browser.close();await new Promise(resolve=>server.close(resolve));}
}
(async()=>{
  for(const [platform,arch] of [['windows','amd64'],['darwin','arm64'],['darwin','amd64']])await check(platform,arch);
  const report={ok:true,version,scope:'Real Chromium with local fixture actions; Mac backend and release acceptance recorded separately.',paid_model_calls:0,results};
  if(process.env.UI_EVIDENCE_OUTPUT)fs.writeFileSync(process.env.UI_EVIDENCE_OUTPUT,JSON.stringify(report,null,2)+'\n');
  console.log(JSON.stringify(report,null,2));
})().catch(error=>{console.error(error);process.exitCode=1;});
