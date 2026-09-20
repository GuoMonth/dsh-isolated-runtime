#!/usr/bin/env node
// Regenerate deployment YAML from the accepted image source, not mutable main.
import {execFileSync} from 'node:child_process';
import {readFileSync, writeFileSync, mkdtempSync, rmSync} from 'node:fs';
import {tmpdir} from 'node:os';
import {join, resolve} from 'node:path';
import {createHash} from 'node:crypto';
const root = resolve(import.meta.dirname, '..');
const pkg = join(root, 'packages/cell-cli');
const release = JSON.parse(readFileSync(join(pkg, 'release.json')));
if (!/^[a-f0-9]{40}$/.test(release.sourceSHA)) throw new Error('Expected exact runtime source');
const stage = mkdtempSync(join(tmpdir(), 'dsh-cell-manifests-'));
try {
  const archive = execFileSync('git', ['archive', release.sourceSHA, 'config'], {cwd: root});
  execFileSync('tar', ['-xf', '-', '-C', stage], {input: archive});
  let yaml = execFileSync('kubectl', ['kustomize', join(stage, 'config/platform')], {encoding: 'utf8'});
  const placeholder = 'image: ghcr.io/guomonth/dsh-isolated-runtime-operator:main';
  if (yaml.split(placeholder).length !== 2) throw new Error('Expected exactly one operator image');
  yaml = yaml.replace(placeholder, `image: ${release.images.operator}`);
  writeFileSync(join(pkg, 'operator.yaml'), yaml);
  writeFileSync(join(pkg, 'deployment-source.json'), JSON.stringify({sourceSHA: release.sourceSHA, overlay: 'config/platform', sha256: createHash('sha256').update(yaml).digest('hex')}, null, 2) + '\n');
} finally { rmSync(stage, {recursive: true, force: true}); }
