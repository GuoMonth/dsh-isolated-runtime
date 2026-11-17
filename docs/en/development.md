# Develop DSH runtime

[简体中文](../zh-CN/development.md) · [Guides](index.md)

Use Go 1.27.1 and Node 24.11.0 or newer within Node 24, or Node 26 for development. [.node-version](../../.node-version) selects Node 24; Corepack reads `packageManager` from the root [package.json](../../package.json). Exact development tools are locked in [pnpm-lock.yaml](../../pnpm-lock.yaml) against official [DSH 0.2.2-rc.3](https://github.com/deepseek-ai/deepseek-harness/tree/38a6a5c965aa99ab525161efb7cc5cf1fe62fced). This engineering baseline does not change the separate [application pin](../../compat/dsh/baseline.json). Make and the launcher image builder pin Go. `dev-run` is optional; standard Go/Node installations work too.

The published application and Connector retain the Node 24.9.0 compatibility floor. The development floor is higher because the official declaration-build closure includes Babel packages requiring Node 24.11.0. Keep strict install checks enabled; run floor compatibility against built artifacts.

```sh
pnpm install --frozen-lockfile
pnpm fmt:check
pnpm lint
make docs-check
make verify
make verify-dsh
```

The root pnpm workspace owns Connector development. `images/environment` remains outside that workspace: its npm lockfile describes the official published DSH runtime closure and `make verify-dsh` consumes it with `npm ci`. [Connector packing](../../hack/pack-environment-connector.mjs) builds with the frozen pnpm workspace, then uses `npm pack` for the platform's tarball consumer.

TypeScript and JavaScript use two spaces, single quotes, no statement-ending semicolons and multiline trailing commas. TypeScript relative imports use `.ts`; the compiler rewrites emitted imports. `pnpm fmt` applies pinned Oxlint stylistic fixes and gofmt; `pnpm fmt:check` checks JavaScript/TypeScript without writing. Lines over 140 columns need manual wrapping when no safe fix exists. `make lint` also checks Go formatting, UTF-8, LF, whitespace, JSON, shell, JavaScript and Python syntax.

`pnpm lint` runs official strict TypeScript rules. Existing diagnostics are frozen by file, rule, owning statement and declaration, and occurrence count in [lint-baseline.json](../../hack/lint-baseline.json). New or relocated diagnostics fail. `pnpm lint --prune` can only remove resolved allowances; PR CI compares against its trusted base commit and rejects growth. Fetch `origin/main` before local linting; `LINT_BASE_REF` may select a trusted base SHA. `pnpm lint:raw` shows unresolved debt as well. `pnpm test:tooling` exercises the gate's rejection cases.

`make verify` runs source/format/lint checks, builds Go and Connector, runs Go race, Connector and tooling tests, then vet and documentation checks. PR CI runs it on Node 24; Node 26 runs Connector format/lint/types/tests and tooling tests. The Node 24.9 compatibility lane installs and builds with Node 24, then runs the compiled Connector tests on Node 24.9. Workflow syntax is checked on the Node 24 lane. Cluster and publication workflows stay manual. `make verify-dsh` installs the exact DSH package and tests its identity and launcher transport.

`make docs-check` checks paths, anchors, paired languages and command/version agreement. Set `DSH_PLATFORM_DOCS` and `DSH_RUNTIME_DOCS` to paired checkouts for cross-repository links. Technical harness READMEs are English; public guides have both languages.

Identity roles are distinct TypeScript types: owner tenant/principal, allocation, namespace, Sandbox/PVC/Pod UID and resource revision. Admit external values through the exported parsers; keep exact strings. Kubernetes JSON and watch events are parsed before use, preserving unknown fields for exact template comparisons. Incomplete lists cannot establish absence. Transport and domain errors retain internal causes; public diagnostics expose only the fixed contract. A failed write response does not authorize replay.

The launcher reaps its child, closes proxy transports and owns hijacked connections. Every goroutine needs an owner and exit condition. Forced SIGKILL is not a successful environment stop. Go tests use the race detector and goleak; optional pprof captures stay private.

Real Kubernetes lifecycle/volume/network checks use [test/environment](../../test/environment/README.md). Browser and upstream package smoke use [test/rc2](../../test/rc2/README.md); [compat/dsh](../../compat/dsh/README.md) describes the exact DSH baseline and native settings behavior. Use disposable fixtures and one cluster write owner. Model/tool authorization, OIDC and two-user acceptance use the [platform](https://github.com/GuoMonth/dsh-multi-tenant) harness.

Use `git commit -s`. Record actual commands, artifact identities and untested cases in the PR. See [releasing](releasing.md) for package and image preparation.
