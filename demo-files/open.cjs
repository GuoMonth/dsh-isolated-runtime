const fs = require('node:fs');
const path = require('node:path');
const {execFileSync} = require('node:child_process');
async function openDemo(root) {
  const {chromium} = require(path.join(root, 'browser/node_modules/playwright'));
  if (process.platform === 'linux' && !process.env.DISPLAY && !process.env.WAYLAND_DISPLAY) {
    throw new Error('A graphical session is required for demo open; CI uses the headless acceptance runner.');
  }
  const browser = await chromium.launchPersistentContext(path.join(root, 'browser/profile'), {
    headless:false, executablePath:process.env.CHROME_EXECUTABLE || undefined,
    ignoreHTTPSErrors:true,
    args:['--host-resolver-rules=MAP *.cells.test 127.0.0.1,MAP dex.dsh-system.svc 127.0.0.1'],
  });
  const pidFile=path.join(root,'runtime/browser.pid');
  fs.writeFileSync(`${pidFile}.started`,execFileSync('ps',['-p',String(process.pid),'-o','lstart=']),{mode:0o600});
  fs.writeFileSync(pidFile,String(process.pid),{mode:0o600});
  const stop=()=>{void browser.close();};
  process.once('SIGTERM',stop);
  process.once('SIGINT',stop);
  const closed=new Promise(resolve=>browser.once('close',()=>{
    process.removeListener('SIGTERM',stop);
    process.removeListener('SIGINT',stop);
    if(fs.existsSync(pidFile) && fs.readFileSync(pidFile,'utf8')===String(process.pid)) {
      fs.rmSync(pidFile,{force:true});fs.rmSync(`${pidFile}.started`,{force:true});
    }
    resolve();
  }));
  try {
    const page = browser.pages()[0] || await browser.newPage();
    await page.goto(`https://${fs.readFileSync(path.join(root,'runtime/hostname'),'utf8').trim()}:18443`);
    return {browser,page,closed};
  } catch(error) {await browser.close();throw error;}
}
module.exports={openDemo};
if(require.main===module) {
  openDemo(process.argv[2]).then(({closed})=>closed).catch(error=>{console.error(error.message);process.exit(1);});
}
