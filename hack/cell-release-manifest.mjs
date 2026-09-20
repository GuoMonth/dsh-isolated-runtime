#!/usr/bin/env node
// Existing-cluster delivery metadata only. Does not build, publish or infer acceptance.
import { execFileSync } from 'node:child_process';
import { writeFileSync } from 'node:fs';
import { resolve } from 'node:path';
const [version, source, cell, operator, output, ...extra] = process.argv.slice(2);
if (extra.length || !/^v\d+\.\d+\.\d+-alpha\.\d+$/.test(version ?? '') || !/^[a-f0-9]{40}$/.test(source ?? '') || !output) {
  throw new Error('Usage: cell-release-manifest.mjs VERSION EXACT_TESTED_SOURCE CELL_DIGEST OPERATOR_DIGEST OUTPUT');
}
for (const [name, ref] of [['cell', cell], ['operator', operator]]) {
  const prefix = `ghcr.io/guomonth/dsh-isolated-runtime-${name}@sha256:`;
  if (!ref?.startsWith(prefix) || !/^[a-f0-9]{64}$/.test(ref.slice(prefix.length))) throw new Error(`Invalid immutable ${name} image`);
}
const root = resolve(import.meta.dirname, '..');
const git = args => execFileSync('git', args, { cwd: root, encoding: 'utf8', stdio: ['ignore', 'pipe', 'pipe'] });
if (git(['rev-parse', `${source}^{commit}`]).trim() !== source) throw new Error('Source must identify a repository commit');
const baseline = JSON.parse(git(['show', `${source}:compat/dsh/baseline.json`]));
const manifest = { schemaVersion: 1, distribution: 'existing-kubernetes-cell', version, sourceSHA: source,
  baseline, images: { cell, operator } };
// A manifest records approved inputs, not proof that those images were tested or published.
writeFileSync(resolve(output), JSON.stringify(manifest, null, 2) + '\n', { flag: 'wx' });
console.log('Created Cell release manifest; verify image provenance and acceptance before publication');
