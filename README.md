# dsh-isolated-runtime

Kubernetes Cell lifecycle and isolation for native DeepSeek Harness. The [multi-tenant platform](https://github.com/GuoMonth/dsh-multi-tenant/blob/main/README.md) owns OIDC, membership, user protocols and sessions; this repository owns Cell resources, images, exact-instance validation and the restricted internal Connector. DSH owns its native Web and tools.

[中文](README.zh-CN.md)

**Current scope: Cell MVP alpha.** The fixed two-user OIDC + Cell flow and real-model file operations passed [integration regression](https://github.com/GuoMonth/dsh-multi-tenant/blob/4ba252765bccb41314c0bdc6b11bcf60cc0b33ef/docs/evidence/cell-regression-2026-09-20.md). The exact tested Linux/amd64 Cell/Operator images are public in [v0.3.0-alpha.1](https://github.com/GuoMonth/dsh-isolated-runtime/releases/tag/v0.3.0-alpha.1), with the same immutable digests; the platform has its own npm release. The current `main` source now includes fixed-template `cell-mvp-v1`, which passed the 2026-09-22 internal candidate run ([report](https://github.com/GuoMonth/dsh-multi-tenant/blob/main/docs/evidence/cell-mvp-2026-09-22.md)). This source candidate has not been republished: public runtime npm `0.3.0-alpha.1` and its Cell/Operator images still describe the older release. To deploy the new template candidate, follow the [platform candidate guide](https://github.com/GuoMonth/dsh-multi-tenant/blob/main/docs/reference/cell-mvp-v1-candidate.md), not the published npm manifest commands below. New iterations can make breaking changes; published artifact identities remain fixed.

## Fixed release boundary

DSH is exactly **0.1.5-rc.2**, source **`fb2c4b9e698e30edb738bca4cf0618587db7d203`**. Each release locks the publicly pullable Cell and Operator images by `@sha256` digest, with matching runtime source and DSH identity in `cell-release.json` (platform) / `release.json` (runtime). Pin the deployed platform image by digest too. npm `latest` selects a package at installation; it does not authorize moving image tags or a DSH version range at runtime.

Breaking updates are allowed: publish a new explicit combination, update configuration/state expectations as needed and validate the affected flow. No compatibility shim, historical upgrade or migration promise is required. Published artifact identities stay immutable. The public [v0.3.0-alpha.1 manifest](https://github.com/GuoMonth/dsh-isolated-runtime/releases/download/v0.3.0-alpha.1/release.json) records the exact Cell/Operator pair, Linux/amd64. Null or mismatched digests block platform publication.


## Integrate with the platform

Administrators supply Kubernetes, enforced CNI policies, storage, tenant namespaces, Gateway API and DNS/TLS. The commands below install published runtime `0.3.0-alpha.1` and render that release's fixed manifests; they do not deploy the newer `cell-mvp-v1` source candidate. Use the [platform candidate guide](https://github.com/GuoMonth/dsh-multi-tenant/blob/main/docs/reference/cell-mvp-v1-candidate.md) for that candidate. The published release's deployment YAML pins its accepted public image; review it and set the environment domain before deployment:

For the fixed platform-mode deployment, the platform Pod must be in the operator’s system namespace (default `dsh-system`) and carry `dsh.isolated.io/access: platform`; see the [platform setup guide](https://github.com/GuoMonth/dsh-multi-tenant/blob/main/docs/reference/quickstart.md). When selecting packages through `latest`, verify that the runtime release matches the platform’s `cell-release.json` before applying its manifests.

```bash
npx dsh-isolated-runtime@latest release
npx dsh-isolated-runtime@latest manifests > /private/operator-rendered.yaml
# Review/edit the example domain; the public Operator digest is already pinned.
kubectl apply --server-side -f /private/operator-rendered.yaml
```

`--access-mode=platform` creates no standalone user authorizer or direct Cell HTTPRoute. The platform handles authorization and proxies to the verified instance. See [platform access](docs/platform-access.md) and the coordinated [startup guide](https://github.com/GuoMonth/dsh-multi-tenant/blob/main/docs/reference/quickstart.md).

The published platform npm entry requires Node 24+:

```bash
npx dsh-multi-tenant@latest start --config /private/config.json
```

Run it with direct API and Pod-IP reachability, normally inside the cluster. This command does not create a cluster. Runtime npm `0.3.0-alpha.1` delivers Cell metadata and deployment YAML only. The published npm package does not provide the old standalone `start/up/stop/uninstall` commands or create a local cluster; their historical source remains under `packages/cli`.

| Layer | Authority |
| --- | --- |
| multi-tenant | OIDC, trusted members, parent/child sessions, allocation intent and protocol admission |
| isolated-runtime | Cell Operator, images, resource lifecycle/ownership, verified transport |
| DSH | Native application sessions, tools, model calls and private application state |

The current regression uses one cluster, one platform replica and fixed versions. Unknown writes require original-key inspection; delete acceptance does not prove writer cessation. See [constitution](CONSTITUTION.md), [RuntimePort](docs/design/runtime-port.zh-CN.md) and [MVP boundaries](docs/alpha-mvp.md).

## Real integrated session

Native DSH behind platform OIDC and a runtime-owned Cell; deepseek-flash wrote/read a file and read an attachment. No model credentials are included.

![Real model and native file tools in a Cell](docs/images/cell-native.png)

![Expanded native tool operations](docs/images/cell-tools.png)

[Full evidence and limits](https://github.com/GuoMonth/dsh-multi-tenant/blob/4ba252765bccb41314c0bdc6b11bcf60cc0b33ef/docs/evidence/cell-regression-2026-09-20.md) · [Distribution boundaries](docs/distribution.md) · [Contributing](CONTRIBUTING.md).

## Historical standalone distribution

Published `v0.2.0-alpha.1` is the local standalone launcher, not this integrated alpha. Use its [versioned instructions](https://github.com/GuoMonth/dsh-isolated-runtime/tree/v0.2.0-alpha.1) for that artifact. Current npm `0.3.0-alpha.1` uses `latest` and replaces the old launcher with Cell manifests. The old implementation remains as historical source under `packages/cli`; this is a breaking change with no automatic migration.

[Documentation map](docs/README.md) · [Exact DSH baseline](compat/dsh/README.md) · [Architecture](docs/specs/architecture.md). License: [LICENSE](LICENSE).
