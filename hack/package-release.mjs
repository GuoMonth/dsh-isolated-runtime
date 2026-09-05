#!/usr/bin/env node
import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';
import crypto from 'node:crypto';
import {execFileSync as run} from 'node:child_process';
const root = path.resolve(import.meta.dirname, '..');
const [version, cell, operator, output = 'dist'] = process.argv.slice(2);
if (!/^v\d+\.\d+\.\d+(?:-[a-z0-9.]+)?$/.test(version || '') ||
    ![cell,operator].every(ref => /^ghcr\.io\/guomonth\/dsh-isolated-runtime-(cell|operator)@sha256:[a-f0-9]{64}$/.test(ref || ''))) {
  throw new Error('usage: package-release.mjs VERSION CELL_DIGEST_REF OPERATOR_DIGEST_REF [OUTPUT]');
}
const sourceSHA = run('git',['rev-parse','HEAD'],{cwd:root,encoding:'utf8'}).trim();
if (run('git',['status','--porcelain'],{cwd:root,encoding:'utf8'}).trim()) throw new Error('Commit all release inputs before packaging');
const temp = fs.mkdtempSync(path.join(os.tmpdir(),'dsh-release-'));
const name = `dsh-isolated-runtime-${version}-linux-amd64`;
const stage = path.join(temp,name);
const out = path.resolve(output);
fs.mkdirSync(stage); fs.mkdirSync(out,{recursive:true});
try {
  // Only tracked public inputs, never workstation credentials or caches.
  const files = run('git',['ls-files','-z'],{cwd:root}).toString().split('\0').filter(Boolean);
  for (const file of files) {
    if (!/^(demo$|demo-files\/|hack\/lib\/|config\/|test\/e2e\/|compat\/dsh\/(?:baseline.json$|patches\/)|docs\/|README|LICENSE)/.test(file)) continue;
    const to = path.join(stage,file);fs.mkdirSync(path.dirname(to),{recursive:true});fs.copyFileSync(path.join(root,file),to);fs.chmodSync(to,fs.statSync(path.join(root,file)).mode);
    if (/\.ya?ml$/.test(file)) {
      let text = fs.readFileSync(to,'utf8');
      if (file.endsWith('kustomization.yaml')) text = text.replace('newTag: main',`digest: ${operator.split('@')[1]}`);
      text = text.replaceAll('controller:latest',operator).replace(/ghcr\.io\/(?:example\/dsh-cell|guomonth\/dsh-isolated-runtime-cell)@sha256:0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef/g,cell);
      fs.writeFileSync(to,text);
    }
  }
  const manifest = {schemaVersion:1,version,sourceSHA,platform:'linux/amd64',
    baseline:JSON.parse(fs.readFileSync(path.join(root,'compat/dsh/baseline.json'))),
    images:{cell,operator},candidateRun:process.env.GITHUB_RUN_ID || null,
    candidateURL:process.env.GITHUB_RUN_ID ? `https://github.com/GuoMonth/dsh-isolated-runtime/actions/runs/${process.env.GITHUB_RUN_ID}` : null};
  fs.writeFileSync(path.join(stage,'release.json'),JSON.stringify(manifest,null,2)+'\n');
  const epoch = run('git',['show','-s','--format=%ct','HEAD'],{cwd:root,encoding:'utf8'}).trim();
  const archive = path.join(out,`${name}.tar.gz`);
  run('tar',['--sort=name',`--mtime=@${epoch}`,'--owner=0','--group=0','--numeric-owner','-czf',archive,'-C',temp,name]);
  const sum = crypto.createHash('sha256').update(fs.readFileSync(archive)).digest('hex');
  fs.writeFileSync(path.join(out,'SHA256SUMS'),`${sum}  ${path.basename(archive)}\n`);
  fs.copyFileSync(path.join(stage,'release.json'),path.join(out,'release.json'));
  console.log(archive);
} finally {fs.rmSync(temp,{recursive:true,force:true});}
