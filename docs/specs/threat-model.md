# Cell threat model

## Scope

The integrated request path uses the multi-tenant platform for OIDC and user authorization. Gateway terminates TLS and routes traffic; it does not perform the platform's OIDC flow or Cell authorization. The Connector accepts the platform's authorized internal request and validates the target Cell/Pod identity before proxying.

The namespace and Cell UID define the tenant instance boundary. NetworkPolicy limits access to the Cell proxy port to the platform path, subject to actual CNI enforcement. DSH remains loopback-bound in the Pod. Tenant data and private runtime state are separate; provider credentials must stay out of tenant data, status and logs.

This is an application isolation boundary for ordinary Kubernetes Pods. It does not defend against a compromised node/kernel, cluster administrator, or trusted Kubernetes, CNI, CSI, Gateway or cloud control plane. DSH and its enabled plugins are inside the Cell trust boundary.

Keep the existing Pod baseline: non-root, no privilege escalation, dropped capabilities, RuntimeDefault seccomp, no mounted ServiceAccount token, host directories or Docker socket. The fixed template sets `DSH_PERMISSION_MODE=danger-full-access` for DSH's in-Pod tool permission layer: the product uses the ordinary Pod as its execution boundary, without nested tool sandboxing or approval; this does not grant permissions beyond the existing Pod boundary or alter its OS/container, filesystem, token or network controls. CPU/memory requests and limits are explicit in the fixed Cell template; cluster policy must not mutate them. Generated policies primarily cover Cell ingress; administrators configure and verify unnecessary control-plane/private-network egress restrictions in the reference deployment. Pod existence alone does not enforce those restrictions.

## Threats and controls

| Threat | Control / limit |
| --- | --- |
| Cross-tenant target confusion | Resolve targets from platform-authorized Cell references and validate live namespace, Cell UID and owned Pod identity before forwarding. |
| Stale or recreated Pod | Compare the current ownership/UID chain; fail closed when the selected instance no longer matches. |
| Direct network bypass | Keep DSH loopback-bound and allow proxy-port ingress only from the configured platform path. Enforcement requires a CNI that honors NetworkPolicy. |
| Credential disclosure | Keep launch tokens in process memory; redact diagnostics; supply provider secrets outside tenant data and status. |
| Concurrent or stale data access | Kubernetes volume ownership and the current Cell lifecycle define access. Historical snapshot/restore mechanisms are not a current MVP acceptance promise. |
| Compromised workload | Treat DSH and enabled plugins as trusted within that Cell; ordinary Pod isolation does not contain host or kernel compromise. |

## Historical implementation and evidence

Earlier standalone releases used Envoy OAuth, a `cell-authorizer`, SubjectAccessReview and snapshot/restore. Those mechanisms are not the integrated platform request path. Some remain as historical source or published artifact behavior and have not been deleted by this documentation change. Version-specific history is in [the archived standalone documentation](../archive/standalone-alpha1/README.md).

The actual fixed-version integration evidence and its limits are recorded in the [shared regression report](https://github.com/GuoMonth/dsh-multi-tenant/blob/4ba252765bccb41314c0bdc6b11bcf60cc0b33ef/docs/evidence/cell-regression-2026-09-20.md). The current source uses the standard Pod boundary and rejects unsupported `securityClass` values; this does not change the published npm `0.3.0-alpha.1` package or its fixed artifacts.

Unsupported security classes deny readiness and integrated admission. This does not stop or delete existing workloads or revoke historical standalone routes; administrators must inspect and explicitly dispose of those resources. In-place upgrades from historical sandboxed deployments are not supported.
