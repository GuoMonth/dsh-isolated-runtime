#!/usr/bin/env node
import { readFileSync } from 'node:fs';
import { execFileSync } from 'node:child_process';
const expected = JSON.parse(readFileSync(new URL('../release/cell-alpha.json', import.meta.url)));
for (const kind of ['cell', 'operator']) {
  const pin = expected.images[kind];
  const [image] = JSON.parse(execFileSync('docker', ['image', 'inspect', pin.local], { encoding: 'utf8' }));
  const labels = image.Config.Labels;
  if (image.Id !== pin.digest || `${image.Os}/${image.Architecture}` !== expected.platform ||
      labels['org.opencontainers.image.revision'] !== expected.sourceSHA ||
      labels['org.opencontainers.image.source'] !== 'https://github.com/GuoMonth/dsh-isolated-runtime' ||
      (kind === 'cell' && labels['io.dsh.isolated.dsh.version'] !== expected.dsh.version)) {
    throw new Error(`Accepted ${kind} image identity mismatch`);
  }
}
const baseline = JSON.parse(execFileSync('git', ['show', `${expected.sourceSHA}:compat/dsh/baseline.json`], { encoding: 'utf8' }));
if (baseline.source.version !== expected.dsh.version || baseline.source.commit !== expected.dsh.commit) throw new Error('DSH source mismatch');
console.log('Accepted image indexes, platform, source labels and exact DSH baseline match');
