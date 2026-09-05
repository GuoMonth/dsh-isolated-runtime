// Exercise the actual archive verifier against accepted and tampered artifacts.
import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';
import assert from 'node:assert/strict';
import {execFileSync,spawnSync} from 'node:child_process';
const root=path.resolve(import.meta.dirname,'..');
const temp=fs.mkdtempSync(path.join(os.tmpdir(),'dsh-release-contract-'));
try {
  const ref=type=>`ghcr.io/guomonth/dsh-isolated-runtime-${type}@sha256:${'a'.repeat(64)}`;
  execFileSync(process.execPath,[path.join(root,'hack/package-release.mjs'),'v0.1.0',ref('cell'),ref('operator'),temp]);
  const verify=(live)=>spawnSync(process.execPath,[path.join(root,'hack/check-release.mjs'),temp,...live?[live]:[]],{encoding:'utf8'});
  const result=verify();assert.equal(result.status,0,result.stderr);
  const accepted=JSON.parse(result.stdout);
  const live=path.join(temp,'live.json');
  const proof={...accepted,kind:'live-model',model:'deepseek-v4-flash',success:true};
  fs.writeFileSync(live,JSON.stringify(proof));assert.equal(verify(live).status,0);
  for(const change of [{success:false},{sourceSHA:'b'.repeat(40)},{archiveSHA256:'b'.repeat(64)},{images:{cell:ref('operator'),operator:ref('cell')}},{kind:'deterministic'},{model:''}]) {
    fs.writeFileSync(live,JSON.stringify({...proof,...change}));assert.notEqual(verify(live).status,0,'forged live evidence was accepted');
  }
  const manifestFile=path.join(temp,'release.json');const manifest=fs.readFileSync(manifestFile);
  fs.writeFileSync(manifestFile,JSON.stringify({...JSON.parse(manifest),sourceSHA:'b'.repeat(40)}));
  assert.notEqual(verify().status,0,'outer manifest mutation was accepted');fs.writeFileSync(manifestFile,manifest);
  const archive=path.join(temp,fs.readdirSync(temp).find(name=>name.endsWith('.tar.gz')));
  fs.appendFileSync(archive,'tamper');assert.notEqual(verify().status,0,'archive mutation was accepted');
  const tools=path.join(temp,'tools');fs.mkdirSync(tools);
  for(const name of ['bash','dirname','mkdir','flock','sha256sum','cut']) {
    const binary=execFileSync('which',[name],{encoding:'utf8'}).trim();fs.symlinkSync(binary,path.join(tools,name));
  }
  const missing=spawnSync(path.join(root,'demo'),['up'],{env:{...process.env,PATH:tools,DSH_DEMO_HOME:path.join(temp,'missing-docker')},encoding:'utf8'});
  assert.notEqual(missing.status,0);assert.match(missing.stderr,/Missing prerequisite: docker/);
  console.log('Release identity, archive integrity and live-evidence guards passed');
} finally {fs.rmSync(temp,{recursive:true,force:true});}
