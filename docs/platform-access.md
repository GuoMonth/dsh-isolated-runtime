# Platform access: integrated Cell alpha

This configuration separates the launcher's public authority from runtime-owned
HTTPRoutes. The fixed integration setup passed [S1 regression](https://github.com/GuoMonth/dsh-multi-tenant/blob/4ba252765bccb41314c0bdc6b11bcf60cc0b33ef/docs/evidence/cell-regression-2026-09-20.md); the overlay alone is not a complete platform deployment.

Use `--access-mode=platform --base-domain=<domain>` with no `--gateway-name`.
The operator gives each Cell `cell-<UID>.<domain>[:port]` as its authority but
creates neither a direct HTTPRoute nor a standalone access Role. The default
`standalone` mode retains the existing direct-route configuration for current
fixtures; this is not a compatibility or migration promise.

`config/platform` renders the operator/CRDs only:

```sh
kubectl kustomize config/platform
```

Before deployment, replace the example domain and pin the operator and Cell
images to the tested build digests. The inherited `main` operator image is a
placeholder, not evidence that the new flags are published. Use a fresh test
installation. Gateway API HTTPRoute discovery is required even in platform mode,
so inability to inspect direct routes fails startup rather than appearing safe.

The administrator owns TLS/Gateway routes **to the platform**, and the platform
owns authentication and forwarding. Only Pods labelled
`dsh.isolated.io/access=platform` in the configured `--system-namespace` may reach
Cell proxy ports through the generated NetworkPolicy. Keep that namespace and
its labels outside tenant control. CNI enforcement and other additive policies
must be checked in the actual cluster; generating a NetworkPolicy does not prove
network isolation. This overlay does not deploy the platform or change a CNI.

## Conflicts and state

Before mutating resources, the operator checks live API state for the Cell's
workload mode, standalone Role and HTTPRoutes in the Cell namespace referencing
its Service (including differently named routes). Existing workloads without a
platform marker are standalone; platform workloads carry an operator-owned
`dsh.isolated.io/access-mode=platform` annotation. Switching either way is
rejected. It does not roll workloads, delete routes or migrate data to change mode.

A conflict sets Cell Access/Ready false with `AccessModeConflict` and a diagnostic
pointing to a fresh Cell/manual inspection. Read/discovery failures remain errors,
not absence. Existing deployments are left intact, so a conflict is **not** a
claim that old direct access was revoked. Administrators still own arbitrary
cross-namespace routes, out-of-band writes and additional network policies;
this preflight is not a cluster-wide route admission controller.

The Connector validates target identity on every admission. Current browser, revocation and network-bypass results are in the [shared report](https://github.com/GuoMonth/dsh-multi-tenant/blob/4ba252765bccb41314c0bdc6b11bcf60cc0b33ef/docs/evidence/cell-regression-2026-09-20.md); local socket and fixture evidence remain explicitly distinguished. Public release and startup instructions are owned by the [platform delivery guide](https://github.com/GuoMonth/dsh-multi-tenant/blob/main/docs/reference/quickstart.md).
