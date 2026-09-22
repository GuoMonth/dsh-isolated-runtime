# Repository instructions

[CONSTITUTION.md](CONSTITUTION.md) links shared principles. The target is Kubernetes-only Agent Workspace with a thin runtime-owned CRD/Operator; `AgentWorkspace` is a planned W1 breaking rename, while current source still implements `Cell`. Scope/sequence live in the [Agent Workspace design](https://github.com/GuoMonth/dsh-multi-tenant/blob/main/docs/design/agent-workspace.zh-CN.md) and platform [Issue #104](https://github.com/GuoMonth/dsh-multi-tenant/issues/104).

## Task routing

- Current Cell implementation context: [RuntimePort](docs/design/runtime-port.zh-CN.md), [Cell adapter](docs/design/cell-adapter.zh-CN.md), and [architecture](docs/specs/architecture.md). These do not supersede the Agent Workspace design or claim the rename/stop-start work is implemented.
- Development checks: [CONTRIBUTING.md](CONTRIBUTING.md); Go details in [Go development](docs/go-development.md).
- Current release instructions: [distribution](docs/distribution.md) and [documentation index](docs/README.md). Historical standalone instructions are under `docs/archive/`.

## Repository-specific constraints

- Kubernetes/Gateway/CSI own resource mechanics. Namespace is administrator-configured infrastructure scope, not OIDC tenantId; use preconfigured per-user namespaces. Platform authorization is the sole user-owner authority; runtime stores immutable owner linkage only for resource matching and does not implement OIDC. Runtime may directly manage native StatefulSet/PVC/Service resources; separation does not force those operations into the platform. DSH owns application protocols. Validate exact workspace and Pod identity.
- Product execution backend is Kubernetes only. Do not treat DSH child processes, OCI image builds, or kind's Docker substrate as product Process/Docker runtime backends.
- Preserve ownership checks and any legacy fixture/state names still used by current code or tests; their presence does not make historical flows current product requirements.
- Treat published packages, images and manifests as immutable release evidence. A source checkout or candidate PR is not an installable release; artifact acceptance binds actual images, package contents and source SHA.
- Automatic CI is only `Source standards`; do not introduce automatic builds, cluster runs or publication. Local checks can proceed within the task; releases, deployment and data deletion follow existing user authorization.
- Contributions use DCO (`git commit -s`). Report untested environments without inferring them from another platform's fixtures.
