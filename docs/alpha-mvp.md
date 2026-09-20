# OIDC + Cell integration MVP

Updated 2026-09-20 under the [project constitution](../CONSTITUTION.md). The core fixed-version integration passed regression; the accepted Cell/Operator pair is public as v0.3.0-alpha.1 (Linux/amd64). Enterprise self-hosting is a direction, not a production-readiness claim.

## Goal and ownership

Two users sign in through multi-tenant OIDC, create their own Cells and use native DSH; cross-user access is denied. The platform owns user protocols, identity/membership, authorization and sessions. This repository owns Cells, resource lifecycle and restricted application transport. DSH owns application protocols, sessions and tools.

Administrators configure the cluster, namespaces, CNI, storage, DNS/TLS and service permissions. The platform consumes a neutral internal interface, not a second Pod/PVC controller. Formal multi-backend compatibility waits for a second real requirement.

## Minimal deployment and limits

- One cluster with Linux worker nodes, one platform replica, one OIDC provider and a pinned template. Validate one explicit reference setup; kind is sufficient for the first flow.
- Reuse existing Kustomize/test deployment and Gateway capabilities. Helm, two-cluster installation acceptance and broad infrastructure matrices do not block the first integrated flow.
- Pin each source/DSH/image combination. Breaking API, configuration and state-format changes are allowed at any time, with no historical compatibility, upgrade, migration or seamless recovery promise.
- Fail fast on invalid configuration, permissions, templates or versions. Bound readiness waits. Errors include stage, redacted target, observed state, write outcome, retry/check advice and a correlation ID; never secrets.
- Timeouts are not cancellation; missing records are not proof of stopped execution. Inspect the original identity after unknown writes. Do not create a new key automatically or build permanent tombstones, unbounded retries or automatic repair.
- Defer HA, multi-cluster, autoscaling, disaster recovery, generic runtime services and new Process/Docker backends. Current delivery is the platform npm CLI for an administrator-configured cluster; no new cluster installer. Existing features are not compatibility obligations; keep ownership/isolation checks relevant to the current flow.

## Current acceptance

Record both source commits, image digests, exact DSH, CNI/storage/CPU architecture, commands and redacted results for the current combination only.

1. Two OIDC identities and their Cells: create/read and native HTTP/WS/stream/Fetch work; unauthenticated, cross-user and ingress-bypass requests fail.
2. Parent-session invalidation also invalidates derived environment sessions, closes existing connections and denies new ones. Logout/platform restart does not delete Cells; it does not promise to cancel DSH background tasks.
3. Duplicate create reuses the allocation while its resource exists. Unknown writes, version mismatches and permission failures yield AI-readable diagnostics. Old-UID deletion cannot affect a new instance.
4. Normal Pod recreation preserves current-version files/sessions. This is not historical upgrade, disaster recovery or node-partition fencing evidence.
5. One real model request and unique file write/read, with privately configured credentials. Deterministic fixtures remain regression evidence, not a substitute.
6. Administrator deletion/test-reset instructions distinguish data, private-state and external Secrets. Operate only on explicitly authorized resources; do not silently erase old data.

When cleanup cannot be proven, reject old-data reuse and hand off to an administrator. Missing control objects are not proof that a physical writer stopped. Do not expand this into a generic recovery/migration system.

## Historical behavior and distribution

Published artifacts and the standalone OIDC/RBAC path describe their own versions; they do not promise compatibility with new code. This documentation changes policy and planning, not installed artifacts or existing environments. Chart, uninstall/reinstall and multi-environment installation goals may be scheduled for a real requirement; they are not implicit gates for this integration MVP.

Actual installation/release changes still require verification of their current artifacts and flow. Automatic CI remains Source standards only. Publication, deployment and data deletion follow existing user authorization.

[中文](alpha-mvp.zh-CN.md)

Actual results and remaining evidence limits: [2026-09-20 integration regression](https://github.com/GuoMonth/dsh-multi-tenant/blob/4ba252765bccb41314c0bdc6b11bcf60cc0b33ef/docs/evidence/cell-regression-2026-09-20.md). Do not treat earlier R1–R6 deferred-check lists as current untested status.
