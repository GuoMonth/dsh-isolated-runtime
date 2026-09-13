# Repository Instructions

This is a Kubernetes isolation runtime, not an npm implementation of DSH.
For helping a user install a release, read docs/ai/local-run.md first.

- Preserve the Cell/CellSnapshot ownership boundaries and exact DSH baseline.
- The formal local command is dsh-runtime. Legacy demo names may remain in
  internal fixtures and state identities to avoid breaking ownership checks.
- Never erase state merely to make a test pass. Use isolated DSH_RUNTIME_HOME
  directories for local tests and explicitly tear down only test-owned clusters.
- Do not rename required CI checks without updating their branch-protection
  contract. Historical MVP check names are not public product branding.
- npm is a thin version-bound launcher. Do not publish packages, create release
  tags, promote images or change package visibility without release authorization.
- A source checkout is deliberately not an installable release. Packaging binds
  accepted image digests, archives and source SHA; never bypass identity checks.
- Do not assert Mac Docker Desktop or live-model acceptance from Linux fixtures.

Focused verification:

```sh
npm ci --ignore-scripts --prefix packages/cli
npm test --prefix packages/cli
npm ci --ignore-scripts --prefix runtime-files
node --test test/local-runtime.test.cjs
shellcheck -x dsh-runtime demo demo-files/host.sh demo-files/tools.sh demo-files/forward.sh
```

After committing all release inputs, node hack/verify-release-contract.mjs
checks the real archive packer/verifier. Broader runtime changes require the
existing Go and kind gates. Changes to the public local flow also require the
exact-archive acceptance in hack/verify-mvp.sh.
