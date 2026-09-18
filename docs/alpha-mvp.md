# Self-hosted alpha MVP

Status: agreed product direction; implementation and acceptance are pending.
Enterprise self-hosting is the long-term goal, not a claim of current
production readiness. This scope supersedes local desktop installation as the
next development priority. It does not change the published v0.2.0-alpha.1
artifacts, the exact DSH baseline, or the Cell/CellSnapshot contracts.

[中文](alpha-mvp.zh-CN.md)

## Users and responsibilities

An organization deploys the runtime centrally and provides browser-accessible
DSH environments to its users. Installers and maintainers must have Kubernetes
cluster administration permissions. End users need an authorized identity,
not Kubernetes access. Organization developers can reproduce the core flow
locally on a cluster they prepare themselves.

Administrators own cluster lifecycle, networking, storage, DNS/TLS, the OIDC
provider, namespaces and access grants. This project owns its runtime
components and Cell reconciliation. DSH owns sessions, tools and model use.
Keep the existing namespace and resource ownership boundaries.

## Next milestone

- One cluster and one documented reference deployment topology.
- Prefer a versioned Helm chart with accepted, digest-pinned images as the
  installation entry. Helm is planned, not available in the current release.
  Define resource ownership and CRD installation/upgrade ordering before
  implementing the chart; Helm rollback is not a DSH data rollback.
- Use the existing Envoy Gateway access chain with explicit prerequisites.
  Do not promise arbitrary Ingress or Gateway implementation compatibility.
- Standard OIDC for individual identity, with one tested provider configuration
  and a documented stable identity-to-RBAC mapping. Administrators explicitly
  grant Cell access using RoleBindings; successful login alone grants no access.
  No new account service, organization directory sync or group provisioning.
- Administrators create Cells declaratively and distribute access URLs. A
  per-person Cell is a reference flow, not a new one-user/one-Cell API invariant.
  No management portal or self-service provisioning in this milestone.
- Require an explicit usable StorageClass and verify persistence through Pod
  recreation. Keep snapshots optional; do not expand them into a backup service.
- Provide one kind reference path using the same chart and images. Developers
  prepare and own the cluster; document test networking, storage, identity and
  access setup. Do not rebuild host tool, browser or cluster lifecycle management.
- Provide basic readiness/status, logs and troubleshooting. Installation success
  must not be described as proof that the user journey works.

Current existing-cluster installation remains Kustomize under config/default,
config/browser and config/snapshots. Keep it available while the Helm path is
implemented and validated. k3s and other configurations are not newly certified
by this decision; expand documented support only with evidence.

## Alpha boundaries

Defer production availability and capacity SLOs, HA work, automatic scaling,
multi-cluster management, comprehensive auditing, complete disaster recovery,
offline distribution, broad infrastructure compatibility and automatic data
migration. Existing capabilities and regression tests are retained; deferral
does not require removing implemented functionality.

The following remain mandatory even in alpha:

- Reject unauthenticated, unauthorized and cross-Cell access. Keep fail-closed
  authorization and state the required NetworkPolicy enforcement assumptions.
- State the ordinary-container security boundary honestly. This milestone does
  not certify hostile-code containment or an unverified sandbox runtime.
- Do not silently delete data. Document destructive operations and what survives
  runtime uninstall/reinstall. User Cells, tenant namespaces and business data
  should not be deleted as an incidental effect of runtime chart removal.
- Keep credentials out of logs and evidence, preserve artifact identity checks,
  and state unsupported upgrade/restore operations explicitly.
- Do not equate request-time revocation with termination of existing connections
  or running tasks, or local fixture success with production acceptance.

## Acceptance

Record versions, image digests, cluster prerequisites, commands and redacted
results. The next milestone is complete only when both a freshly prepared kind
reference environment and a representative existing cluster demonstrate:

1. Installation from the documented artifacts without project-specific manual
   code changes; actionable errors for unmet documented prerequisites.
2. Two OIDC identities and two Cells with explicit access grants: each identity
   can use its authorized environment, while unauthenticated and cross-Cell
   requests are rejected. Removing a grant denies the next authorized request.
3. A real model request and unique file write/read in DSH, with credentials
   entered privately. Deterministic model tests remain useful regression evidence
   but do not substitute for this recorded live-model check.
4. Pod recreation followed by access to the retained file and session.
5. A documented runtime uninstall/reinstall exercise on test-owned resources,
   with retained Cell data and successful reconciliation after reinstall.

Snapshot acceptance is separate when enabled. Record failures and not-run
checks honestly. Local kind evidence does not certify other CNI/CSI/IdP
combinations, production scale, or Mac Docker Desktop.

## Distribution transition

Stop expanding npm and host-managed local installation as product work. Preserve
published artifacts, version-specific recovery instructions and useful local
and kind regression tests. Do not remove or silently migrate existing state.
The published release's installation instructions still apply to that release;
this document does not introduce working Helm commands or a new release.

Automatic CI remains Source standards only. Run appropriate behavioral checks
locally for implementation changes and record evidence in the PR. Publishing
charts, images, npm packages or release tags still requires release authorization.
