const test = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const os = require('node:os');
const path = require('node:path');
const crypto = require('node:crypto');
const {spawnSync, execFileSync} = require('node:child_process');
const repo = path.resolve(__dirname, '..');
function fixture(t) {
  const root = fs.mkdtempSync(path.join(os.tmpdir(), 'dsh-control-test-'));
  t.after(() => fs.rmSync(root, {recursive: true, force: true}));
  const state = path.join(root, 'state with spaces'); fs.mkdirSync(state);
  const bin = path.join(root, 'bin'); fs.mkdirSync(bin);
  const cluster = 'dsh-demo-' + crypto.createHash('sha256').update(state).digest('hex').slice(0, 10);
  fs.writeFileSync(path.join(bin, 'docker'), `#!/bin/bash
case "$1" in
info) echo linux/amd64 ;;
inspect) if [[ "$*" == *State.Running* ]]; then echo "\${TEST_RUNNING:-true}"; else echo "$TEST_CLUSTER"; fi ;;
stop) echo stop >> "$TEST_LOG" ;;
*) exit 1 ;;
esac
`, {mode: 0o700});
  fs.writeFileSync(path.join(bin, 'kind'), '#!/bin/bash\necho "$*" >> "$TEST_LOG"\n', {mode: 0o700});
  const env = {...process.env, DSH_DEMO_HOME: '', DSH_RUNTIME_HOME: state, PATH: `${bin}:${process.env.PATH}`, TEST_CLUSTER: cluster, TEST_LOG: path.join(root, 'calls')};
  const run = (args, extra = {}) => spawnSync('bash', [path.join(repo, 'dsh-runtime'), ...args], {env: {...env, ...extra}, encoding: 'utf8'});
  return {root, state, cluster, env, run};
}
test('status JSON, command validation, legacy path conflict', t => {
  const f = fixture(t);
  assert.equal(JSON.parse(f.run(['status', '--json']).stdout).state, 'not-installed');
  assert.equal(f.run(['unknown']).status, 2);
  assert.equal(f.run(['uninstall']).status, 2);
  assert.equal(f.run(['status'], {DSH_DEMO_HOME: '/different'}).status, 2);
  assert.match(f.run(['up']).stderr, /Use the release bundle/);
  assert.equal(fs.existsSync(f.env.TEST_LOG), false);
});
test('stop preserves data and uninstall requires exact ownership', t => {
  const f = fixture(t);
  fs.writeFileSync(path.join(f.state, 'owner'), f.cluster);
  fs.writeFileSync(path.join(f.state, 'kubeconfig'), 'test');
  fs.mkdirSync(path.join(f.state, 'runtime'));
  fs.writeFileSync(path.join(f.state, 'runtime', 'marker'), 'retained');
  assert.equal(f.run(['stop']).status, 0);
  assert.equal(fs.readFileSync(path.join(f.state, 'runtime', 'marker'), 'utf8'), 'retained');
  assert.equal(JSON.parse(f.run(['status', '--json'], {TEST_RUNNING: 'false'}).stdout).state, 'stopped');
  assert.equal(f.run(['uninstall', '--yes'], {TEST_CLUSTER: 'another-cluster'}).status, 1);
  assert.equal(fs.existsSync(path.join(f.state, 'owner')), true);
  assert.equal(f.run(['uninstall', '--yes']).status, 0);
  assert.equal(f.run(['uninstall', '--yes']).status, 0);
  assert.match(fs.readFileSync(f.env.TEST_LOG, 'utf8'), /delete cluster --name dsh-demo-/);
  assert.equal(fs.existsSync(path.join(f.state, 'kubeconfig')), false);
});
test('identity is random, private, stable, and rendered as a Secret', t => {
  const f = fixture(t); fs.mkdirSync(path.join(f.state, 'identity'));
  fs.symlinkSync(path.join(repo, 'runtime-files/node_modules'), path.join(f.state, 'identity/node_modules'));
  const script = path.join(repo, 'runtime-files/identity.cjs');
  execFileSync(process.execPath, [script, 'prepare', f.state]);
  const first = fs.readFileSync(path.join(f.state, 'credentials.json'), 'utf8');
  execFileSync(process.execPath, [script, 'prepare', f.state]);
  assert.equal(fs.readFileSync(path.join(f.state, 'credentials.json'), 'utf8'), first);
  assert.equal(fs.statSync(path.join(f.state, 'credentials.json')).mode & 0o777, 0o600);
  const credentials = JSON.parse(first); assert.ok(credentials.password.length >= 32);
  const input = {items: [{kind: 'ConfigMap', metadata: {name: 'dex'}, data: {}}, {kind: 'Deployment', metadata: {name: 'dex'}, spec: {template: {spec: {volumes: [{name: 'config', configMap: {name: 'dex'}}]}}}}]};
  const result = JSON.parse(execFileSync(process.execPath, [script, 'render', f.state], {input: JSON.stringify(input)}));
  assert.equal(result.items[0].kind, 'Secret');
  const config = JSON.parse(result.items[0].stringData['config.yaml']);
  assert.equal(config.staticPasswords.length, 1);
  assert.equal(config.staticPasswords[0].email, credentials.email);
  assert.equal(require('../runtime-files/node_modules/bcryptjs').compareSync(credentials.password, config.staticPasswords[0].hash), true);
  assert.deepEqual(result.items[1].spec.template.spec.volumes[0].secret, {secretName: 'dex'});
});
test('reference image lock covers each declared source with both native architectures', () => {
  const inventory = JSON.parse(fs.readFileSync(path.join(repo, 'runtime-files/images.json')));
  const lock = JSON.parse(fs.readFileSync(path.join(repo, 'runtime-files/images.lock.json')));
  assert.deepEqual(lock.images.map(item => item.source), inventory.images.map(item => item.source));
  assert.equal(new Set(lock.images.map(item => item.source)).size, lock.images.length);
  for (const item of lock.images) {
    assert.match(item.ref, /@sha256:[a-f0-9]{64}$/);
    assert.deepEqual(item.platforms, ['linux/amd64', 'linux/arm64']);
    if (item.source.includes('@')) assert.equal(item.ref.split('@')[1], item.source.split('@')[1]);
  }
});
