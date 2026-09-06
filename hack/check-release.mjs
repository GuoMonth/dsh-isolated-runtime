#!/usr/bin/env node
import fs from 'node:fs';
import path from 'node:path';
import crypto from 'node:crypto';
import {execFileSync as run} from 'node:child_process';
const [directory,proofFile,expectedKind='live-model'] = process.argv.slice(2);
if (!['live-model','deterministic'].includes(expectedKind)) throw new Error('Invalid acceptance kind');
const manifest = JSON.parse(fs.readFileSync(path.join(directory,'release.json')));
const platforms = {'linux/amd64':'linux/amd64','darwin/arm64':'linux/arm64'};
if (manifest.schemaVersion !== 1 || !/^[a-f0-9]{40}$/.test(manifest.sourceSHA) ||
    !/^v\d+\.\d+\.\d+(?:-[a-z0-9.]+)?$/.test(manifest.version) || !platforms[manifest.packagePlatform] ||
    manifest.platform !== platforms[manifest.packagePlatform]) throw new Error('Invalid release identity');
for (const type of ['cell','operator']) if (!new RegExp(`^ghcr.io/guomonth/dsh-isolated-runtime-${type}@sha256:[a-f0-9]{64}$`).test(manifest.images[type])) throw new Error('Invalid image identity');
const sums = fs.readFileSync(path.join(directory,'SHA256SUMS'),'utf8').trim();
const match = /^([a-f0-9]{64})  (dsh-isolated-runtime-v[\w.-]+-(?:linux-amd64|darwin-arm64)\.tar\.gz)$/.exec(sums);
if (!match || match[2]!==`dsh-isolated-runtime-${manifest.version}-${manifest.packagePlatform.replace('/','-')}.tar.gz`) throw new Error('Invalid checksum manifest');
const archive = path.join(directory,match[2]);
const checksum = crypto.createHash('sha256').update(fs.readFileSync(archive)).digest('hex');
if(checksum !== match[1]) throw new Error('Archive checksum mismatch');
const names = run('tar',['-tzf',archive],{encoding:'utf8'}).trim().split('\n');
if(names.some(name=>name.startsWith('/') || name.split('/').includes('..'))) throw new Error('Unsafe archive');
const inner = JSON.parse(run('tar',['-xOzf',archive,`${match[2].replace(/\.tar\.gz$/,'')}/release.json`],{encoding:'utf8'}));
if(JSON.stringify(inner)!==JSON.stringify(manifest)) throw new Error('Archive release identity mismatch');
if(proofFile) {
  const proof = JSON.parse(fs.readFileSync(proofFile));
  if(proof.kind!==expectedKind || proof.success!==true || proof.sourceSHA!==manifest.sourceSHA || proof.archiveSHA256!==checksum ||
    JSON.stringify(proof.images)!==JSON.stringify(manifest.images) || proof.packagePlatform!==manifest.packagePlatform ||
    proof.platform!==manifest.platform || proof.version!==manifest.version || !proof.model) throw new Error(`${expectedKind} acceptance does not match this candidate`);
}
console.log(JSON.stringify({sourceSHA:manifest.sourceSHA,archiveSHA256:checksum,images:manifest.images,version:manifest.version,packagePlatform:manifest.packagePlatform,platform:manifest.platform}));
