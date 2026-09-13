#!/usr/bin/env node
// Explicit maintainer operation; never refresh dependency identities at install time.
import fs from 'node:fs';
import path from 'node:path';
import {execFileSync} from 'node:child_process';
const root = path.resolve(import.meta.dirname, '..');
const source = JSON.parse(fs.readFileSync(path.join(root, 'runtime-files/images.json')));
const images = [];
for (const item of source.images) {
  console.error(`Resolve ${item.source}`);
  const manifest = JSON.parse(execFileSync('docker', ['buildx', 'imagetools', 'inspect', item.source, '--format', '{{json .Manifest}}'], {encoding: 'utf8', timeout: 120_000}));
  if (!/^sha256:[a-f0-9]{64}$/.test(manifest.digest)) throw new Error('Missing digest');
  const platforms = manifest.manifests?.map(m => `${m.platform?.os}/${m.platform?.architecture}`) || [];
  if (!['linux/amd64', 'linux/arm64'].every(platform => platforms.includes(platform))) throw new Error(`Missing native platforms: ${item.source}`);
  const repository = item.source.split('@')[0].replace(/:[^/:]+$/, '');
  images.push({...item, ref: `${repository}@${manifest.digest}`, platforms: ['linux/amd64', 'linux/arm64']});
}
fs.writeFileSync(path.join(root, 'runtime-files/images.lock.json'), JSON.stringify({schemaVersion: 1, images}, null, 2) + '\n');
