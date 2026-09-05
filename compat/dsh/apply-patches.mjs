import fs from 'node:fs';
import path from 'node:path';
import crypto from 'node:crypto';
import {execFileSync} from 'node:child_process';
const [source] = process.argv.slice(2);
const root=import.meta.dirname;
const baseline=JSON.parse(fs.readFileSync(path.join(root,'baseline.json')));
for(const patch of baseline.distribution.patches) {
  const file=path.join(root,patch.file);
  const digest=crypto.createHash('sha256').update(fs.readFileSync(file)).digest('hex');
  if(digest!==patch.sha256)throw Error('Integration patch integrity mismatch');
  execFileSync('git',['apply','--check',file],{cwd:source,stdio:'inherit'});
  execFileSync('git',['apply',file],{cwd:source,stdio:'inherit'});
}
