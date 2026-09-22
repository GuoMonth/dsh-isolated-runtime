# Agent Workspace architecture direction

> **Design target, not current implementation.** The authoritative design and sequence are in the [platform Agent Workspace design](https://github.com/GuoMonth/dsh-multi-tenant/blob/main/docs/design/agent-workspace.zh-CN.md) and [Issue #104](https://github.com/GuoMonth/dsh-multi-tenant/issues/104). Current source and released artifacts still use `Cell`; W1 plans a breaking rename to `AgentWorkspace`, removal of Process/Docker product backends, standalone launch, and snapshot/restore. Do not infer these changes are implemented or released.

## Ownership

An `AgentWorkspace` is the planned namespaced resource boundary for one agent instance and its persistent HOME/data. Kubernetes is the only product execution backend. A thin runtime-owned CRD/Operator may reconcile native StatefulSet, PVC, Service and policy resources; direct runtime management of those native resources is also valid. The platform/runtime boundary isolates user identity and product protocols from infrastructure mechanics; it does not require pushing StatefulSet/PVC ownership into the platform. Namespace is administrator-configured infrastructure scope, not OIDC tenant identity; the reference setup preconfigures per-user namespaces rather than creating an automatic namespace tenancy system. Platform authorization is the sole user-owner authority; runtime stores immutable owner linkage only for resource matching and does not implement OIDC.

| Concern | Owner |
| --- | --- |
| User identity, OIDC, membership, authorization and user sessions | multi-tenant platform |
| AgentWorkspace resource, image, lifecycle and verified internal transport | runtime repository (planned name; currently Cell) |
| Application protocols, sessions, tools and model calls | DSH |
| Pod scheduling, services, network policy and volumes | Kubernetes and its installed providers |
| TLS termination and ingress routing | Gateway API implementation |

The planned AgentWorkspace API will not accept a Pod name, UID, IP, Node, route or session as a user-selected target. Runtime resolves an instance from Kubernetes state and checks its identity before forwarding. **Current implementation fact:** the source uses a standard Kubernetes Pod boundary and `Cell`; current main accepts only `securityClass: standard`. Published npm `0.3.0-alpha.1` artifacts still describe their own older release and are not changed by this design text.

## Request path

```text
Browser → Gateway (TLS and routing) → multi-tenant platform (OIDC and authorization)
        → Connector → verified Cell Pod → DSH
```

Gateway provides TLS termination and routing in the integrated flow. OIDC login and user authorization belong to the platform. The Connector resolves the platform-authorized Cell reference, validates the current Kubernetes identity chain (including the Cell UID and Pod ownership), and proxies HTTP, WebSocket and streaming traffic to the selected Pod. The Go proxy keeps the DSH launch token in process memory and uses it for the local bootstrap exchange; it does not place it in a public URL or logs. DSH retains ownership of its application protocol and session behavior.

The network policy permits the platform Connector path to reach the Cell proxy port. The DSH listener remains loopback-only within the Pod. NetworkPolicy enforcement depends on a CNI that enforces the policy; a rendered policy alone is not proof of isolation.

## Identity and data boundaries

The target workspace identity is its Kubernetes namespace and immutable resource UID; same-name recreation with a new UID is a new instance. Current Connector code checks the Cell and owned Pod identity against live Kubernetes state and fails closed on stale or mismatched targets.

Data and private runtime state use separate PVCs. Stopping retains both; data may be retained on deletion, while private state is deleted only when the explicit deletion scope includes credentials. The W3 target puts HOME/XDG auth in private and DSH sessions/work files in data after verifying exact client paths. Current implementation only places the DSH-designated credential in private; HOME remains in data, so current code does not isolate all CLI credentials. The platform's OIDC secret is separate and must never be mounted into a workspace. Kubernetes objects and installed providers remain authoritative for resource state and enforcement.

Ordinary Pods do not protect against a compromised node, kernel, cluster administrator, or storage/network provider. This POC does not claim that boundary.

## Planned lifecycle and deferred work

W2 is planned to add explicit `Running`/`Stopped` state and healthy-node normal stop/start over an explicit StatefulSet and explicit data/private PVCs, retaining both PVC identities. It does not promise fencing under node partition; force deletion on an unhealthy or partitioned node is outside the stop guarantee. Stopping retains both volumes. Backups, hot pools and automatic idle are deferred. W3 requires joint validation of real authorization, persistent HOME and performance before release.

W1 also plans removal of the product Process/Docker execution backends and the old standalone launcher and snapshot/restore flows. This does not remove child processes inside the workspace Pod, OCI image builds, or Docker as kind's cluster substrate.

## Historical Cell implementation

Current source still includes CellSnapshot/restore and historical standalone files; this design update does not remove code or change release artifacts. Those paths are not part of the AgentWorkspace target. See [archived standalone alpha documentation](../archive/standalone-alpha1/README.md) for version-specific context.
