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
- In Phase 1 only labelled Pods in the operator namespace may reach the launcher
  Service. Phase 2 must authenticate users and authorize an exact namespace/Cell.
- Images resolve to the admitted digest. CSI enforces volume identity and access
  mode. The DSH version matches the compatibility record.
- DSH itself and its enabled plugins are inside the Cell trust boundary. Code
  execution can compromise that Cell.

## Threat matrix

| Threat | Required control / guarantee |
| --- | --- |
| Namespace confusion | Namespace is the only tenant key; no spoofable `tenant` field. Cross-namespace references are absent. |
| Over-broad RBAC or ServiceAccount | The cluster-wide operator is limited to the native resources it reconciles; Cell workload API tokens are never mounted. |
| Network bypass | The Cell ingress policy admits only labelled access Pods in the operator namespace on the proxy port; management is not served; DSH remains loopback. |
| Route or identity confusion | Phase 1 uses a derived Service authority. Phase 2 must bind an authenticated principal to the exact namespace/Cell before routing. |
| Host/Origin forgery | Preserve external Host/Origin for DSH; reject untrusted authorities and cross-site requests; never synthesize identity headers. |
| Token/cookie disclosure | Launch token stays in memory and is redacted; public URL is clean; cookie remains HttpOnly/SameSite and gains Secure on HTTPS. |
| Provider credential disclosure | `credentialsRef` is same-namespace only; values enter environment, not Cell status/logs/data snapshots. Internal DSH credential/signing storage is separate from the data PVC. |
| PVC/snapshot disclosure | Namespace/RBAC and CSI identity gate access; snapshots contain data only; Retain is the safe default. |
| Image replacement | Cell requires `name@sha256:<digest>`; admission and status compare the resolved digest. |
| Concurrent writes | One active writer per data volume; quiesce and flush before snapshot/restore or replacement. |
| Stale or foreign format | Persisted data is tied to an exact DSH version; incompatible session formats fail closed. |
| Shutdown loss | PID 1 launcher drains ingress, sends SIGTERM, waits for DSH flush, then kills only after a bounded timeout. |

## Residual risk and verification

Phase 1 proves generated RBAC, Pod security context, ownership, NetworkPolicy,
volume lifecycle, and restart persistence in kind. It deliberately does not
provide a public or authenticated boundary. Phase 2 must
prove the external authz and route binding end to end. Security regressions are
release blockers; milestone reviews record any conditional guarantees.
