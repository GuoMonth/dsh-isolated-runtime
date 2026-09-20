import {test} from 'node:test';
import assert from 'node:assert/strict';
import {spawnSync} from 'node:child_process';
import {cpSync, mkdtempSync, readFileSync, rmSync, writeFileSync} from 'node:fs';
import {tmpdir} from 'node:os';
import {join, resolve} from 'node:path';
const root = resolve(import.meta.dirname, '..');
const run = (command, directory = root) => spawnSync(process.execPath, [join(directory, 'bin/cli.mjs'), command], {encoding: 'utf8'});
test('release and manifests bind the accepted platform-mode operator', () => {
  const result = run('release'); assert.equal(result.status, 0);
  const release = JSON.parse(result.stdout);
  assert.equal(release.version, 'v0.3.0-alpha.1');
  const manifests = run('manifests'); assert.equal(manifests.status, 0);
  assert.ok(manifests.stdout.includes(release.images.operator));
  assert.ok(manifests.stdout.includes('--access-mode=platform'));
  assert.ok(!manifests.stdout.includes('kind: HTTPRoute'));
  assert.ok(!manifests.stdout.includes('image: controller:latest'));
});
test('old destructive or standalone commands fail without cluster operations', () => {
  for (const command of ['start', 'up', 'stop', 'uninstall']) {
    const result = run(command); assert.equal(result.status, 1); assert.equal(result.stdout, '');
    assert.equal(JSON.parse(result.stderr).effect, 'not-submitted');
  }
});
test('changed manifest or release identity fails before emitting deployment YAML', () => {
  for (const file of ['operator.yaml', 'release.json']) {
    const stage = mkdtempSync(join(tmpdir(), 'dsh-cli-test-'));
    try {
      cpSync(root, stage, {recursive: true});
      const target = join(stage, file);
      writeFileSync(target, file.endsWith('.yaml') ? readFileSync(target, 'utf8') + '# tampered\n' : '{}\n');
      const result = run('manifests', stage);
      assert.equal(result.status, 1); assert.equal(result.stdout, '');
      assert.equal(JSON.parse(result.stderr).code, 'INVALID_RELEASE');
    } finally { rmSync(stage, {recursive: true, force: true}); }
  }
});
