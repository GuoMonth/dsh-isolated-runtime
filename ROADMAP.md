[简体中文](./ROADMAP.zh-CN.md) | English

# Roadmap

Statuses: ✅ done · 🚧 next (settled) · ⏳ deferred (decision-gated).

The initial milestone is **M0 — bootstrapping**: stand up the repository and
frame the four components that form the isolated-runtime control plane — in one
place, rather than prematurely split.

## Next (settled)

- 🚧 **M1 — Scheduler.** Place each tenant session on a dedicated runtime (Pod)
  with resource and security-policy constraints; expose the placement decision
  as a stable interface so a non-Kubernetes backend can implement it later.
- 🚧 **M2 — Gateway.** The admission point that routes a session to its isolated
  runtime; the single place where "which tenant, which runtime" is decided.
- 🚧 **M3 — checkpoint/restore.** Capture and resume a session's runtime state
  across pod rescheduling, so isolation does not cost session continuity.
- 🚧 **M4 — runtime images.** Standard runtime images and profiles per workload
  (Terminal, Python, Node, …) with a versioned profile contract.

## Deferred (decision-gated)

- ⏳ **Runtime-agnostic backend.** Prove the control plane against a second
  container-runtime interface (Docker / Firecracker / Nomad) before generalizing.
- ⏳ **Public-contract freeze.** Names and surfaces are provisional until M1–M4
  settle the component boundaries.

## Milestones

- **M0 — Bootstrapping** 🚧 repository, bilingual docs, initial scope framing.
- **M1 — Scheduler.**
- **M2 — Gateway.**
- **M3 — checkpoint/restore.**
- **M4 — runtime images.**
- **M5 — End-to-end isolation suite** — the executable proof that a tenant cannot
  escape its runtime boundary (network, filesystem, host).

Each milestone is gated by its predecessor's decision; deferred items are pulled
forward only when their gate (a decision or an upstream seam) closes.
