# Operational metrics contract

Metrics are disabled by default. Set `--metrics-bind-address` on the operator or
authorizer to expose a Pod-local Prometheus endpoint. The project creates no
Service, scraper, storage, dashboard or alerting resource.

The operator exports dependency-owned controller-runtime, workqueue, Go and
process metrics. Their labels are controller or queue names and bounded result
classes. The authorizer additionally exports:

```text
dsh_authorizer_decisions_total{decision="..."}
```

The only decision values are `allow`, `unauthenticated`, `denied`,
`route_mismatch` and `dependency_error`. Unknown internal values collapse to
`dependency_error`; no error text becomes a label.

Cell, Snapshot, PVC, Pod and Route inventories are deliberately absent.
Kubernetes Conditions and Events remain the object-level diagnostic source;
cluster operators may use their existing kube-state-metrics installation.
Names, namespaces, UIDs, image digests, hosts, routes, IPs, OIDC claims,
credentials and Secret values are forbidden from project metric labels.

These custom metrics are a pre-release operational contract. Metrics owned by
controller-runtime follow that dependency's lifecycle.
