const fs=require('node:fs');
const path=require('node:path');
const crypto=require('node:crypto');
const root=process.env.DSH_DEMO_HOME;
const {chromium}=require(path.join(root,'browser/node_modules/playwright'));
const mode=process.env.MVP_MODE||'deterministic';
const stateFile=path.join(root,'runtime/journey.json');
const resume=process.env.MVP_RESUME==='1';
const restored=process.env.MVP_RESTORED==='1';
const marker=(resume||restored)?JSON.parse(fs.readFileSync(stateFile)).marker:`MVP_${crypto.randomBytes(6).toString('hex')}`;
const cookieFile=path.join(root,'runtime/journey-cookies.json');
(async()=>{
  const browser=await chromium.launch({headless:true,executablePath:process.env.CHROME_EXECUTABLE||undefined,args:['--host-resolver-rules=MAP *.cells.test 127.0.0.1,MAP dex.dsh-system.svc 127.0.0.1']});
  try {
    const context=await browser.newContext({ignoreHTTPSErrors:true,locale:'en-US',storageState:resume?cookieFile:undefined});
    const page=await context.newPage();const errors=[];page.on('pageerror',error=>errors.push(error.message));
    const host=fs.readFileSync(path.join(root,'runtime/hostname'),'utf8').trim();
    await page.goto(`https://${host}:18443`,{waitUntil:'domcontentloaded',timeout:90000});
    if(page.url().includes('/dex/')) {
      await page.locator('input[name="login"]').fill('alice@example.com');
      await page.locator('input[name="password"]').fill('password');
      await page.getByRole('button',{name:/login/i}).click();
      await page.waitForURL(url=>url.hostname===host,{timeout:90000});
    }
    // Reuse the shipped first-run flow, including the write-only provider key.
    const welcome=page.getByRole('dialog');
    const continuation=welcome.getByRole('button',{name:/continue|继续/i});
    if(await continuation.first().isVisible().catch(()=>false))await continuation.first().click();
    const keyInput=page.getByLabel(/API key|API 密钥/i,{exact:true});
    if(!resume) {
      await keyInput.waitFor({timeout:30000});
      let key='mvp-fixture-key';
      if(mode==='live-model') {
        const file=process.env.DEEPSEEK_KEY_FILE;
        if(!file || (fs.statSync(file).mode&0o077)!==0)throw Error('live smoke requires a private DEEPSEEK_KEY_FILE');
        key=fs.readFileSync(file,'utf8').trim();if(!key)throw Error('empty key');
      }
      await keyInput.fill(key);
      await page.getByRole('button',{name:/save.*continue|保存并继续/i}).click();
    }
    await page.locator('[data-composer-input]').first().waitFor({timeout:30000});
    if(resume||restored) {
      await page.getByText(marker,{exact:false}).first().waitFor({timeout:30000});
    }
    const input=page.locator('[data-composer-input]').first();
    if(!resume) {
      await page.locator('input[type="file"]').setInputFiles([
        {name:`${marker}-upload.txt`,mimeType:'text/plain',buffer:Buffer.from(`Uploaded ${marker}`)},
        {name:`${marker}.png`,mimeType:'image/png',buffer:Buffer.from('iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNk+A8AAQUBAScY42YAAAAASUVORK5CYII=','base64')},
      ]);
    }
    const prompt=`${marker}: Use the write tool to create ${marker}.txt containing exactly ${marker}, then use the read tool to read it. Also read any uploaded text file. End your response with the word Verified followed by the marker. Do not merely describe the steps.`;
    await input.fill(prompt);
    await input.press('Enter');
    await page.locator('[data-tool="write"][data-state="ok"]').last().waitFor({timeout:180000});
    await page.locator('[data-tool="read"][data-state="ok"]').last().waitFor({timeout:180000});
    await page.getByText(`Verified ${marker}`,{exact:false}).last().waitFor({timeout:180000});
    // A completed answer must leave the composer ready for another turn.
    await page.getByRole('button',{name:/stop response|stop generation/i}).waitFor({state:'hidden',timeout:30000});
    await context.storageState({path:cookieFile});
    fs.chmodSync(cookieFile,0o600);
    fs.writeFileSync(stateFile,JSON.stringify({marker,mode,model:process.env.MVP_MODEL||'deepseek-v4-flash'}),{mode:0o600});
    if(errors.length)throw Error(`Browser errors: ${errors.join('; ')}`);
    await page.screenshot({path:path.join(root,'runtime/journey.png')});
    console.log(`MVP ${mode} browser journey passed (resume=${resume})`);
  } finally {await browser.close();}
})().catch(error=>{console.error(error.message);process.exit(1);});
