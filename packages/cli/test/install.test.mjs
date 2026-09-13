import test from 'node:test';
import assert from 'node:assert/strict';
import fs from 'node:fs/promises';
import os from 'node:os';
import path from 'node:path';
import crypto from 'node:crypto';
import * as tar from 'tar';
import {spawnSync} from 'node:child_process';
import {targetFor, validateRelease, install, download, unpack} from '../lib/install.mjs';

const version = '0.2.0-alpha.1';
const target = 'linux-amd64';
const folder = `dsh-isolated-runtime-v${version}-${target}`;
const images = Object.fromEntries(['cell', 'operator'].map(type => [type, `ghcr.io/guomonth/dsh-isolated-runtime-${type}@sha256:${'a'.repeat(64)}`]));
async function fixture(t) {
  const root = await fs.mkdtemp(path.join(os.tmpdir(), 'dsh-cli-test-'));
  t.after(() => fs.rm(root, {recursive: true, force: true}));
  const release = {schemaVersion: 2, version: `v${version}`, sourceSHA: 'b'.repeat(40), images, packages: {}};
  await fs.mkdir(path.join(root, folder));
  await fs.writeFile(path.join(root, folder, 'release.json'), JSON.stringify({...release, schemaVersion: 1, packagePlatform: 'linux/amd64', platform: 'linux/amd64'}));
  await fs.writeFile(path.join(root, folder, 'dsh-runtime'), '#!/bin/sh\nexit 0\n');
  const archive = path.join(root, 'archive.tgz');
  await tar.c({gzip: true, cwd: root, file: archive}, [folder]);
  const body = await fs.readFile(archive);
  release.packages[target] = {archive: `${folder}.tar.gz`, archiveSHA256: crypto.createHash('sha256').update(body).digest('hex')};
  return {root, release, archive, body};
}
test('supported hosts and exact identities', () => {
  assert.equal(targetFor('darwin', 'arm64'), 'darwin-arm64');
  assert.equal(targetFor('linux', 'x64'), 'linux-amd64');
  for (const [os, arch] of [['win32', 'x64'], ['darwin', 'x64'], ['linux', 'arm64']]) assert.throws(() => targetFor(os, arch));
  assert.throws(() => validateRelease({schemaVersion: 2, version: 'v9'}, version, target));
});
test('verified install and cache reuse without network', async t => {
  const f = await fixture(t); let count = 0;
  const fetcher = async url => { count++; assert.match(url, /^https:\/\/github.com\/GuoMonth\//); return new Response(f.body); };
  const cache = path.join(f.root, 'cache with spaces');
  const first = await install(f.release, version, target, cache, fetcher);
  assert.equal(JSON.parse(await fs.readFile(path.join(first.directory, 'release.json'))).version, `v${version}`);
  await first.cleanup();
  const second = await install(f.release, version, target, cache, fetcher);
  assert.equal(count, 1); await second.cleanup();
  assert.equal((await fs.readdir(cache)).filter(name => name.startsWith('launch-')).length, 0);
});
test('tampered download is rejected and removed', async t => {
  const f = await fixture(t); const dest = path.join(f.root, 'bad');
  await assert.rejects(download('https://example.invalid', dest, 'a'.repeat(64), async () => new Response('tampered')), /checksum/);
  await assert.rejects(fs.stat(dest));
});
test('symlinks and foreign archive roots are rejected before extraction', async t => {
  const f = await fixture(t);
  await fs.symlink('/tmp', path.join(f.root, folder, 'link'));
  await tar.c({gzip: true, cwd: f.root, file: f.archive}, [folder]);
  await assert.rejects(unpack(f.archive, f.root, folder), /Unsafe archive/);
  await fs.unlink(path.join(f.root, folder, 'link'));
  await tar.c({gzip: true, cwd: f.root, file: f.archive}, [folder]);
  await assert.rejects(unpack(f.archive, f.root, 'other-root'), /Unsafe archive/);
});
test('cached archive with a forged inner identity never executes', async t => {
  const f = await fixture(t);
  await fs.writeFile(path.join(f.root, folder, 'release.json'), JSON.stringify({version: 'v9'}));
  await tar.c({gzip: true, cwd: f.root, file: f.archive}, [folder]);
  const body = await fs.readFile(f.archive);
  f.release.packages[target].archiveSHA256 = crypto.createHash('sha256').update(body).digest('hex');
  await assert.rejects(install(f.release, version, target, path.join(f.root, 'cache'), async () => new Response(body)), /identity mismatch/);
});
test('source cannot publish or destroy data without explicit confirmation', () => {
  const cli = new URL('../bin/cli.mjs', import.meta.url);
  for (const argv of [['--verify-release'], ['uninstall'], ['unknown'], ['start', '--bogus']]) {
    assert.notEqual(spawnSync(process.execPath, [cli.pathname, ...argv]).status, 0);
  }
  assert.equal(spawnSync(process.execPath, [cli.pathname, '--help']).status, 0);
});
test('bound CLI start invokes the shipped runtime and propagates failures', async t => {
  const f = await fixture(t);
  await fs.writeFile(path.join(f.root, folder, 'dsh-runtime'), '#!/bin/bash\nprintf "%s\\n" "$*" >> "$DSH_TEST_LOG"\nexit "${DSH_TEST_EXIT:-0}"\n');
  await tar.c({gzip: true, cwd: f.root, file: f.archive}, [folder]);
  const body = await fs.readFile(f.archive);
  f.release.packages[target].archiveSHA256 = crypto.createHash('sha256').update(body).digest('hex');
  f.release.packages['darwin-arm64'] = {archive: `dsh-isolated-runtime-v${version}-darwin-arm64.tar.gz`, archiveSHA256: 'a'.repeat(64)};
  const bound = path.join(f.root, 'cli'); await fs.mkdir(bound);
  for (const name of ['bin', 'lib', 'package.json']) await fs.cp(new URL(`../${name}`, import.meta.url), path.join(bound, name), {recursive: true});
  await fs.symlink(new URL('../node_modules', import.meta.url).pathname, path.join(bound, 'node_modules'));
  await fs.writeFile(path.join(bound, 'release.json'), JSON.stringify(f.release));
  const preload = path.join(f.root, 'fetch.mjs');
  await fs.writeFile(preload, `globalThis.fetch=async()=>new Response(Buffer.from('${body.toString('base64')}','base64'));`);
  const log = path.join(f.root, 'commands');
  const env = {...process.env, XDG_CACHE_HOME: path.join(f.root, 'cache'), DSH_TEST_LOG: log};
  const run = (argv, extra = {}) => spawnSync(process.execPath, ['--import', preload, path.join(bound, 'bin/cli.mjs'), ...argv], {env: {...env, ...extra}, encoding: 'utf8'});
  const result = run(['start', '--no-open', '--snapshots']);
  assert.equal(result.status, 0, result.stderr);
  assert.equal(await fs.readFile(log, 'utf8'), 'up --snapshots\n');
  assert.equal(run(['status'], {DSH_TEST_EXIT: '7'}).status, 7);
  assert.equal(run(['uninstall']).status, 1);
  assert.equal(await fs.readFile(log, 'utf8'), 'up --snapshots\nstatus\n');
});
