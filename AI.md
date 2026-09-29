# AI guide: runtime ownership and installation routing

If the user wants to **install or use** DSH multi-tenant, start with the [platform AI installation guide](https://github.com/GuoMonth/dsh-multi-tenant/blob/main/packages/multi-tenant/AI.md). The platform npm package supplies the CLI, Chart, OIDC/member configuration and two-user acceptance. Do not improvise a separate runtime service or ask users to install the internal Connector.

This repository owns the workload image, launcher, fixed template, namespace/PVC/Sandbox identities, lifecycle and verified in-process transport. Read [AGENTS.md](AGENTS.md), the shared [constitution](CONSTITUTION.md), [environment contract](docs/design/environment-contract.zh-CN.md) and [CONTRIBUTING.md](CONTRIBUTING.md) before source edits. Platform identity/authorization/binding/connection-revocation changes belong in [dsh-multi-tenant](https://github.com/GuoMonth/dsh-multi-tenant).

The only runtime backend is Kubernetes with upstream Agent Sandbox core as sole workload controller. Each owner has one PVC and stable Sandbox/PVC UIDs. Never infer stop from a404, replace a missing volume, adopt a changed UID or resume an unproven writer. Delete retains user data. Platform secrets and Kubernetes credentials must stay outside user workloads; user tool credentials follow the tool's native HOME/DSH_HOME storage.

Release images and source use a simple `vVERSION` tag. Consume the exact image digest in the platform release manifest, not a floating tag. The Connector is private and bundled into the platform; preserve its source.json and SHA256/SHA512 provenance. Runtime RBAC belongs here and the platform packages a checksummed byte-identical copy. See [distribution](docs/distribution.md).

For changes run `npm ci --prefix packages/environment-connector`, then the pinned Go toolchain with `make verify` and `make verify-dsh`. The [real cluster harness](test/environment/README.md) needs a dedicated cluster write owner and disposable fixtures. Preserve user PVCs and private evidence. Fixture tests are not a substitute for actual model calls, tool authorization, OIDC, native HTTP/WS and retained-data acceptance in the platform.
