const assert=require('node:assert/strict');
const fs=require('node:fs');
const path=require('node:path');
const https=require('node:https');
const {openDemo}=require('../../../demo-files/open.cjs');
const [root,screenshot]=process.argv.slice(2);
(async()=>{
  assert.equal(process.platform,'darwin');assert.equal(process.arch,'arm64');
  const runtime=path.join(root,'runtime');
  fs.writeFileSync(path.join(runtime,'hostname'),'macos.cells.test');
  const server=https.createServer({key:fs.readFileSync(path.join(runtime,'gateway.key')),cert:fs.readFileSync(path.join(runtime,'gateway.crt'))},(_req,res)=>{
    res.writeHead(200,{'content-type':'text/html'});
    res.end('<!doctype html><title>DSH Apple Silicon host proof</title><h1>Apple Silicon browser connected</h1><p>Native Chromium, isolated profile, local DNS and test TLS.</p>');
  });
  await new Promise(resolve=>server.listen(18443,'127.0.0.1',resolve));
  let context;
  try {
    context=await openDemo(root);
    assert.equal(await context.page.title(),'DSH Apple Silicon host proof');
    assert.equal(new URL(context.page.url()).hostname,'macos.cells.test');
    await context.page.evaluate(()=>localStorage.setItem('host-proof','retained'));
    await context.page.screenshot({path:screenshot});
    assert.equal(fs.readFileSync(path.join(runtime,'browser.pid'),'utf8'),String(process.pid));
    await context.page.close();
    await Promise.race([context.closed,new Promise((_,reject)=>setTimeout(()=>reject(new Error('Last macOS window did not release the demo profile')),15000).unref())]);
    assert.equal(fs.existsSync(path.join(runtime,'browser.pid')),false);
    context=await openDemo(root);
    assert.equal(await context.page.evaluate(()=>localStorage.getItem('host-proof')),'retained');
    await context.page.close();await context.closed;context=undefined;
    assert.equal(fs.existsSync(path.join(runtime,'browser.pid.started')),false);
    console.log('Native Apple Silicon Chromium DNS, TLS and retained private-profile lifecycle passed');
  } finally {
    if(context) await context.browser.close();
    await new Promise(resolve=>server.close(resolve));
  }
})().catch(error=>{console.error(error);process.exit(1);});
