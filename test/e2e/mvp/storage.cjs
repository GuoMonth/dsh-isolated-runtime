// Runs inside the real Cell. Never prints credentials or file contents.
const fs=require('node:fs');
const path=require('node:path');
const crypto=require('node:crypto');
const assert=require('node:assert/strict');
const [marker,mode,phase]=process.argv.slice(2);
assert.match(marker,/^MVP_[a-f0-9]{12}$/);
const base='/var/lib/dsh/data';
const hashes=new Set();
let sessions=0;
function walk(directory) {
  for(const entry of fs.readdirSync(directory,{withFileTypes:true})) {
    const file=path.join(directory,entry.name);
    if(entry.isDirectory())walk(file);
    else if(entry.isFile()) {
      const data=fs.readFileSync(file);
      assert(!data.includes(Buffer.from('mvp-fixture-key')),'provider key entered data snapshot surface');
      assert(!entry.name.includes('.credentials'),'private credentials file entered data snapshot surface');
      if(file.includes('/attachments/v1/'))hashes.add(crypto.createHash('sha256').update(data).digest('hex'));
      if(file.includes('/sessions/')&&data.includes(Buffer.from(marker)))sessions++;
    }
  }
}
walk(base);
const hash=buffer=>crypto.createHash('sha256').update(buffer).digest('hex');
assert(hashes.has(hash(Buffer.from(`Uploaded ${marker}`))),'uploaded file bytes missing');
if(mode==='deterministic')assert(hashes.has(hash(Buffer.from('iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNk+A8AAQUBAScY42YAAAAASUVORK5CYII=','base64'))),'uploaded image bytes missing');
assert(sessions>0,'durable session content missing');
assert.equal(fs.readFileSync(path.join(base,'workspace',`${marker}.txt`),'utf8'),marker);
const privateFile='/var/lib/dsh-private/.credentials.yaml';
assert(fs.existsSync(privateFile),'private state missing');
const privateData=fs.readFileSync(privateFile,'utf8');
if(phase==='fresh-restore')assert(!privateData.includes('DEEPSEEK_API_KEY'),'provider key restored');
else if(mode==='deterministic')assert(privateData.includes('mvp-fixture-key'),'native editor did not persist the provider key privately');
console.log(JSON.stringify({phase,attachments:true,sessions:true,workspace:true,privateState:true}));
