# Documentation map

Start with the [AgentEnvironment design](https://github.com/GuoMonth/dsh-multi-tenant/blob/main/docs/design/agent-environment.zh-CN.md), [Issue #104](https://github.com/GuoMonth/dsh-multi-tenant/issues/104), and [current implementation status](alpha-mvp.md). The target is Kubernetes-only AgentEnvironment; current source/artifacts still use Cell until the planned source changes. The platform owns user identity and OIDC; runtime owns environment resource mechanics and the verified Connector path.

- [Shared constitution](../CONSTITUTION.md)
- [RuntimePort](design/runtime-port.zh-CN.md) / [Cell adapter](design/cell-adapter.zh-CN.md) — legacy Cell implementation/design references; AgentEnvironment design takes priority
- [Platform access configuration](platform-access.md)
- [Cell Connector](cell-connector.md)
- [Cell MVP scope and acceptance](alpha-mvp.md)
- [Architecture](specs/architecture.md) and [threat model](specs/threat-model.md)
- [Go development](go-development.md)
- [Current distribution contract](distribution.md) and [Cell CLI source README](../packages/cell-cli/README.md)
- [Exact DSH baseline](../compat/dsh/README.md)
- [Current roadmap](../ROADMAP.md)

The target W1 removes Process/Docker product backends, old standalone launch, and snapshot/restore; this does not remove Pod child processes, OCI image builds, or Docker as kind's substrate. Runtime [#100](https://github.com/GuoMonth/dsh-isolated-runtime/issues/100) passed its bounded live upstream check, so W1 adopts the core-only `Sandbox` controller, ordinary Pod and external PVCs, with no synonym CRD or fork. See the [local evidence report](https://github.com/GuoMonth/dsh-multi-tenant/blob/main/docs/evidence/agent-sandbox-local-2026-09-23.md). Production implementation is still Cell and the W1/W2/W3 work is not complete. W2 retains stop evidence, cross-resourceVersion concurrency, unknown deletion barriers and exact old-UID checks. W3 requires OIDC authorization, real tools, persistent HOME and release validation before publication. Backups, hot pools and automatic idle are deferred. Published npm `0.3.0-alpha.1` and its fixed manifests describe that release; source plans are not shipped until separately released. `archive/` contains historical implementation material, not current acceptance.
