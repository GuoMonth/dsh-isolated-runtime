# DSH Isolated Runtime

[中文](README.zh-CN.md) · [Install the platform](https://github.com/GuoMonth/dsh-multi-tenant#install) · [AI guide](AI.md)

The Kubernetes resource and execution layer for [DSH multi-tenant](https://github.com/GuoMonth/dsh-multi-tenant). It supplies the fixed DSH workload image, launcher and in-process environment Connector. **Install through the platform**; it bundles the Connector and owns the installation CLI and Helm chart.

DSH is pinned to **0.2.0-rc.2**. Each environment has one independently owned PVC at `/var/lib/dsh/data`, with `workspace/` for files, `home/` for user tools/configuration and `dsh/` for native conversations/credentials. Normal stop/start and Pod recreation retain the same volume. CPU/memory use Kubernetes requests/limits and storage has one requested capacity.

## Responsibilities

| Component | Owns |
| --- | --- |
| This repository | Fixed workload image and launcher, namespace/PVC/Sandbox identity, explicit create/query/stop/start/delete, verified access channel |
| [Platform](https://github.com/GuoMonth/dsh-multi-tenant) | OIDC, members, user authorization, allocation bindings, connection revocation, npm CLI and installation |
| Upstream Agent Sandbox core | Sole controller reconciling Sandbox into Pod/Service |
| DeepSeek Harness | Native Web, conversations, model/tool execution and user credentials |

The Connector runs inside the platform process. It is an internal bundled package, not a separate public npm dependency or network service. User authorization always belongs to the platform; resource labels and namespaces are not authorization.

## Installation with AI

Give your assistant the [platform AI installation guide](https://github.com/GuoMonth/dsh-multi-tenant/blob/main/packages/multi-tenant/AI.md). It checks cluster, OIDC, DNS/TLS, storage and release image identities before using the packaged installer. Runtime contributors use [this repository's AI guide](AI.md) and [executable contract](docs/design/environment-contract.zh-CN.md).

## Lifecycle and limits

Stop requires positive evidence for the exact writer on a healthy node. Unknown outcomes are inspected against the original allocation; missing/replaced PVC identity fails closed. Delete requires verified stop and retains the PVC, namespace and isolation policy. Automatic node-partition recovery, backup, HA and data migration are outside the Alpha guarantees.

- [Connector implementation](packages/environment-connector/src/runtime.ts) and [fixed template](packages/environment-connector/src/template.ts)
- [Platform RBAC](config/runtime/cluster-role.yaml)
- [Development and checks](CONTRIBUTING.md)
- [Image and Connector distribution](docs/distribution.md)
- [Releases](https://github.com/GuoMonth/dsh-isolated-runtime/releases)

MIT licensed.
