// Real RC browser/HTTP/WS test. The API key is deliberately non-functional.
const { chromium } = require(process.env.PLAYWRIGHT_MODULE || 'playwright');
const fs = require('node:fs');
const assert = require('node:assert/strict');
const url = fs.readFileSync(process.env.RC2_URL_FILE, 'utf8');
const rpc = (page, method, args) => page.evaluate(async ({method,args}) => {
  const response = await fetch('/api/'+method, {method:'POST',headers:{'content-type':'application/json'},body:JSON.stringify({type:'client-request',rpcId:'rc2-'+Date.now(),method,payload:{args}})});
  const body = await response.json();
  if (!response.ok || !body.result?.ok) throw Error(method+': '+JSON.stringify(body.result?.error));
  return body.result.value;
}, {method,args});
(async()=>{
 const browser=await chromium.launch({headless:true,args:['--host-resolver-rules=MAP environment.test 127.0.0.1','--no-proxy-server']});
 try {
  const page=await browser.newPage({ignoreHTTPSErrors:true,locale:'en-US'});
  const errors=[];page.on('pageerror',e=>errors.push(e.message));
  await page.goto(url);
  await page.getByRole('button',{name:'Settings',exact:true}).waitFor();
  await page.waitForTimeout(1000);
  if(await page.getByRole('button',{name:'Continue',exact:true}).count()) await page.getByRole('button',{name:'Continue',exact:true}).click();
  await page.waitForTimeout(1000);
  if(await page.getByRole('button',{name:'Save and continue',exact:true}).count()) {
   await page.getByRole('textbox',{name:'API key',exact:true}).fill('rc2-smoke-not-a-real-key');
   await page.getByRole('button',{name:'Save and continue',exact:true}).click();
  }
  await page.getByRole('button',{name:'Settings',exact:true}).click();
  await page.getByRole('button',{name:'Models',exact:true}).click();
  if(process.env.RC2_EXPECT_UNPATCHED==='1') {
   await page.getByText('Loading the provider directory failed: settings are unavailable in this browser').waitFor();
   console.log(JSON.stringify({unpatchedRemoteSettingsFailure:true,errors}));return;
  }
  await page.getByRole('button',{name:'Edit DeepSeek (deepseek-official)',exact:true}).waitFor();
  const describe=await rpc(page,'settings/describe',{});
  assert.equal(describe.writable,true);
  // Native supported model configuration API, no model network request.
  const model=describe.namespaces.find(n=>n.ns==='llm-deepseek');
  assert.ok(model);
  await rpc(page,'settings/update',{ns:model.ns,patch:{baseURL:'https://api.deepseek.com/anthropic'},expectedRevision:model.revision});
  const credential=await rpc(page,'credentials/describe',{refs:['DEEPSEEK_API_KEY']});
  assert.equal(credential.DEEPSEEK_API_KEY.configured,true);
  const {sessionId}=await rpc(page,'session/create',{request:{}});
  assert.ok(sessionId);
  await page.evaluate(sessionId=>new Promise((resolve,reject)=>{
   const s=new WebSocket('wss://'+location.host+'/api/remote.mux');
   const timer=setTimeout(()=>{s.close();reject(Error('WS timeout'));},15000);
   s.onopen=()=>s.send(JSON.stringify({type:'open',streamId:'rc2',endpoint:'session/follow',payload:{args:{request:{address:{kind:'session',sessionId}}}}}));
   s.onmessage=e=>{const f=JSON.parse(e.data);if(f.streamId!=='rc2')return;if(f.type==='item'&&f.value?.type==='snapshot'){clearTimeout(timer);s.close();resolve();}else if(f.type==='error'){clearTimeout(timer);s.close();reject(Error('WS follow rejected'));}};
   s.onerror=()=>{clearTimeout(timer);reject(Error('WS error'));};
  }),sessionId);
  assert.deepEqual(errors,[]);
  console.log(JSON.stringify({remoteModels:true,httpSettings:true,credentialsConfigured:true,sessionId,websocketSnapshot:true,errors}));
 } finally {await browser.close();}
})().catch(e=>{console.error(e);process.exitCode=1;});
