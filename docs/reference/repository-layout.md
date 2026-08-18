[简体中文](./repository-layout.zh-CN.md) | English

# Repository layout

```text
api/v1alpha1/          versioned API types
cmd/
  gateway/            trusted admission/router skeleton
  scheduler/          runtime-allocation process (name provisional)
  controller/         lifecycle orchestrator
pkg/
  runtime/            tenant-owned runtime contract
    kubernetes/       Kubernetes backend adapter
    runtimetest/      reusable backend contract suite
  provisioning/       DSH runtime images + profiles
  scheduling/         runtime allocation; never Node placement
  gateway/            trusted principal + admission
  checkpoint/         single logical persistence/restore authority
  controlplane/       lifecycle orchestration
config/
  crds/               Kubernetes CRD exemplars
  rbac/               controller RBAC
```

## Dependency and ownership rules

- `api/` stays independent from implementations.
- `pkg/runtime` owns the isolation-boundary contract and tenant-aware
  create/get/delete semantics.
- `pkg/runtime/kubernetes` is allowed to use real Kubernetes concepts; those
  details must not leak through unrelated packages.
- `pkg/scheduling` allocates **Runtimes**, not Nodes.
- `pkg/checkpoint` owns continuity; runtime backends do not define a competing
  checkpoint API.
- `pkg/runtime/runtimetest` is the reusable contract suite every backend runs.

Interfaces remain prerelease through M1–M4.
