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

Delivered by this milestone:

- reconcile one Cell into two PVCs, ServiceAccount, StatefulSet, headless and
  access Services, ingress NetworkPolicy, and current-generation status;
- package the exact DSH npm artifact in a digest-pinned Cell image with the
  launcher as PID 1;
- prove ownership, rollout, admission, network access, retention, failure
  recovery, and durable DSH/browser state through Pod replacement in kind;
- publish linux/amd64 Cell and operator images with SBOMs.

The local, image, exact-upstream, and repeatable kind evidence passed. The
[Phase 1 retrospective](https://github.com/GuoMonth/dsh-isolated-runtime/issues/23)
recorded `GO`.

## Phase 2 — Trusted browser access

Delivered by this milestone:

- derive one HTTPRoute and access Role from each Cell UID without expanding the
  Cell API;
- terminate public HTTPS and OIDC at Envoy Gateway, then bind trusted route
  metadata and exact live Kubernetes objects to an uncached SAR;
- prove real Chromium access, cross-Cell isolation, immediate grant/revocation,
  route confusion rejection, 503 failure closure and restart persistence;
- retain the two-image publication surface by packaging the authorizer with the
  operator image.

Evidence and the milestone decision are recorded in
[the Phase 2 retrospective](https://github.com/GuoMonth/dsh-isolated-runtime/issues/28).

## Phase 3 — Data lifecycle

Define CSI snapshot/restore around the data PVC while explicitly excluding
provider credentials and DSH browser-signing state. Prove quiescing and reject
concurrent writers.

## Phase 4 — Fleet operations

Add policy, upgrades, observability, quotas, and multi-namespace operations.
Kubernetes remains the fleet scheduler and state reconciler; this project does
not grow a second cluster orchestrator.
