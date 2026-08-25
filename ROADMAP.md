[简体中文](./ROADMAP.zh-CN.md) | English

# Roadmap

Statuses: ✅ done · 🚧 next · 🧭 planned · ⏳ deferred.

## Done

- ✅ **M0 — Bootstrapping.** Repository, bilingual docs, six-layer architecture,
  Go control-plane skeleton, CI.
- ✅ **M0.1 — Architecture alignment.** Make Runtime tenant ownership explicit;
  define runtime allocation as reuse/create rather than Node placement; move
  Gateway identity to trusted transport context; establish one logical
  persistence/restore authority; add reusable runtime contract tests; tighten
  isolation wording and Kubernetes adapter boundaries.
- ✅ **M1 — Self-hosted control plane + Kubernetes Runtime backend.** Persist
  tenant-owned Runtime desired state as namespaced CRs; reconcile each Runtime
  into one hardened Pod; maintain live inventory for safe same-tenant reuse;
  support `standard` / `sandboxed` security posture and deny-all Runtime
  NetworkPolicy; ship a single-instance Admin UI/API with M1 bootstrap
  `Admin / Admin`; provide Docker/Kustomize self-host packaging.

## Production-ready definition

**M1 is a self-hostable Developer Preview, not a production release.** The
project becomes production-ready only after M8 and its production acceptance
criteria are complete.

A production release must at minimum provide:

- no dependency on the public fixed `Admin / Admin` credential; administrator
  identity, credentials, and authorization must be securely configurable and
  rotatable;
- Runtime/session routing and logical state that survive control-plane restart
  and upgrades without losing tenant desired state;
- idempotent reconciliation, retry/backoff, termination cleanup, recovery, and
  safe multi-replica control-plane operation;
- structured logs, metrics, audit events, health/readiness, alerts, and minimum
  operational runbooks;
- versioned releases, secure image supply chain, CRD/API upgrade and rollback
  strategy;
- real Kubernetes E2E plus isolation, restart/upgrade, fault-injection, and basic
  load acceptance tests;
- explicit backup/restore boundaries so DSH logical state and object-storage
  data can be recovered and control-plane resources can be rebuilt.

## Next

- 🚧 **M2 — Runtime Gateway + Session Binding.** Turn Gateway from an admission
  skeleton into the actual data entrypoint: wire real authentication and
  authorization, persist tenant + conversation/session → Runtime bindings, and
  transparently proxy DSH HTTP/WebSocket/stream traffic without exposing Pod,
  Service, Namespace, Node, or other runtime topology. Runtime allocation remains
  same-tenant reuse/create only; kube-scheduler remains solely responsible for
  Node placement.

- 🧭 **M3 — Logical persistence + restore.** Persist DSH/session/workspace/
  artifact state and a versioned manifest to S3/MinIO-compatible object storage;
  delete a Runtime and restore into a fresh Pod. Define explicit snapshot/restore
  APIs, state-version compatibility, retries, idempotent restore, and data
  lifecycle rules.

- 🧭 **M4 — Standard DSH Runtime Images + Profiles.** Publish versioned `base`,
  `data`, `dev`, and later specialized images/profiles. Pin immutable image
  digests and let profiles generate resource, security, and RuntimeClass policy.
  Add image build automation, vulnerability scanning, SBOM, and minimum
  provenance as the production supply-chain baseline.

- 🧭 **M5 — End-to-end isolation and security suite.** Continuously prove
  cross-tenant isolation across Runtime ownership, filesystem, network,
  credentials/service accounts, Gateway routing, and persistence. Verify
  `standard` versus `sandboxed`, invalid profile/RuntimeClass rejection, default
  deny network policy, and turn the threat model into executable regression
  tests.

- 🧭 **M6 — Production control-plane hardening.** Remove fixed `Admin / Admin`
  from production paths: support administrator credentials from Kubernetes
  Secrets and establish a stable OIDC/external-IdP authentication boundary; add
  CSRF protection, secure-cookie behavior, API rate limits, and audit trails.
  Replace simple polling with reliable Kubernetes watch + resync semantics,
  finalizers, exponential backoff, conflict retry, startup recovery, and orphan
  cleanup. Support multiple control-plane replicas with leader election/leases
  for safe writes/reconciliation, and define version-skew expectations during
  upgrades.

- 🧭 **M7 — Observability and operations.** Ship structured logs, Prometheus
  metrics, Kubernetes Events, administrative audit records, dashboards/alerts;
  separate liveness from readiness; expose Runtime creation latency, failure
  rate, reconcile errors, Pod startup time, and active Runtime/session counts.
  Add capacity/quota policy, troubleshooting, credential rotation, object-store
  failure, and Kubernetes-API outage runbooks, then define initial SLOs.

- 🧭 **M8 — Release, upgrade, disaster recovery, and production acceptance.**
  Establish SemVer releases, immutable/signed images, SBOM and vulnerability
  gates; define CRD/API migrations, forward/backward compatibility, rolling
  upgrade, and rollback procedures; exercise control-plane rebuild and DSH state
  recovery. CI gains real/ephemeral Kubernetes E2E covering control-plane
  restart, Pod crash, temporary Kubernetes API failure, upgrade/rollback,
  cross-tenant attack regressions, and baseline concurrency/load tests.
  **Passing M8 is the gate for the first production-ready release.**

## Deferred

- ⏳ **Process/container-memory checkpointing.** CRIU/runc/containerd checkpoint
  is not required for the initial continuity model; revisit only with a proven
  workload need.
- ⏳ **Second backend / runtime-agnostic contract freeze.** Keep Kubernetes-first
  adapter boundaries clean, then extract a stable common contract only after
  Docker, Firecracker, Nomad, or another backend demonstrates it.
- ⏳ **Multi-cluster / multi-region control plane.** The first production-ready
  target is reliable operation in one Kubernetes cluster. Multi-cluster
  placement, cross-region DR, and federation wait for demonstrated scale needs.
- ⏳ **Complex enterprise IAM.** M6 provides securely configurable administrator
  identity and an external-IdP boundary. Fine-grained org/project/role models,
  SCIM, and similar enterprise IAM are not required for the first production
  release.

## Milestones

- **M0** ✅ Bootstrapping.
- **M0.1** ✅ Architecture alignment.
- **M1** ✅ Self-hosted control plane + Kubernetes Runtime backend — Developer Preview.
- **M2** 🚧 Runtime Gateway + Session Binding.
- **M3** 🧭 Logical persistence + restore.
- **M4** 🧭 Standard DSH Runtime Images + Profiles.
- **M5** 🧭 End-to-end isolation and security suite.
- **M6** 🧭 Production control-plane hardening.
- **M7** 🧭 Observability and operations.
- **M8** 🧭 Release, upgrade, disaster recovery, and production acceptance — Production Ready Gate.
