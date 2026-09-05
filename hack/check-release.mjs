#!/usr/bin/env node
import fs from 'node:fs';
import path from 'node:path';
import crypto from 'node:crypto';
import {execFileSync as run} from 'node:child_process';
const [directory,liveFile] = process.argv.slice(2);
const manifest = JSON.parse(fs.readFileSync(path.join(directory,'release.json')));
if (manifest.schemaVersion !== 1 || !/^[a-f0-9]{40}$/.test(manifest.sourceSHA) || manifest.platform !== 'linux/amd64') throw new Error('Invalid release identity');
for (const type of ['cell','operator']) if (!new RegExp(`^ghcr.io/guomonth/dsh-isolated-runtime-${type}@sha256:[a-f0-9]{64}$`).test(manifest.images[type])) throw new Error('Invalid image identity');
const sums = fs.readFileSync(path.join(directory,'SHA256SUMS'),'utf8').trim();
const match = /^([a-f0-9]{64})  (dsh-isolated-runtime-v[\w.-]+-linux-amd64\.tar\.gz)$/.exec(sums);
if (!match) throw new Error('Invalid checksum manifest');
const archive = path.join(directory,match[2]);
const checksum = crypto.createHash('sha256').update(fs.readFileSync(archive)).digest('hex');
if(checksum !== match[1]) throw new Error('Archive checksum mismatch');
const names = run('tar',['-tzf',archive],{encoding:'utf8'}).trim().split('\n');
if(names.some(name=>name.startsWith('/') || name.split('/').includes('..'))) throw new Error('Unsafe archive');
const inner = JSON.parse(run('tar',['-xOzf',archive,`${match[2].replace(/\.tar\.gz$/,'')}/release.json`],{encoding:'utf8'}));
if(JSON.stringify(inner)!==JSON.stringify(manifest)) throw new Error('Archive release identity mismatch');
if(liveFile) {
  const live = JSON.parse(fs.readFileSync(liveFile));
  if(live.kind!=='live-model' || live.success!==true || live.sourceSHA!==manifest.sourceSHA || live.archiveSHA256!==checksum ||
    JSON.stringify(live.images)!==JSON.stringify(manifest.images) || !live.model) throw new Error('Live acceptance does not match this candidate');
}
console.log(JSON.stringify({sourceSHA:manifest.sourceSHA,archiveSHA256:checksum,images:manifest.images,version:manifest.version}));
