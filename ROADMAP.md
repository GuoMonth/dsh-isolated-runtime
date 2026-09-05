# Roadmap

The roadmap advances in vertical milestones. Each major milestone ends with a
review gate before the next one starts.

## Phase 0 — Cell contract reset and DSH evidence

Complete in this delivery:

- one namespaced `Cell` API and reproducibly generated CRD;
- exact DSH source, state, protocol, and shutdown baseline, now advanced to `dsh-v0.1.3-alpha.1`;
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

Delivered by this milestone:

- introduce immutable `CellSnapshot` intent and creation-only
  `storage.restoreFrom` without creating a backup control plane;
- scale the StatefulSet to zero and snapshot only the data PVC through the
  stable CSI API after a Kubernetes writer-stop barrier;
- serialize operations per Cell, fail closed on cleanup ambiguity, and resume
  the source automatically only after snapshot success or proven cleanup;
- restore only into a fresh Cell with the exact recorded image digest and DSH
  RC, then prove same-RC digest rollout and fresh-Cell rollback;
- keep private/provider/browser-signing state outside the snapshot and give
  every restored Cell a new UID, hostname, private PVC and DSH cookie.

The reference kind fixture installs external-snapshotter and the CSI hostpath
test driver only for verification. Production CSI lifecycle remains owned by
the cluster administrator. Evidence and the `GO` decision are recorded in the
[Phase 3 retrospective](https://github.com/GuoMonth/dsh-isolated-runtime/issues/33).

## Phase 3.1 — Contract hardening

This corrective milestone replaces conclusions that were stronger than the
evidence:

- strip the pinned Envoy OAuth2 access, ID, refresh, nonce, expiry, and HMAC
  cookies at the launcher boundary while preserving DSH cookies;
- remove the unprovable application-quiesce protocol and state the exact
  writer-stopped, crash-consistent CSI guarantee;
- bind the source data PVC UID and close workload-lineage, stale-lock,
  create/adopt, EndpointSlice, restore
  deletion, and first-reader races with API-visible Kubernetes invariants;
- build release candidates once, run every required gate against those exact
  digests, then promote the same manifests with SBOM and provenance.

The [Phase 3.1 retrospective](https://github.com/GuoMonth/dsh-isolated-runtime/issues/44)
recorded `GO` from post-merge evidence and unblocked Phase 4.

## Phase 4 — Fleet operations

Delivered by this milestone:

- define a capability-based namespace conformance contract without reading,
  copying, or owning namespace policy;
- retain ResourceQuota, LimitRange, admission, APF, scheduling and garbage
  collection as native Kubernetes failure and recovery surfaces;
- replace steady polling with object watches, exact deadline wakeups and
  controller-runtime exponential error backoff, with explicitly bounded Cell
  and CellSnapshot worker pools;
- expose optional private Prometheus endpoints whose labels are limited to
  controller-runtime dimensions and a closed authorization-decision enum;
- prove convergence and recovery for 50 Cells in 10 namespaces, native quota
  and LimitRange denial, namespace recreation, controller restart and eight
  overlapping CSI operations on the documented reference runner.

Kubernetes remains the fleet scheduler and state reconciler. Phase 4 adds no
Fleet API, inventory database, policy engine, custom queue or second cluster
orchestrator. Evidence and the gate decision are recorded in the
[Phase 4 retrospective](https://github.com/GuoMonth/dsh-isolated-runtime/issues/39).

## MVP v0.1.0 — Core closure and release

In progress: [milestone 7](https://github.com/GuoMonth/dsh-isolated-runtime/milestone/7).
Latest exact DSH baseline (#50), complete user journey (#51), local demo (#52),
immutable release bundle (#53), live acceptance and final GO/publication (#54).
No fleet-platform expansion or historical compatibility.
