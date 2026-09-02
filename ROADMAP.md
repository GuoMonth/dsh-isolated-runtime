# Roadmap

The roadmap advances in vertical milestones. Each major milestone ends with a
review gate before the next one starts.

## Phase 0 — Cell contract reset and DSH evidence

Complete in this delivery:

- one namespaced `Cell` API and reproducibly generated CRD;
- exact DSH `dsh-v0.1.2-alpha.4` source, state, protocol, and shutdown baseline;
- executable Cell-local launcher experiment;
- kind contract tests, lightweight CI, and path-gated full DSH CI;
- removal of the old Runtime/control-plane architecture.

The gate is `GO` only when local, kind, and exact-upstream compatibility checks
all pass.

## Phase 1 — Single-Cell vertical slice

- Reconcile one Cell into PVC, workload, Service, policy, and status.
- Build a digest-pinned Cell image with the launcher as PID 1.
- Prove durable DSH state through Pod replacement in kind.
- Review the vertical slice and record a GO/CONDITIONAL GO decision.

## Phase 2 — Authenticated access

Integrate the external identity and authorization component with Gateway API.
Derive routing from cluster base domain plus Cell identity, preserve DSH
Host/Origin semantics, and keep the launcher reachable only from the approved
ingress path.

## Phase 3 — Data lifecycle

Define CSI snapshot/restore around the data PVC while explicitly excluding
provider credentials and DSH browser-signing state. Prove quiescing and reject
concurrent writers.

## Phase 4 — Fleet operations

Add policy, upgrades, observability, quotas, and multi-namespace operations.
Kubernetes remains the fleet scheduler and state reconciler; this project does
not grow a second cluster orchestrator.
