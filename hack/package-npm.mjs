#!/usr/bin/env node
import fs from 'node:fs';
import path from 'node:path';
import os from 'node:os';
import {execFileSync} from 'node:child_process';
const root = path.resolve(import.meta.dirname, '..');
const [input, output] = process.argv.slice(2);
if (!input || !output) throw new Error('usage: package-npm.mjs VERIFIED_PUBLIC_RELEASE OUTPUT');
execFileSync(process.execPath, [path.join(root, 'hack/public-release.mjs'), 'verify', input], {stdio: 'inherit'});
const manifest = JSON.parse(fs.readFileSync(path.join(input, 'release.json')));
const pkg = JSON.parse(fs.readFileSync(path.join(root, 'packages/cli/package.json')));
if (`v${pkg.version}` !== manifest.version) throw new Error('npm version must match the accepted release');
const stage = fs.mkdtempSync(path.join(os.tmpdir(), 'dsh-npm-'));
try {
  for (const file of ['package.json', 'README.md', 'bin', 'lib']) fs.cpSync(path.join(root, 'packages/cli', file), path.join(stage, file), {recursive: true});
  fs.copyFileSync(path.join(root, 'LICENSE'), path.join(stage, 'LICENSE'));
  fs.copyFileSync(path.join(input, 'release.json'), path.join(stage, 'release.json'));
  fs.mkdirSync(path.resolve(output), {recursive: true});
  execFileSync('npm', ['pack', '--ignore-scripts', '--pack-destination', path.resolve(output)], {cwd: stage, stdio: 'inherit'});
} finally { fs.rmSync(stage, {recursive: true, force: true}); }
