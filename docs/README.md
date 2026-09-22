# Documentation map

Start with the [Agent Workspace design](https://github.com/GuoMonth/dsh-multi-tenant/blob/main/docs/design/agent-workspace.zh-CN.md), [Issue #104](https://github.com/GuoMonth/dsh-multi-tenant/issues/104), and [current implementation status](alpha-mvp.md). The target is Kubernetes-only Agent Workspace; current source/artifacts still use Cell until W1. The platform owns user identity and OIDC; runtime owns workspace resource mechanics and the verified Connector path.

- [Shared constitution](../CONSTITUTION.md)
- [RuntimePort](design/runtime-port.zh-CN.md) / [Cell adapter](design/cell-adapter.zh-CN.md) — legacy Cell implementation/design references; Agent Workspace design takes priority
- [Platform access configuration](platform-access.md)
- [Cell Connector](cell-connector.md)
- [Cell MVP scope and acceptance](alpha-mvp.md)
- [Architecture](specs/architecture.md) and [threat model](specs/threat-model.md)
- [Go development](go-development.md)
- [Current distribution contract](distribution.md) and [Cell CLI source README](../packages/cell-cli/README.md)
- [Exact DSH baseline](../compat/dsh/README.md)
- [Current roadmap](../ROADMAP.md)

The target W1 removes Process/Docker product backends, old standalone launch, and snapshot/restore; this does not remove Pod child processes, OCI image builds, or Docker as kind's substrate. W2 plans healthy-node normal Running/Stopped transitions through an explicit StatefulSet and data/private PVCs, retaining both PVC identities without partition-fencing guarantees. W3 requires joint authorization, persistent HOME and performance validation before release. Backups, hot pools and automatic idle are deferred. Published npm `0.3.0-alpha.1` and its fixed manifests describe that release; source plans are not shipped until separately released. `archive/` contains historical implementation material, not current acceptance.
