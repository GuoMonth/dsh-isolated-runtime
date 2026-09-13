#!/usr/bin/env node
import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';
import {fileURLToPath} from 'node:url';
import {spawn} from 'node:child_process';
import {install, targetFor, validateRelease} from '../lib/install.mjs';
const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..');
const {version} = JSON.parse(fs.readFileSync(path.join(root, 'package.json')));
const [command = 'start', ...args] = process.argv.slice(2);
async function main() {
  if (['--help', '-h', 'help'].includes(command)) {
    console.log('dsh-runtime [start [--no-open] [--snapshots] | up [--snapshots] | open | doctor [--json] | status [--json] | credentials | stop | uninstall --yes]\nRequires Docker. start installs, starts and opens the local runtime. stop retains data. uninstall --yes destroys data.'); return;
  }
  if (command === '--version') { console.log(version); return; }
  const allowed = {start: ['--no-open', '--snapshots'], up: ['--snapshots'], open: [], doctor: ['--json'], status: ['--json'], credentials: [], stop: [], uninstall: ['--yes'], '--verify-release': []};
  if (!Object.hasOwn(allowed, command) || args.some(arg => !allowed[command].includes(arg)) || new Set(args).size !== args.length) throw new Error('Invalid command or option; use --help');
  if (command === 'uninstall' && !args.includes('--yes')) throw new Error('Uninstall deletes ALL local cluster data; explicit --yes required');
  const releaseFile = path.join(root, 'release.json');
  if (!fs.existsSync(releaseFile)) throw new Error('This launcher is not bound to an accepted release. Build it with hack/package-npm.mjs; do not publish the source directory.');
  const release = JSON.parse(fs.readFileSync(releaseFile));
  for (const target of ['linux-amd64', 'darwin-arm64']) validateRelease(release, version, target);
  if (command === '--verify-release') { console.log(`Bound to ${release.version} (${release.sourceSHA})`); return; }
  const target = targetFor(process.platform, process.arch);
  if (command === 'start' && !args.includes('--no-open') && process.platform === 'linux' && !process.env.DISPLAY && !process.env.WAYLAND_DISPLAY) throw new Error('No graphical session; use start --no-open');
  const cache = path.join(process.env.XDG_CACHE_HOME || path.join(os.homedir(), '.cache'), 'dsh-isolated-runtime');
  const installation = await install(release, version, target, cache);
  const run = (argv) => new Promise((resolve, reject) => {
    const child = spawn('bash', [path.join(installation.directory, 'dsh-runtime'), ...argv], {stdio: 'inherit'});
    const interrupt = signal => child.kill(signal);
    const int = () => interrupt('SIGINT'); const term = () => interrupt('SIGTERM');
    process.once('SIGINT', int); process.once('SIGTERM', term);
    child.once('error', reject);
    child.once('exit', (code, signal) => {
      process.removeListener('SIGINT', int); process.removeListener('SIGTERM', term);
      resolve(code ?? (signal === 'SIGINT' ? 130 : 143));
    });
  });
  try {
    if (command === 'start') {
      process.exitCode = await run(['up', ...args.filter(arg => arg === '--snapshots')]);
      if (!process.exitCode && !args.includes('--no-open')) process.exitCode = await run(['open']);
    } else process.exitCode = await run([command, ...args]);
  } finally { await installation.cleanup(); }
}
main().catch(error => { console.error(error.message); process.exitCode = 1; });
