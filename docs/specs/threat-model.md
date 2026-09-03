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
  Service; the reference deployment grants that label to Envoy. Envoy OIDC and
  `cell-authorizer` must run before the Cell route.
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
| Route or identity confusion | The authorizer rereads the metadata-selected HTTPRoute and Cell and compares owner, UID, hostname, authority, parent and backend before an uncached SAR. |
| Host/Origin forgery | Preserve external Host/Origin for DSH; reject untrusted authorities and cross-site requests; never synthesize identity headers. |
| Token/cookie disclosure | Launch and OIDC tokens are stripped before DSH and redacted from diagnostics; the public URL is clean; the DSH cookie is HttpOnly, Secure and SameSite=Lax. |
| Stale authorization | The authorizer does not cache SAR decisions; adding or removing a RoleBinding applies to the next HTTP or WebSocket request. |
| Authorization outage | Missing/invalid identity is 401, denied identity is 403, and OIDC/JWKS/Kubernetes/authorizer failures are 503; Envoy never fails open. |
| Provider credential disclosure | `credentialsRef` is same-namespace only; values enter environment, not Cell status/logs/data snapshots. Internal DSH credential/signing storage is separate from the data PVC. |
| PVC/snapshot disclosure | Namespace/RBAC and CSI identity gate access; snapshots contain data only; Retain is the safe default. |
| Image replacement | Cell requires `name@sha256:<digest>`; admission and status compare the resolved digest. |
| Concurrent writes | One active writer per data volume; quiesce and flush before snapshot/restore or replacement. |
| Stale or foreign format | Persisted data is tied to an exact DSH version; incompatible session formats fail closed. |
| Shutdown loss | PID 1 launcher drains ingress, sends SIGTERM, waits for DSH flush, then kills only after a bounded timeout. |

## Residual risk and verification

The kind browser proof covers HTTPS/OIDC, route binding, cross-Cell denial,
grant and immediate revocation, failure closure, NetworkPolicy, and durable DSH
state across Pod replacement. It does not claim to operate DNS, certificate or
IdP lifecycles, provide a WAF, or resist a compromised cluster administrator,
Gateway, node, runtime, kernel, CSI driver, or cloud control plane.
