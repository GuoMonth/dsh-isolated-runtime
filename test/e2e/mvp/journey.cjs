const fs=require('node:fs');
const path=require('node:path');
const crypto=require('node:crypto');
const root=process.env.DSH_DEMO_HOME;
const {chromium}=require(path.join(root,'browser/node_modules/playwright'));
const mode=process.env.MVP_MODE||'deterministic';
const stateFile=path.join(root,'runtime/journey.json');
const resume=process.env.MVP_RESUME==='1';
const restored=process.env.MVP_RESTORED==='1';
const previous=(resume||restored)?JSON.parse(fs.readFileSync(stateFile)):undefined;
const turnMarker=`MVP_${crypto.randomBytes(6).toString('hex')}`;
const marker=previous?.marker||turnMarker;
const model=process.env.MVP_MODEL||(mode==='deterministic'?'deepseek-v4-flash-vision-exp':'deepseek-v4-flash');
const cookieFile=path.join(root,'runtime/journey-cookies.json');
(async()=>{
  const browser=await chromium.launch({headless:true,executablePath:process.env.CHROME_EXECUTABLE||undefined,args:['--host-resolver-rules=MAP *.cells.test 127.0.0.1,MAP dex.dsh-system.svc 127.0.0.1']});
  let page;
  try {
    const context=await browser.newContext({ignoreHTTPSErrors:true,locale:'en-US',storageState:resume?cookieFile:undefined});
    page=await context.newPage();const errors=[];page.on('pageerror',error=>errors.push(error.message));
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
    const continuation=welcome.getByRole('button',{name:/^Continue$|^继续$/i});
    const keyInput=page.getByLabel(/^API key$|^API 密钥$/i,{exact:true});
    if(!resume) {
      await continuation.or(keyInput).first().waitFor({timeout:30000});
      if(await continuation.isVisible())await continuation.click();
    }
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
      if(!await page.getByText(marker,{exact:false}).first().isVisible()) {
        await page.getByRole('button',{name:'Search sessions',exact:true}).click();
        await page.getByPlaceholder('Search sessions...').fill(previous.sessionTitle||'MVP file verification');
        await page.getByRole('tree',{name:'Search results'}).getByRole('treeitem').first().click({timeout:30000});
      }
      await page.getByText(marker,{exact:false}).first().waitFor({timeout:30000});
    }
    if(!resume&&!restored && !await page.getByRole('button',{name:/^Select model/}).isVisible()) {
      await page.getByRole('button',{name:'Choose workspace',exact:true}).click();
      await page.getByRole('button',{name:'Edit path',exact:true}).click();
      await page.getByRole('textbox',{name:'Edit path',exact:true}).fill('/var/lib/dsh/data/workspace');
      await page.getByRole('textbox',{name:'Edit path',exact:true}).press('Enter');
      await page.getByRole('button',{name:'Open',exact:true}).click();
    }
    await page.getByRole('button',{name:/^Select model/}).click();
    await page.getByRole('menuitem',{name:/^Model/}).click();
    await page.getByRole('menuitemradio',{name:new RegExp(`^${model}$`,'i')}).click();
    const input=page.locator('[data-composer-input]').first();
    if(!resume&&!restored) {
      const uploads=[
        {name:`${marker}-upload.txt`,mimeType:'text/plain',buffer:Buffer.from(`Uploaded ${marker}`)},
      ];
      if(mode==='deterministic')uploads.push({name:`${marker}.png`,mimeType:'image/png',buffer:Buffer.from('iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNk+A8AAQUBAScY42YAAAAASUVORK5CYII=','base64')});
      await page.locator('input[type="file"]').setInputFiles(uploads);
    }
    const prompt=`${turnMarker}: Use the write tool to create ${turnMarker}.txt containing exactly ${turnMarker}, then use the read tool to read it. Also read any uploaded text file. End your response with the word Verified followed by the marker. Do not merely describe the steps.`;
    await input.fill(prompt);
    const writes=await page.locator('[data-tool="write"][data-state="ok"]').count();
    await page.getByRole('button',{name:'Send message',exact:true}).click();
    await page.locator('[data-tool="write"][data-state="ok"]').nth(writes).waitFor({timeout:180000});
    await page.locator('[data-tool="read"][data-state="ok"]').last().waitFor({timeout:180000});
    await page.getByText(`Verified ${turnMarker}`,{exact:false}).last().waitFor({timeout:180000});
    if(mode==='deterministic') {
      // The fixture emits separate SSE chunks; the UI must render before EOF.
      await page.getByRole('button',{name:'Stop generating',exact:true}).waitFor({timeout:5000});
    }
    // A completed answer must leave the composer ready for another turn.
    await page.getByRole('button',{name:/stop generating/i}).waitFor({state:'hidden',timeout:30000});
    await context.storageState({path:cookieFile});
    fs.chmodSync(cookieFile,0o600);
    fs.writeFileSync(stateFile,JSON.stringify({marker,turnMarker,mode,model,sessionTitle:await page.locator('nav[aria-label="Session hierarchy"] button:disabled').innerText()}),{mode:0o600});
    if(errors.length)throw Error(`Browser errors: ${errors.join('; ')}`);
    await page.screenshot({path:path.join(root,'runtime/journey.png')});
    console.log(`MVP ${mode} browser journey passed (resume=${resume})`);
  } catch(error) {
    if(page)await page.screenshot({path:path.join(root,'runtime/journey-failed.png')}).catch(()=>{});
    throw error;
  } finally {await browser.close();}
})().catch(error=>{console.error(error.message);process.exit(1);});
