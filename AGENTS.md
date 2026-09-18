# Repository Instructions

This is a Kubernetes isolation runtime, not an npm implementation of DSH.
For helping a user install a release, read docs/ai/local-run.md first.

Product direction and next-milestone acceptance live in docs/alpha-mvp.md
(Chinese: docs/alpha-mvp.zh-CN.md). Enterprise self-hosting is the long-term
goal; current work is alpha MVP. Helm is planned, not delivered. Prioritize
administrator-managed Kubernetes, standard OIDC individual access and kind
validation; do not expand npm/host-managed installation as product scope.
Preserve published-release behavior and existing regression coverage.

- Preserve the Cell/CellSnapshot ownership boundaries and exact DSH baseline.
- The formal local command is dsh-runtime. Legacy demo names may remain in
  internal fixtures and state identities to avoid breaking ownership checks.
- Never erase state merely to make a test pass. Use isolated DSH_RUNTIME_HOME
  directories for local tests and explicitly tear down only test-owned clusters.
- Automatic GitHub CI is limited to Source standards (syntax, formatting and
  LF endings). Keep that name aligned with branch protection. Do not restore
  automatic builds, cluster tests or image publication without authorization.
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
existing Go and kind checks locally, with results recorded in the PR.
Changes to the public local flow also require the
exact-archive acceptance in hack/verify-mvp.sh.
