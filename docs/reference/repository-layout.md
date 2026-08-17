[简体中文](./repository-layout.zh-CN.md) | English

# Repository layout

The six layers map onto the tree as follows. `api/` is the versioned public
contract; `pkg/` holds the layers and their backends; `cmd/` holds the three
processes.

```text
api/v1alpha1/          versioned API types (the contract)
cmd/
  gateway/            ④ admission — HTTP server, fail-closed
  scheduler/          ③ scheduling — placement process
  controller/         ⑥ control plane — lifecycle orchestrator
pkg/
  runtime/            ① isolation boundary — the runtime-agnostic seam
    kubernetes/       ① first backend (in-memory stub) + contract test
  provisioning/       ② runtime images + profiles
  scheduling/         ③ placement interface + first-fit default
  gateway/            ④ admission logic
  checkpoint/         ⑤ snapshot/restore contract
  controlplane/       ⑥ lifecycle orchestration
config/
  crds/               CRD manifest (exemplar)
  rbac/               RBAC manifest
hack/                 local quality gates (verify.sh)
```

## Dependency direction

```text
api  ◀──  pkg/*  ◀──  cmd/*
```

- `api/` is a leaf — it imports nothing from `pkg/` or `cmd/`.
- `pkg/runtime` defines the seam the control plane depends on; backends
  (`pkg/runtime/kubernetes`) implement it.
- `pkg/scheduling` and `pkg/controlplane` import `api/` for the contract types.
- `cmd/` wires the packages into processes.

## The runtime seam (①)

The single most important boundary is `pkg/runtime.Runtime` — the
runtime-agnostic interface every backend implements. The control plane
(scheduler, gateway, control plane) depends on `runtime.Runtime` and nothing
Kubernetes-specific. The contract test any backend must pass lives at
`pkg/runtime/kubernetes/kube_test.go`.

## Status

All components are skeletons — interfaces locked, implementations stubbed in
memory. Real cluster integration lands at M1–M4; see [ROADMAP](../../ROADMAP.md).
