# dsh-isolated-runtime

Kubernetes-native isolation for [DeepSeek Harness (DSH)](https://github.com/deepseek-ai/deepseek-harness).
The project defines one durable boundary—`Cell`—and lets Kubernetes, Gateway
API, and CSI keep ownership of the infrastructure they already model.

**Current state: Phase 4 fleet operations are complete.** The same narrow Cell
and CellSnapshot APIs now converge across namespaces under native Kubernetes
quota and admission policy. Controllers use bounded workers, object watches,
deadline wakeups, and Kubernetes error backoff; optional metrics expose only
aggregate controller and closed authorization outcomes. There is still no
project fleet inventory, scheduler, namespace policy engine, or backup service.

[中文](./README.zh-CN.md)

MVP v0.1.0 is being prepared. Start with the [local Quickstart](docs/quickstart.md); release status and acceptance are tracked in [milestone 7](https://github.com/GuoMonth/dsh-isolated-runtime/milestone/7).

## Cell contract

```yaml
apiVersion: dsh.isolated.io/v1alpha1
kind: Cell
metadata:
  name: assistant
  namespace: tenant-alice
spec:
  image: ghcr.io/example/dsh-cell@sha256:0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef
  storage:
    size: 20Gi
```

The namespace is the tenant boundary. Images are digest-pinned, storage may
grow but not shrink, and `storageClassName`, `retentionPolicy`, and
`restoreFrom` are immutable. The API does not
expose sessions, Pod or Node addresses, `RuntimeClass`, revisions, scheduling,
checkpoints, profiles, or hostnames. See the complete
[sample](./config/samples/dsh_v1alpha1_cell.yaml) and
[generated CRD](./config/crd/bases/dsh.isolated.io_cells.yaml).

## Evidence

```bash
make verify             # formatting, generation, vet, race tests, build
make test-envtest        # controller reconciliation against a real API server
make verify-cell        # CRD behavior in a disposable kind cluster
make verify-images      # production images and real DSH persistence smoke test
make verify-kind        # complete Phase 1 vertical slice in kind
make verify-kind-phase2 # HTTPS/OIDC/RBAC browser proof with Envoy, Dex, Chromium
make verify-kind-phase3 # writer-stop/CSI restore/rollout/fresh rollback proof
make verify-kind-phase4 # 10-namespace/50-Cell quota, pressure and recovery proof
make verify-dsh         # exact upstream DSH compatibility suite
golangci-lint run
```

The supported DSH baseline is exactly `dsh-v0.1.3-alpha.1` at commit
`d347e703908d0406b7a7ef80e3a0e594d86b2215`; it is not a semver range. The
[compatibility record](./compat/dsh/README.md) explains why the selected access
seam is a Cell-local launcher that owns the DSH child process.

`kubectl apply -k config/default` installs the Phase 1 surface without any
Gateway API dependency. After installing Envoy Gateway and supplying wildcard
DNS/TLS plus OIDC provider settings, `config/browser` adds the reference Gateway,
authorizer, and routing mode. Administrators grant access with ordinary
RoleBindings; the operator intentionally never manages them. The Envoy Gateway
installation must use the adjacent `envoy-gateway.yaml` configuration so its
data plane runs in the Gateway namespace and the Backend extension is enabled.

After a CSI snapshot controller and compatible driver are installed,
`kubectl apply -k config/snapshots` enables `CellSnapshot`. The project does not
install production CSI components. See the executable
[snapshot/restore sample](./config/samples/dsh_v1alpha1_cellsnapshot.yaml).

`config/metrics` is an optional Kustomize component for browser/snapshot installs. It enables
bounded controller concurrency and private metrics listeners without creating
a metrics Service or scraper. Namespace labels, ResourceQuota, LimitRange,
StorageClass, RuntimeClass, Gateway route eligibility and CSI capabilities stay
administrator-owned; see the [namespace contract](./docs/specs/namespace-contract.md)
and [metrics contract](./docs/specs/metrics.md).

Release candidates are built once, tested by immutable Cell and Operator
digests across every gate, and only then promoted to `main`/`sha-*` GHCR tags.
The promoted manifests retain the candidate SBOM and provenance; promotion does
not rebuild them.

## Design

- [Architecture](./docs/specs/architecture.md)
- [Threat model](./docs/specs/threat-model.md)
- [Roadmap](./ROADMAP.md)
- [Contributing](./CONTRIBUTING.md)

Apache-2.0 licensed. No removed pre-Cell API or deployment carries a
compatibility promise.
