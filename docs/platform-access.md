# Platform access: R1 development configuration

This configuration separates the launcher's public authority from runtime-owned
HTTPRoutes. It is the first implementation slice of [S1 #85](https://github.com/GuoMonth/dsh-multi-tenant/issues/85),
not a working OIDC/platform deployment or a validated cluster reference.

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

R2 adds the platform Connector and validates target identity on every admission.
R3 must prove real browser/cookie/stream behavior and network bypass rejection.
No claims about those checks follow from R1 configuration/unit tests.
