# Platform access

The current integrated path is documented in the [multi-tenant setup guide](https://github.com/GuoMonth/dsh-multi-tenant/blob/main/docs/reference/quickstart.md). The multi-tenant platform owns OIDC, user authorization and sessions. Gateway terminates TLS and routes requests to the platform. The runtime Connector validates the authorized Cell reference and current Kubernetes Cell/Pod UID ownership before proxying to DSH.

The Connector's network path is restricted to the configured platform pods by NetworkPolicy. DSH listens on loopback inside the Cell Pod. Network isolation is only effective when the cluster CNI enforces NetworkPolicy; verify this in the actual cluster.

The public npm `0.3.0-alpha.1` package contains the fixed manifests described by its [`release.json`](../packages/cell-cli/release.json) and [`operator.yaml`](../packages/cell-cli/operator.yaml). Those published artifacts are immutable. Later source-only changes, including the standard-Pod sandbox candidate in [runtime PR #93](https://github.com/GuoMonth/dsh-isolated-runtime/pull/93), do not change the package or manifest until a new release is made.

## Historical platform-mode overlay

Earlier source and releases included a standalone Envoy OIDC / `cell-authorizer` path with HTTPRoute and SubjectAccessReview validation. This is not the current integrated request path. That code remains in the repository as historical implementation; this document does not claim it was removed. See the [archived standalone alpha docs](archive/standalone-alpha1/README.md).
