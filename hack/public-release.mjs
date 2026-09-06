#!/usr/bin/env node
// Preserve both original archives; bind public metadata to their actual proofs.
import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';
import assert from 'node:assert/strict';
import {execFileSync} from 'node:child_process';
const targets=['linux-amd64','darwin-arm64'];
const checker=path.join(import.meta.dirname,'check-release.mjs');
const read=file=>JSON.parse(fs.readFileSync(file));
const write=(file,value)=>fs.writeFileSync(file,JSON.stringify(value,null,2)+'\n');
function prepare(input,output) {
  fs.mkdirSync(output,{recursive:true});
  const packages={};let common;
  for(const target of targets) {
    const directory=path.join(input,target);
    const proofFile=path.join(directory,'evidence/deterministic.json');
    const identity=JSON.parse(execFileSync(process.execPath,[checker,directory,proofFile,'deterministic'],{encoding:'utf8'}));
    const manifest=read(path.join(directory,'release.json'));
    assert.equal(manifest.packagePlatform.replace('/','-'),target);
    const shared={version:manifest.version,sourceSHA:manifest.sourceSHA,images:manifest.images,candidateRun:manifest.candidateRun,candidateURL:manifest.candidateURL};
    if(common) assert.deepEqual(shared,common); else common=shared;
    const archive=`dsh-isolated-runtime-${manifest.version}-${target}.tar.gz`;
    packages[target]={archive,archiveSHA256:identity.archiveSHA256,platform:manifest.platform,manifest:`release-${target}.json`,evidence:`deterministic-${target}.json`};
    fs.copyFileSync(path.join(directory,archive),path.join(output,archive));
    fs.copyFileSync(path.join(directory,'release.json'),path.join(output,packages[target].manifest));
    fs.copyFileSync(proofFile,path.join(output,packages[target].evidence));
  }
  const host=read(path.join(input,'darwin-arm64/evidence/macos-host.json'));
  const mac=read(path.join(input,'darwin-arm64/evidence/deterministic.json'));
  for(const key of ['sourceSHA','archiveSHA256','images','version','packagePlatform','platform']) assert.deepEqual(host[key],mac[key]);
  assert.equal(host.kind,'macos-host');assert.equal(host.success,true);
  assert.equal(host.hostPlatform,'darwin/arm64');
  assert.equal(host.dockerDesktopEndToEnd,'not-run');
  fs.copyFileSync(path.join(input,'darwin-arm64/evidence/macos-host.json'),path.join(output,'macos-host.json'));
  fs.writeFileSync(path.join(output,'SHA256SUMS'),targets.map(target=>`${packages[target].archiveSHA256}  ${packages[target].archive}\n`).join(''));
  write(path.join(output,'release.json'),{schemaVersion:2,...common,packages});
  write(path.join(output,'release-acceptance.json'),{...common,packages,
    automaticAcceptance:{status:'passed',runtimePlatforms:['linux/amd64','linux/arm64'],macosHost:'passed'},
    dockerDesktopEndToEnd:{status:'not-run',timing:'after-publication',owner:'maintainer'},
    liveModel:{status:'not-run',timing:'after-publication',owner:'maintainer'}});
}
function verify(directory) {
  const release=read(path.join(directory,'release.json'));
  assert.equal(release.schemaVersion,2);
  assert.match(release.version,/^v\d+\.\d+\.\d+(?:-[a-z0-9.]+)?$/);
  const temp=fs.mkdtempSync(path.join(os.tmpdir(),'dsh-public-verify-'));
  try {
    for(const target of targets) {
      const dest=path.join(temp,'input',target);fs.mkdirSync(path.join(dest,'evidence'),{recursive:true});
      const archive=`dsh-isolated-runtime-${release.version}-${target}.tar.gz`;
      fs.copyFileSync(path.join(directory,archive),path.join(dest,archive));
      fs.copyFileSync(path.join(directory,`release-${target}.json`),path.join(dest,'release.json'));
      fs.copyFileSync(path.join(directory,`deterministic-${target}.json`),path.join(dest,'evidence/deterministic.json'));
      const sums=fs.readFileSync(path.join(directory,'SHA256SUMS'),'utf8').split('\n').filter(line=>line.endsWith(`  ${archive}`));
      assert.equal(sums.length,1);fs.writeFileSync(path.join(dest,'SHA256SUMS'),sums[0]+'\n');
    }
    fs.copyFileSync(path.join(directory,'macos-host.json'),path.join(temp,'input/darwin-arm64/evidence/macos-host.json'));
    const expected=path.join(temp,'expected');prepare(path.join(temp,'input'),expected);
    for(const file of fs.readdirSync(expected)) assert.deepEqual(fs.readFileSync(path.join(directory,file)),fs.readFileSync(path.join(expected,file)),`Public artifact mismatch: ${file}`);
    console.log(`Both original archives and their evidence match ${release.sourceSHA}`);
  } finally {fs.rmSync(temp,{recursive:true,force:true});}
}
const [mode,input,output]=process.argv.slice(2);
if(mode==='prepare' && input && output) prepare(input,output);
else if(mode==='verify' && input && !output) verify(input);
else throw new Error('usage: public-release.mjs prepare CANDIDATE_ROOT PUBLIC_DIR | verify PUBLIC_DIR');
