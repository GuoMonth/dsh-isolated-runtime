# DSH Isolated Runtime — Cell Alpha

`dsh-isolated-runtime@0.3.0-alpha.1` delivers the fixed Kubernetes Cell runtime
release manifest, CRD, RBAC and platform-mode Operator deployment. Node.js 22+.
The runtime images support Linux/amd64 and pin DSH **0.1.5-rc.2**.

## Existing Kubernetes cluster

Administrators prepare Kubernetes, enforced NetworkPolicy, storage, tenant
namespaces, Gateway API, DNS/TLS and private configuration. This package does
not create a local cluster or install OIDC. It only prints bundled metadata/YAML;
no kubectl, Docker or network access is used by its commands.

```sh
npx dsh-isolated-runtime@latest release
npx dsh-isolated-runtime@latest manifests > operator.yaml
# Review namespace/RBAC and replace the example base domain for your environment.
kubectl apply --server-side -f operator.yaml
kubectl -n dsh-system rollout status deployment/cell-operator --timeout=120s
```

`operator.yaml` pins the public Operator image by digest. Use the `release`
output's Cell image in the platform's tenant templates; do not substitute mutable
tags. The deployment uses `--access-mode=platform`, with no standalone authorizer
or direct Cell HTTPRoute. Applying to an existing installation can change shared
cluster resources: review the rendered YAML and kubectl context first.

The paired platform owns OIDC, members, sessions and user protocol access:

```sh
npx dsh-multi-tenant@0.9.0-alpha.1 start --config /private/config.json
```

The platform needs Node.js 24+ and API/Pod network access, usually inside K8s.
See the [platform setup](https://github.com/GuoMonth/dsh-multi-tenant/blob/main/docs/reference/quickstart.md)
for tenant templates, networking and private configuration.

## Breaking change from 0.2

The standalone Docker/local-cluster launcher is replaced. `start`, `up`, `stop`,
`uninstall`, host archives and macOS installation are not commands in this
version. Existing data is not modified by the npm CLI. No automatic migration,
historical compatibility, HA or recovery is promised. Historical standalone
users must explicitly select `dsh-isolated-runtime@0.2.0-alpha.1` and its
[versioned instructions](https://github.com/GuoMonth/dsh-isolated-runtime/tree/v0.2.0-alpha.1).

Alpha maturity is in the version name; npm uses `latest` and GitHub uses Release /
Latest. The package embeds immutable image identities, the runtime source SHA
and SHA-256 of its bundled deployment YAML. `--verify-release` checks that local
binding; it is not a signature or a claim of cluster readiness.
