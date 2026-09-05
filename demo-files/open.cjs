const fs = require('node:fs');
const path = require('node:path');
const root = process.argv[2];
const {chromium} = require(path.join(root, 'browser/node_modules/playwright'));
(async () => {
  if (!process.env.DISPLAY && !process.env.WAYLAND_DISPLAY) throw new Error('A Linux graphical session is required for demo open; CI uses the headless acceptance runner.');
  const browser = await chromium.launchPersistentContext(path.join(root, 'browser/profile'), {
    headless:false, executablePath:process.env.CHROME_EXECUTABLE || undefined,
    ignoreHTTPSErrors:true,
    args:['--host-resolver-rules=MAP *.cells.test 127.0.0.1,MAP dex.dsh-system.svc 127.0.0.1'],
  });
  const pidFile=path.join(root,'runtime/browser.pid');
  fs.writeFileSync(pidFile,String(process.pid),{mode:0o600});
  process.once('SIGTERM',()=>{void browser.close();});
  process.once('SIGINT',()=>{void browser.close();});
  try {
    const closed=new Promise(resolve=>browser.on('close',resolve));
    const page = browser.pages()[0] || await browser.newPage();
    await page.goto(`https://${fs.readFileSync(path.join(root,'runtime/hostname'),'utf8').trim()}:18443`);
    await closed;
  } finally {
    await browser.close();
    fs.rmSync(pidFile,{force:true});
  }
})().catch(error => { console.error(error.message); process.exit(1); });
