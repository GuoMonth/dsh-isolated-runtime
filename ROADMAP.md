[简体中文](./ROADMAP.zh-CN.md) | English

# Roadmap

Statuses: ✅ done · 🚧 next · ⏳ deferred.

## Done

- ✅ **M0 — Bootstrapping.** Repository, bilingual docs, six-layer architecture,
  Go control-plane skeleton, CI.
- ✅ **M0.1 — Architecture alignment.** Make Runtime tenant ownership explicit;
  define runtime allocation as reuse/create rather than Node placement; move
  Gateway identity to trusted transport context; establish one logical
  persistence/restore authority; add reusable runtime contract tests; tighten
  isolation wording and Kubernetes adapter boundaries.

## Next

- 🚧 **M1 — Kubernetes Runtime backend + allocation.** Reconcile tenant-owned
  Runtime objects into real Pods, apply profile/resource/security constraints,
  and let kube-scheduler choose Nodes. Add inventory needed for safe same-tenant
  reuse.
- 🚧 **M2 — Gateway router/reverse proxy.** Wire real authentication and
  authorization, resolve tenant + conversation/session → Runtime, then proxy DSH
  HTTP/WebSocket/stream traffic without exposing runtime topology.
- 🚧 **M3 — Logical persistence + restore.** Persist DSH/session/workspace/
  artifact state and a versioned manifest to S3/MinIO-compatible object storage;
  restore into a fresh Runtime.
- 🚧 **M4 — Standard DSH runtime images.** Publish `base`, `data`, `dev`, and
  later specialized profiles with platform-generated `standard` / `sandboxed`
  security posture.

## Deferred

- ⏳ **Process/container-memory checkpointing.** CRIU/runc/containerd checkpoint
  is not required for the initial continuity model; revisit only with a proven
  workload need.
- ⏳ **Second backend / runtime-agnostic contract freeze.** Keep Kubernetes-first
  adapter boundaries clean, then extract a stable common contract only after
  Docker, Firecracker, Nomad, or another backend demonstrates it.
- ⏳ **Public-contract freeze.** Names and surfaces remain provisional through
  M1–M4.

## Milestones

- **M0** ✅ Bootstrapping.
- **M0.1** ✅ Architecture alignment.
- **M1** Kubernetes Runtime backend + allocation.
- **M2** Gateway router / reverse proxy.
- **M3** Logical persistence + restore.
- **M4** Standard DSH runtime images + profiles.
- **M5 — End-to-end isolation suite.** Prove cross-tenant isolation across
  runtime ownership, filesystem, network, credentials/service accounts, and
  routing. Host-compromise/hostile-kernel guarantees are explicitly tied to the
  selected security class and RuntimeClass rather than implied by every Pod.
