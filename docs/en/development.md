# Develop DSH runtime

[简体中文](../zh-CN/development.md) · [Guides](index.md)

Use Go 1.27.4 and Node 24.22.3 or later within 24.x, or Node 26.13.1 or later. [.node-version](../../.node-version) pins the development Node version. Corepack uses `packageManager` in the root [package.json](../../package.json). The [pnpm lockfile](../../pnpm-lock.yaml) fixes the tooling. `go.mod` selects the Go version for Make, CI and the builder. The workload image pins both toolchain images by digest.

```sh
pnpm install --frozen-lockfile
pnpm fmt:check
pnpm lint
make docs-check
make verify
make verify-dsh
```

The root pnpm workspace owns Connector development. `images/environment` remains outside that workspace: its npm lockfile describes the official published DSH runtime closure and `make verify-dsh` consumes it with `npm ci`. [Connector packing](../../hack/pack-environment-connector.mjs) builds with the frozen pnpm workspace, then uses `npm pack` for the platform's tarball consumer.

TypeScript and JavaScript use two spaces, single quotes, no statement-ending semicolons and multiline trailing commas. TypeScript relative imports use `.ts`; the compiler rewrites emitted imports. `pnpm fmt` applies pinned Oxlint stylistic fixes and gofmt; `pnpm fmt:check` checks JavaScript/TypeScript without writing. Lines over 140 columns need manual wrapping when no safe fix exists. `make lint` also checks Go formatting, UTF-8, LF, whitespace, JSON, shell, JavaScript and Python syntax and runs the pinned `golangci-lint` with [.golangci.yml](../../.golangci.yml), including `goimports` and the HTTP response body check.

`pnpm lint` runs official strict TypeScript rules. Existing diagnostics are frozen by file, rule, owning statement and declaration, and occurrence count in [lint-baseline.json](../../hack/lint-baseline.json). New or relocated diagnostics fail. `pnpm lint --prune` can only remove resolved allowances; PR CI compares against its trusted base commit and rejects growth. Fetch `origin/main` before local linting; `LINT_BASE_REF` may select a trusted base SHA. `pnpm lint:raw` shows unresolved debt as well. `pnpm test:tooling` exercises the gate's rejection cases.

`make verify` runs source, format and strict type checks, Go and Connector builds, race and Connector tests, `go vet`, documentation checks and security scans. CI runs this on Node 24.22.3 and 26.13.1. It also checks workflow syntax and runs `make verify-dsh` on both versions when DSH integration inputs change. That gate installs the exact npm closure and uses race-enabled Go tests to verify launcher transport and Session creation, streaming, export and recovery across a restart. Cluster and publication workflows are manual.

`make docs-check` checks paths, anchors, paired languages and command/version agreement. Set `DSH_PLATFORM_DOCS` and `DSH_RUNTIME_DOCS` to paired checkouts for cross-repository links. Technical harness READMEs are English; public guides have both languages.

Identity roles are distinct TypeScript types: owner tenant/principal, allocation, namespace, Sandbox/PVC/Pod UID and resource revision. Admit external values through the exported parsers; keep exact strings. Kubernetes JSON and watch events are parsed before use, preserving unknown fields for exact template comparisons. Incomplete lists cannot establish absence. Transport and domain errors retain internal causes; public diagnostics expose only the fixed contract. A failed write response does not authorize replay.

Allocation operations may receive a platform binding with additional control metadata. Project its allocation key and owner before constructing a resource reference; never spread platform state into that strict identity. Both initial creation and read-only discovery must return the same exact reference without replaying accepted writes.

Parse Kubernetes wire responses rather than assuming kubectl's printed shape. Typed PodList/EndpointSliceList items may omit apiVersion/kind; derive those fields only from a validated list header, reject conflicting item types and incomplete pagination, and retain every other field for identity and template verification.

New Services can have owned EndpointSlices with null endpoints and ports. Preserve these empty wire collections and report Pending until the controller publishes a ready target; ownership and populated endpoint/port checks remain strict.

The launcher reaps its child, closes proxy transports and owns hijacked connections. Management binds before the child starts, and every exit path joins both servers and child cleanup. `os/exec` owns output copying; output is drained before process completion is published, and `Cmd.WaitDelay` bounds inherited pipes after child exit. Forced SIGKILL is not a successful environment stop.

Keep Go wire data in concrete structs, pass context through blocking work and wrap returned errors with their operation. Each goroutine needs a clear stop and join point. Node engineering scripts use the root strict `tsconfig.json` and version-pinned Oxlint.

Use the official `go fix -diff ./...` to review applicable modernizations; `errors.AsType`, `strings.SplitSeq`, typed atomics and `WaitGroup.Go` keep intent explicit. Connection-drain tests use `testing/synctest` and its virtual clock instead of scheduling sleeps. Preserve JSON wire semantics when choosing a newer API.

Launcher and command tests run both goleak and Go 1.27's native `goroutineleak` profile after test cleanup. The native profile triggers its own detection GC and catches unreachable permanently blocked goroutines; goleak also catches goroutines still reachable through globals. Subprocess negative controls prove both detectors reject leaks. Race checks cover executed paths, not every possible interleaving. Profiles stay in test diagnostics; the workload exposes no pprof HTTP endpoint.

The Connector pins Zod 4.6.7; x/net is updated to 0.62.0. goleak 1.3.0 is its current tagged release. The WebSocket probe retains its newer Gorilla pseudo-version rather than downgrading to the older latest tag. Dependency review uses module updates and vulnerability reports; all official DSH packages retain their exact RC pin; three reviewed transitive SDK overrides close known advisories, as documented in [compat/dsh](../../compat/dsh/README.md).

Real Kubernetes lifecycle/volume/network checks use [test/environment](../../test/environment/README.md). Browser and upstream package smoke use [test/rc2](../../test/rc2/README.md); [compat/dsh](../../compat/dsh/README.md) describes the exact DSH baseline and native settings behavior. Use disposable fixtures and one cluster write owner. Model/tool authorization, OIDC and two-user acceptance use the [platform](https://github.com/GuoMonth/dsh-multi-tenant) harness.

Use `git commit -s`. Record actual commands, artifact identities and untested cases in the PR. See [releasing](releasing.md) for package and image preparation.
