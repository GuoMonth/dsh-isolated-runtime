[简体中文](./architecture.zh-CN.md) | English

# Architecture — six layers

The project is an isolated-runtime control plane. Isolation is enforced at the
runtime boundary, while the layers above it own allocation, trusted routing,
logical persistence, and lifecycle orchestration.

## Core invariant

> **Every Runtime belongs to exactly one Tenant. A Tenant may own multiple
> Runtimes. No Runtime is shared across tenants.**

The invariant appears in API types, runtime backend contracts, allocation, and
end-to-end tests; it is not merely a README convention.

## Six layers

| # | Layer | Artifact | Responsibility |
| --- | --- | --- | --- |
| ① | **Isolation boundary** | tenant-owned Runtime / Pod | Dedicated filesystem/process/network boundary; `standard` or `sandboxed` security class. |
| ② | **Provisioning** | DSH runtime images + profiles | `base`, `data`, `dev`, later specialized profiles; platform-generated security posture. |
| ③ | **Runtime allocation** | allocator | Decide same-tenant reuse vs create and desired constraints. Never choose a Kubernetes Node. |
| ④ | **Trusted routing** | Gateway | Authenticate server-side, authorize tenant/session, resolve Runtime, proxy DSH HTTP/WS/stream traffic. |
| ⑤ | **Continuity** | logical persistence / restore | Persist DSH/session/workspace/artifact state + manifest to object storage; restore into a fresh Runtime. |
| ⑥ | **Control plane** | API / CRDs / controllers | Reconcile tenant-owned Runtime lifecycle across ①–⑤. |

## Request flow

```text
Browser / API client
  -> Gateway authentication
  -> trusted Principal in server context
  -> authorize target tenant + conversation/session
  -> resolve existing Runtime or request allocation
  -> create/reuse tenant-owned Runtime
  -> Kubernetes realizes Pod; kube-scheduler chooses Node
  -> proxy DSH HTTP / WebSocket / streaming
```

On suspend/resume:

```text
Runtime
  -> logical state manifest + workspace/session/artifacts
  -> S3 / MinIO-compatible object storage
  -> Runtime may be deleted
  -> fresh Runtime from pinned image/profile
  -> restore logical state
  -> DSH resume
```

## Kubernetes boundary

Kubernetes is the first implementation, not something the generic layers must
pretend does not exist. Kubernetes-specific API usage belongs behind
`pkg/runtime/kubernetes` and controller/provisioning adapters. The project does
**not** implement Node placement; it emits the Pod constraints and lets
kube-scheduler place the Pod.

A second backend is required before the runtime abstraction is considered
stable.

## Security classes

- **standard** — hardened ordinary container policy: non-root, no privilege
  escalation, capability drop, seccomp, resource limits, network isolation,
  service-account/token minimization.
- **sandboxed** — stronger RuntimeClass for workloads where arbitrary code should
  be treated as hostile. Exact backend is deployment-specific.

Dedicated Pods materially strengthen tenant isolation, but standard containers
still share the host kernel. Documentation and tests therefore scope host-level
claims to the selected security class rather than promising absolute host escape
prevention.

## Continuity ownership

`pkg/checkpoint` is the single persistence/restore authority. `pkg/runtime`
creates/gets/deletes isolation boundaries; it does not own a second
checkpoint/restore API.

CRIU/runc/container-memory checkpointing is deferred until a real workload
requires it.
