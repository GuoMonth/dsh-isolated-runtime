# DSH Isolated Runtime — Cell Alpha

`dsh-isolated-runtime@0.3.0-alpha.1` is the published package described by this
README: it bundles its fixed runtime manifest, CRDs, RBAC and Operator YAML.
Node.js 22+. The runtime images support Linux/amd64 and pin DSH **0.1.5-rc.2**.
This tracked README may change ahead of a package release; a source edit does not
mean the public npm tarball or its `operator.yaml` / `release.json` changed.

## Existing Kubernetes cluster

Administrators prepare Kubernetes, enforced NetworkPolicy, storage, tenant
namespaces, Gateway API, DNS/TLS and private configuration. This package does
not create a local cluster or install OIDC. It only prints bundled metadata/YAML;
no kubectl, Docker or network access is used by its commands.

For the fixed platform-mode deployment, the platform Pod must be in the operator’s system namespace (default `dsh-system`) and carry `dsh.isolated.io/access: platform`; see the [platform setup guide](https://github.com/GuoMonth/dsh-multi-tenant/blob/main/docs/reference/quickstart.md). When selecting packages through `latest`, verify that the runtime release matches the platform’s `cell-release.json` before applying its manifests.

```sh
npx dsh-isolated-runtime@latest release
npx dsh-isolated-runtime@latest manifests > operator.yaml
# Review namespace/RBAC and replace the example base domain for your environment.
kubectl apply --server-side -f operator.yaml
kubectl -n dsh-system rollout status deployment/cell-operator --timeout=120s
```

`operator.yaml` pins the public Operator image by digest. Use the `release`
output's Cell image in the platform's tenant templates; do not substitute mutable
tags. The integrated platform owns OIDC and user authorization. Gateway provides TLS
and routing, and the runtime Connector validates the Cell/Pod UID ownership
chain before proxying. Applying to an existing installation can change shared
cluster resources: review the rendered YAML and kubectl context first.

The paired platform owns OIDC, members, sessions and user protocol access:

```sh
npx dsh-multi-tenant@0.9.0-alpha.1 start --config /private/config.json
```

The platform needs Node.js 24+ and API/Pod network access, usually inside K8s.
See the [platform setup](https://github.com/GuoMonth/dsh-multi-tenant/blob/main/docs/reference/quickstart.md)
for tenant templates, networking and private configuration.

## Source candidate status

The published package still contains the fixed CRD and manifests, including
legacy snapshot and RuntimeClass schema. Runtime PR #93 proposes a source-only
standard-Pod sandbox reduction; it is not included in this released package.
Do not infer published package contents from a newer source checkout.

## Breaking change from 0.2

The standalone Docker/local-cluster launcher is replaced. `start`, `up`, `stop`,
`uninstall`, host archives and macOS installation are not commands in this
version. Existing data is not modified by the npm CLI. No automatic migration from older
standalone data is provided. Historical standalone
users must explicitly select `dsh-isolated-runtime@0.2.0-alpha.1` and its
[versioned instructions](https://github.com/GuoMonth/dsh-isolated-runtime/tree/v0.2.0-alpha.1).

Alpha maturity is in the version name; npm uses `latest` and GitHub uses Release /
Latest. The package embeds immutable image identities, the runtime source SHA
and SHA-256 of its bundled deployment YAML. `--verify-release` checks that local
binding; it is not a signature or a claim of cluster readiness.
