# OIDC + Agent Workspace integration MVP direction

Updated 2026-09-22 under the [project constitution](../CONSTITUTION.md). The accepted Cell/Operator pair remains public as v0.3.0-alpha.1 (Linux/amd64). Current `main` also contains fixed-template `cell-mvp-v1`, which passed the 2026-09-22 internal candidate run ([report](https://github.com/GuoMonth/dsh-multi-tenant/blob/main/docs/evidence/cell-mvp-2026-09-22.md)). The source candidate has not been republished: public runtime npm `0.3.0-alpha.1` and its images still describe the previous release. Deploy the candidate only using the [platform candidate guide](https://github.com/GuoMonth/dsh-multi-tenant/blob/main/docs/reference/cell-mvp-v1-candidate.md); the published npm manifests below are not for the new template. Enterprise self-hosting is a direction, not a production-readiness claim.

## Direction and current implementation

The product target is Kubernetes-only Agent Workspace. The planned `AgentWorkspace` kind will replace `Cell` in W1 as a breaking change; current source and released packages still implement/use `Cell`. The canonical target and phases are in the [platform Agent Workspace design](https://github.com/GuoMonth/dsh-multi-tenant/blob/main/docs/design/agent-workspace.zh-CN.md) and [Issue #104](https://github.com/GuoMonth/dsh-multi-tenant/issues/104).

W1 removes Process/Docker product backends, old standalone launch, and snapshot/restore. Pod child processes, OCI image builds, and Docker as kind's substrate remain. W2 plans healthy-node normal stop/start through an explicit StatefulSet and explicit data/private PVCs, retaining both PVC identities. Force deletion on an unhealthy/partitioned node is outside the guarantee; this does not promise node-partition fencing. W3 requires joint validation of real authorization, persistent HOME and performance before release. Backups, hot pools and automatic idle are deferred.

Namespaces are administrator-configured infrastructure scopes, not OIDC tenant IDs. The reference setup preconfigures per-user namespaces and does not create an automatic namespace tenancy system. Platform authorization is the only user-owner authority; runtime keeps immutable owner linkage for resource matching and does not implement OIDC. Runtime may manage native Kubernetes resources behind its contract; the platform is not required to operate StatefulSets/PVCs directly.

Data and private are separate PVCs. Stop retains both; data may be retained on deletion, while private is deleted only when the explicit deletion scope includes credentials. W3 targets HOME/XDG auth in private and DSH sessions/work files in data after exact client paths are verified. Current implementation puts only the DSH-designated credential in private; HOME remains in data, so this MVP does not claim all CLI credentials are isolated.

## Current MVP evidence and ownership

The 2026-09-22 internal candidate run tested two users signing in through multi-tenant OIDC, creating their current `Cell` resources and using native DSH; cross-user access was denied. The platform owns user protocols, identity/membership, authorization and sessions. This repository currently owns Cell resources, lifecycle and restricted application transport. DSH owns application protocols, sessions and tools.

Administrators configure the cluster, infrastructure-scope namespaces, CNI, storage, DNS/TLS and service permissions. Platform/runtime separation is an application boundary, not a multi-backend promise; runtime may manage native StatefulSet/PVC resources behind the internal contract.

## Minimal deployment and limits

- The fixed-version reference regression records one cluster, one platform replica, an OIDC provider and a pinned template. Use one explicit administrator-configured setup when validating that combination.
- Use the selected administrator-configured Kubernetes deployment for the current integration flow.
- Pin each source/DSH/image combination. Breaking API, configuration and state-format changes are allowed at any time, with no historical compatibility, upgrade, migration or seamless recovery promise.
- Fail fast on invalid configuration, permissions, templates or versions. Bound readiness waits. Errors include stage, redacted target, observed state, write outcome, retry/check advice and a correlation ID; never secrets.
- Timeouts are not cancellation; missing records are not proof of stopped execution. Inspect the original identity after unknown writes. Do not create a new key automatically or build permanent tombstones, unbounded retries or automatic repair.
- Published delivery uses the platform npm CLI with an administrator-configured cluster; this repository does not add a cluster installer. The `cell-mvp-v1` candidate is source-only until a new package and images are released, and its deployment instructions live in the platform candidate guide above.
- The source uses the standard Kubernetes Pod boundary: `securityClass` accepts only `standard`, and the controller rejects unsupported existing values. The published npm `0.3.0-alpha.1` package and its fixed deployment artifacts remain unchanged; this source change does not revise or republish them.

## Current acceptance

Record both source commits, image digests, exact DSH, CNI/storage/CPU architecture, commands and redacted results for the current combination only.

1. Two OIDC identities and their Cells: create/read and native HTTP/WS/stream/Fetch work; unauthenticated, cross-user and ingress-bypass requests fail.
2. Parent-session invalidation also invalidates derived environment sessions, closes existing connections and denies new ones. Logout/platform restart does not delete Cells; it does not promise to cancel DSH background tasks.
3. Duplicate create reuses the allocation while its resource exists. Unknown writes, version mismatches and permission failures yield AI-readable diagnostics. Old-UID deletion cannot affect a new instance.
4. Normal Pod recreation preserves current-version files/sessions. This is not historical upgrade, disaster recovery or node-partition fencing evidence.
5. One real model request and unique file write/read, with privately configured credentials. Deterministic fixtures remain regression evidence, not a substitute.
6. Administrator deletion/test-reset instructions distinguish data, private-state and external Secrets. Operate only on explicitly authorized resources; do not silently erase old data.

When cleanup cannot be proven, reject old-data reuse and hand off to an administrator. Missing control objects are not proof that a physical writer stopped. Do not expand this into a generic recovery/migration system.

## Published artifact boundary

Published artifacts describe their own versions and remain unchanged by these source documents. The current npm package and fixed manifests are documented in [distribution](distribution.md); older standalone behavior is retained in [the archive](archive/standalone-alpha1/README.md).

[中文](alpha-mvp.zh-CN.md)

Earlier release results remain in the [2026-09-20 integration regression](https://github.com/GuoMonth/dsh-multi-tenant/blob/4ba252765bccb41314c0bdc6b11bcf60cc0b33ef/docs/evidence/cell-regression-2026-09-20.md). Current source-candidate results and limits are in the [2026-09-22 Cell MVP report](https://github.com/GuoMonth/dsh-multi-tenant/blob/main/docs/evidence/cell-mvp-2026-09-22.md). Do not treat earlier R1–R6 deferred-check lists as current untested status.

Unsupported security classes deny readiness and integrated admission. This does not stop or delete existing workloads or revoke historical standalone routes; administrators must inspect and explicitly dispose of those resources. In-place upgrades from historical sandboxed deployments are not supported.
