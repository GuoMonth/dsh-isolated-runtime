[简体中文](./README.zh-CN.md) | English

# dsh-isolated-runtime

Kubernetes-native isolated runtime for
[DeepSeek Harness](https://github.com/deepseek-ai/deepseek-harness) (DSH): one
tenant, one Pod, with strong isolation, resumable sessions, and pluggable
runtime images.

> **Phase: bootstrapping.** The repository and its docs are being initialized;
> the control-plane components (Scheduler, Gateway, checkpoint/restore, runtime
> images) are laid out but not yet implemented. See [ROADMAP.md](./ROADMAP.md).

## What this is

`dsh-isolated-runtime` is the **infrastructure-level** tenant-isolation layer
for DSH. Where [`dsh-multi-tenant`](https://github.com/GuoMonth/dsh-multi-tenant)
enforces isolation *inside* a shared runtime (application layer), this project
gives each tenant its **own runtime boundary** — a dedicated Pod / container
runtime — so that untrusted tenants (third-party plugins, Terminal sessions,
Python / Node code execution) cannot escape into their neighbours or the host.

The isolation boundary is the **runtime itself**, not a rule inside a shared
process. A tenant's code, plugins, and filesystem live in their own Pod with
their own network namespace, resource limits, and container-runtime policy. The
control plane (Scheduler, Gateway) decides *where* a session runs and *resumes*
it across restarts via checkpoint/restore.

## Choosing between the two

| Project | Mode | Isolation boundary | Use case |
| --- | --- | --- | --- |
| [`dsh-multi-tenant`](https://github.com/GuoMonth/dsh-multi-tenant) | Shared Runtime | DSH application layer | High resource utilization, trusted plugin ecosystem |
| **`dsh-isolated-runtime`** | Isolated Runtime | Pod / Container Runtime | Strong tenant isolation, plugins / Terminal / code execution |

> If you require **strong tenant isolation with a dedicated runtime boundary** —
> third-party plugins, Terminal sessions, Python / Node code execution — use
> **`dsh-isolated-runtime`**.
>
> If you only need application-level isolation on a shared DSH runtime — maximum
> resource utilization across trusted tenants — use
> [`dsh-multi-tenant`](https://github.com/GuoMonth/dsh-multi-tenant).

In one line:

- **`dsh-multi-tenant`** → want to save resources and share a runtime; follow the
  multi-tenant protocol.
- **`dsh-isolated-runtime`** → need strong tenant isolation; give each tenant its
  own runtime.

The two projects are complementary, not competing: **Shared Runtime** vs
**Isolated Runtime**, **application-enforced isolation** vs
**infrastructure-enforced isolation**.

## Guiding principles

- **Infrastructure-enforced isolation** — the boundary is a real runtime
  primitive (Pod / container), not a convention in shared code. A tenant cannot
  cross it by accident or by omission.
- **One tenant, one runtime** — no two tenants share a runtime; isolation does
  not depend on tenants following a protocol.
- **Runtime-agnostic control plane** — Scheduler and Gateway target a
  container-runtime interface, not Kubernetes alone; Docker, Firecracker, or
  Nomad may back it later.
- **Resumable sessions** — checkpoint/restore keeps a session's runtime state
  across pod rescheduling, so isolation does not cost session continuity.

## Components (initial scope)

| Component | Role |
| --- | --- |
| Scheduler | Places each tenant session on a dedicated runtime (Pod) with resource and security-policy constraints. |
| Gateway | The admission point that routes a session to its isolated runtime. |
| checkpoint/restore | Captures and resumes a session's runtime state across rescheduling. |
| runtime images | Standard runtime images and profiles per workload (Terminal, Python, Node, …). |

See [ROADMAP.md](./ROADMAP.md) for milestones and status. See
[CONTRIBUTING.md](./CONTRIBUTING.md) for how development is done here. Full
documentation lives in [`docs/`](./docs/README.md).

## License

MIT
