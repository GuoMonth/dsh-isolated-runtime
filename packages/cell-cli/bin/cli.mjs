#!/usr/bin/env node
import {readFileSync} from 'node:fs';
import {createHash} from 'node:crypto';
const root = new URL('../', import.meta.url);
const read = name => readFileSync(new URL(name, root), 'utf8');
const pkg = JSON.parse(read('package.json'));
const [command = '--help', ...args] = process.argv.slice(2);
function fail(code, message, nextAction) {
  console.error(JSON.stringify({code, stage: 'cell-distribution', effect: 'not-submitted', retry: false, message, nextAction}));
  process.exitCode = 1;
}
try {
  if (args.length) {
    fail('INVALID_ARGUMENT', 'Unexpected arguments.', 'Run dsh-runtime --help.');
  } else if (['--help', '-h', 'help'].includes(command)) {
    console.log(`dsh-runtime ${pkg.version} — existing Kubernetes Cell distribution

  release           Print fixed runtime source, DSH version and image digests as JSON
  manifests         Print platform-mode Operator, CRD and RBAC YAML for review
  --version         Print npm package version
  --verify-release  Check bundled release identity and deployment integrity

Prepare K8s, NetworkPolicy enforcement, storage and tenant configuration first.
Review manifests before applying with kubectl. This CLI does not modify a cluster.
Start OIDC/user access with dsh-multi-tenant@0.9.0-alpha.1.
Breaking change: 0.2 standalone start/up/stop/uninstall commands are removed.`);
  } else if (command === '--version') {
    console.log(pkg.version);
  } else if (['release', 'manifests', '--verify-release'].includes(command)) {
    const release = JSON.parse(read('release.json'));
    const deployment = JSON.parse(read('deployment-source.json'));
    const yaml = read('operator.yaml');
    if (release.schemaVersion !== 1 || release.distribution !== 'existing-kubernetes-cell' || release.version !== `v${pkg.version}` || !/^[a-f0-9]{40}$/.test(release.sourceSHA)) throw new Error('Invalid identity');
    for (const kind of ['cell', 'operator']) {
      if (!new RegExp(`^ghcr.io/guomonth/dsh-isolated-runtime-${kind}@sha256:[a-f0-9]{64}$`).test(release.images[kind])) throw new Error('Invalid image');
    }
    if (deployment.sourceSHA !== release.sourceSHA || deployment.sha256 !== createHash('sha256').update(yaml).digest('hex') || !yaml.includes(`image: ${release.images.operator}`) || !yaml.includes('--access-mode=platform')) throw new Error('Invalid deployment');
    if (command === 'release') process.stdout.write(read('release.json'));
    else if (command === 'manifests') process.stdout.write(yaml);
    else console.log(`Verified ${release.version}; runtime source ${release.sourceSHA}; DSH ${release.baseline.source.version}`);
  } else {
    fail('UNSUPPORTED_COMMAND', 'This version delivers Kubernetes Cell manifests; standalone commands are not supported.', 'Run dsh-runtime manifests, then follow the README for existing-cluster setup.');
  }
} catch {
  fail('INVALID_RELEASE', 'Bundled release identity or deployment integrity is invalid.', 'Reinstall this exact npm version from the public registry; do not apply its manifests.');
}
