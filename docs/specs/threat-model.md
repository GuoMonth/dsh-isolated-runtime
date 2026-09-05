# Threat model

## Security claim

The project prevents one authorized Cell user from being routed to, mounting,
or reading another Cell's resources when Kubernetes namespace/RBAC, Gateway
authorization, CSI, and NetworkPolicy enforce their documented boundaries.
`sandboxed` requests a cluster-defined stronger runtime; `standard` is an
ordinary container boundary and does not resist a compromised node, runtime,
kernel, storage driver, cluster administrator, or cloud control plane.

## Assumptions

- Namespaces, ServiceAccounts, RBAC, admission, and Secret delivery are trusted.
- Only labelled access Pods in the system namespace may reach a launcher
  Service; the reference deployment grants that label to Envoy. The management
  port has no Service and is not admitted by the Pod ingress policy. Envoy OIDC
  and `cell-authorizer` must run before the Cell route.
- Images resolve to the admitted digest. CSI enforces volume identity and access
  mode. The DSH version matches the compatibility record.
- DSH itself and its enabled plugins are inside the Cell trust boundary. Code
  execution can compromise that Cell.

## Threat matrix

| Threat | Required control / guarantee |
| --- | --- |
| Namespace confusion | Namespace is the only tenant key; no spoofable `tenant` field. Cross-namespace references are absent. |
| Over-broad RBAC or ServiceAccount | The cluster-wide operator is limited to the native resources it reconciles; Cell workload API tokens are never mounted. |
| Network bypass | The Cell ingress policy admits labelled access Pods only on the proxy port; management has no Service or Pod ingress allowance; DSH remains loopback. |
| Route or identity confusion | The authorizer rereads the metadata-selected HTTPRoute and Cell and compares owner, UID, hostname, authority, parent and backend before an uncached SAR. |
| Host/Origin forgery | Preserve external Host/Origin for DSH; reject untrusted authorities and cross-site requests; never synthesize identity headers. |
| Token/cookie disclosure | Launch tokens, identity headers, and the pinned Envoy OAuth access/ID/refresh/nonce/HMAC cookies are stripped before DSH and redacted from diagnostics; unrelated and DSH cookies remain. The launcher adds Secure/SameSite=Lax while DSH supplies HttpOnly. |
| Stale authorization | The authorizer does not cache SAR decisions; adding or removing a RoleBinding applies to the next HTTP or WebSocket request. |
| Authorization outage | Missing/invalid identity is 401, denied identity is 403, and OIDC/JWKS/Kubernetes/authorizer failures are 503; Envoy never fails open. |
| Provider credential disclosure | `credentialsRef` is same-namespace only; values enter environment, not Cell status/logs/data snapshots. Internal DSH credential/signing storage is separate from the data PVC. |
| PVC/snapshot disclosure | Namespace/RBAC and CSI identity gate access; snapshots contain data only; private/provider/signing state receives a fresh PVC or Secret; Retain is the safe default. |
| Image replacement | Cell requires `name@sha256:<digest>`; admission and status compare the resolved digest. |
| Concurrent writes | A UID/CAS-owned Cell annotation serializes snapshots; observed zero StatefulSet replicas plus absence of Pods owned by that exact StatefulSet UID fences the writer before CSI snapshot creation. |
| Partial snapshot failure | The owned Kubernetes VolumeSnapshot is deleted before the source resumes. `CleanupBlocked` describes only API-visible object deletion; backend handling remains CSI policy. |
| Restore confusion | Restore is same-namespace, exact-image, exact-DSH, same-StorageClass and fresh-Cell only. PVC provenance and UID finalizers enforce the recorded image through its first Ready reader and protect concurrent deletion. |
| Stale or foreign format | Persisted data is tied to an exact DSH version; incompatible session formats fail closed. |
| Shutdown loss | Ordinary Pod termination is bounded, but exact DSH 0.1.3-alpha.1 exposes no distinguishable flush acknowledgement. Snapshots are explicitly crash-consistent after Kubernetes writer fencing. |
| Namespace policy bypass | The operator never owns or mirrors ResourceQuota, LimitRange, route-eligibility labels, StorageClass, RuntimeClass, PriorityClass, or APF policy. Native admission denial remains authoritative and recoverable. |
| Reconcile amplification | Worker pools are explicitly bounded; normal progress is watch-driven, errors use exponential rate limiting, and deadline/fallback wakeups are narrow. The reference scale gate verifies no steady-state reconcile churn. |
| Metric cardinality or identity leak | Metrics are disabled by default and not exposed through a Service. Project labels use only a closed decision enum; resource identity, topology, authority, user and secret data remain in Kubernetes or are omitted. |

## Residual risk and verification

The kind browser proofs cover HTTPS/OIDC, route binding, cross-Cell denial,
grant and immediate revocation, failure closure, NetworkPolicy, writer-stop
fencing, CSI data-only restore, exact-version rollout and fresh-Cell rollback. They do
not claim production capacity, latency or availability SLOs from the bounded
10-namespace/50-Cell reference proof. They also do not claim to operate DNS,
certificate, IdP or backup lifecycles, provide a WAF,
or resist a compromised cluster administrator, Gateway, node, runtime, kernel,
CSI driver, or cloud control plane.
