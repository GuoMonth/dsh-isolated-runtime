[简体中文](./README.zh-CN.md) | English

# dsh-isolated-runtime

Kubernetes-native isolated runtimes for
[DeepSeek Harness](https://github.com/deepseek-ai/deepseek-harness) (DSH):
tenant-owned runtime boundaries, resumable logical state, and pluggable runtime
profiles.

> **Phase: M1 self-hosted control plane.** The repository now ships a real
> Kubernetes Runtime backend plus a single-instance business control plane with
> Runtime reconciliation, live inventory, and a minimal Admin UI/API. See
> [ROADMAP.md](./ROADMAP.md).

## M1 quick start

Build/push the control-plane image (or point the Deployment at your own image),
then install the manifests:

```sh
kubectl apply -k config
kubectl -n dsh-isolated-system port-forward service/dsh-isolated-control-plane 8080:8080
```

Open `http://127.0.0.1:8080` and use the intentionally fixed M1 bootstrap login:

```text
Admin / Admin
```

The default Service is `ClusterIP`; do not expose the M1 bootstrap Admin UI
directly to the public Internet. See
[docs/guides/self-hosted-control-plane.md](./docs/guides/self-hosted-control-plane.md)
for installation, API examples, and the M1 boundary.

## What this is

`dsh-isolated-runtime` is the **infrastructure-enforced** tenant-isolation path
for DSH. Where
[`dsh-multi-tenant`](https://github.com/GuoMonth/dsh-multi-tenant) isolates
tenants inside a shared DSH runtime, this project gives each Runtime an explicit
tenant owner and realizes that Runtime as a dedicated Pod/container boundary.

The cardinality is intentionally:

> **1 Runtime → exactly 1 Tenant; 1 Tenant → 0..N Runtimes.**

A tenant may start different runtimes for general assistant work, data analysis,
development, or other profiles. A Runtime is never shared across tenants.

This is a stronger boundary than application-level multi-tenancy, but it is not
marketed as an absolute host-security guarantee. Standard containers share the
host kernel; hostile arbitrary-code workloads can select a stronger
`sandboxed` security class backed by an appropriate Kubernetes `RuntimeClass`.

## Choosing between the two

| Project | Mode | Isolation boundary | Use case |
| --- | --- | --- | --- |
| [`dsh-multi-tenant`](https://github.com/GuoMonth/dsh-multi-tenant) | Shared Runtime | DSH application layer | High resource utilization, trusted plugin ecosystem |
| **`dsh-isolated-runtime`** | Isolated Runtime | Dedicated runtime / Pod boundary | Stronger tenant isolation, plugins / Terminal / code execution |

The two projects are complementary: **Shared Runtime** vs **Isolated Runtime**,
**application-enforced isolation** vs **infrastructure-enforced isolation**.

## Guiding principles

- **Ownership is data, not documentation.** Runtime ownership is explicit in the
  API and runtime seam; tenant-aware lookup/delete fail closed for foreign
  tenants.
- **Own business control-plane semantics, not Kubernetes mechanics.** The M1
  service owns tenant/runtime desired state, inventory, reuse/create decisions,
  and Admin operations. Kubernetes remains responsible for Pod-to-Node
  scheduling and low-level infrastructure state.
- **Allocate runtimes; do not reimplement kube-scheduler.** This control plane
  decides reuse/create/profile/resources/security posture. Kubernetes decides
  which Node a Pod runs on.
- **Trusted identity comes from the transport boundary.** Gateway principals are
  established server-side by authentication, never accepted from an ordinary
  request body. The fixed M1 Admin identity is intentionally temporary and
  separate from the future Gateway identity model.
- **Logical state is durable; Pods are disposable.** Initial continuity is
  session/workspace/artifact state in object storage (S3/MinIO-compatible),
  restored into a fresh runtime. Kernel/process-memory checkpointing is not an
  M1 promise.
- **Kubernetes-native first.** Kubernetes-specific implementation belongs behind
  a clear adapter boundary. A runtime-agnostic public contract is promoted only
  after a second backend proves the common denominator.
- **Security classes, not arbitrary knobs.** Platform profiles select hardened
  `standard` or stronger `sandboxed` policies rather than asking callers to
  assemble low-level container security correctly.

## Components

| Component | Role |
| --- | --- |
| Runtime boundary | Tenant-owned Pod/container runtime; never cross-tenant shared. |
| Provisioning | Standard DSH runtime images + workload/security profiles. |
| Runtime allocation | Decide reuse vs create from live tenant Runtime inventory; never choose a Kubernetes Node. |
| Admin control plane | M1 self-hosted UI/API + Runtime desired state and reconciliation. |
| Gateway | Trusted authentication + tenant/session resolution; evolves into the DSH HTTP/WS/stream runtime router/reverse proxy in M2. |
| Persistence / restore | Logical DSH/session/workspace/artifact state to object storage and restore into a fresh Runtime. |

## Repository layout

```text
api/v1alpha1/          versioned API types
cmd/
  gateway/            trusted admission/router skeleton (M2)
  scheduler/          standalone allocation skeleton; M1 allocation is embedded
  controller/         self-hosted M1 control plane
pkg/
  runtime/            tenant-owned runtime seam
    kubernetes/       real Runtime CR + Pod backend and reconciler
    runtimetest/      reusable backend contract suite
  provisioning/       DSH runtime images + profiles
  scheduling/         runtime allocation (not Node scheduling)
  gateway/            trusted principal + admission
  checkpoint/         logical persistence/restore authority
  controlplane/       Admin UI/API + lifecycle orchestration
config/               CRD + RBAC + Deployment + Service + Kustomize install
```

See [docs/specs/architecture.md](./docs/specs/architecture.md) for the global
model and [ROADMAP.md](./ROADMAP.md) for sequencing.

## License

MIT
