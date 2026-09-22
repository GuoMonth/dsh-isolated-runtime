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

**Product stage: fast-iteration MVP.** The current goal is multi-tenant OIDC + Cell behind a neutral internal interface. Validate pinned versions; breaking changes are allowed without historical compatibility, upgrade or seamless recovery promises. Fail fast with AI-readable diagnostics. See the [project constitution](https://github.com/GuoMonth/dsh-isolated-runtime/blob/v0.2.0-alpha.1/CONSTITUTION.md) and [current MVP acceptance](https://github.com/GuoMonth/dsh-isolated-runtime/blob/v0.2.0-alpha.1/docs/alpha-mvp.md). Helm and a broad installation matrix do not block the first flow.
The next cluster baseline and infrastructure prerequisites are documented in
[Kubernetes alpha baseline](https://github.com/GuoMonth/dsh-isolated-runtime/blob/v0.2.0-alpha.1/docs/kubernetes-baseline.md).

**Published local installation alpha: v0.2.0-alpha.1.**
Start with the [Quickstart](https://github.com/GuoMonth/dsh-isolated-runtime/blob/v0.2.0-alpha.1/docs/quickstart.md), or give your AI assistant the
[AI installation runbook](https://github.com/GuoMonth/dsh-isolated-runtime/blob/v0.2.0-alpha.1/docs/ai/local-run.md). The npm launcher and direct
[GitHub Release](https://github.com/GuoMonth/dsh-isolated-runtime/releases)
download use the same immutable installation bundle and public GHCR images.
See [distribution and release gates](https://github.com/GuoMonth/dsh-isolated-runtime/blob/v0.2.0-alpha.1/docs/distribution.md).

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
[sample](https://github.com/GuoMonth/dsh-isolated-runtime/blob/v0.2.0-alpha.1/config/samples/dsh_v1alpha1_cell.yaml) and
[generated CRD](https://github.com/GuoMonth/dsh-isolated-runtime/blob/v0.2.0-alpha.1/config/crd/bases/dsh.isolated.io_cells.yaml).

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
make lint
```

The supported DSH baseline is exactly `dsh-v0.1.5-rc.2` at commit
`fb2c4b9e698e30edb738bca4cf0618587db7d203`; it is not a semver range. The
[compatibility record](https://github.com/GuoMonth/dsh-isolated-runtime/blob/v0.2.0-alpha.1/compat/dsh/README.md) explains why the selected access
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
[snapshot/restore sample](https://github.com/GuoMonth/dsh-isolated-runtime/blob/v0.2.0-alpha.1/config/samples/dsh_v1alpha1_cellsnapshot.yaml).

`config/metrics` is an optional Kustomize component for browser/snapshot installs. It enables
bounded controller concurrency and private metrics listeners without creating
a metrics Service or scraper. Namespace labels, ResourceQuota, LimitRange,
StorageClass, RuntimeClass, Gateway route eligibility and CSI capabilities stay
administrator-owned; see the [namespace contract](https://github.com/GuoMonth/dsh-isolated-runtime/blob/v0.2.0-alpha.1/docs/specs/namespace-contract.md)
and [metrics contract](https://github.com/GuoMonth/dsh-isolated-runtime/blob/v0.2.0-alpha.1/docs/specs/metrics.md).

Release candidates are built once, tested by immutable Cell and Operator
digests across every gate, and only then promoted to `main`/`sha-*` GHCR tags.
The promoted manifests retain the candidate SBOM and provenance; promotion does
not rebuild them.

## Design

- [Architecture](https://github.com/GuoMonth/dsh-isolated-runtime/blob/v0.2.0-alpha.1/docs/specs/architecture.md)
- [Threat model](https://github.com/GuoMonth/dsh-isolated-runtime/blob/v0.2.0-alpha.1/docs/specs/threat-model.md)
- [Roadmap](https://github.com/GuoMonth/dsh-isolated-runtime/blob/v0.2.0-alpha.1/ROADMAP.md)
- [Contributing](https://github.com/GuoMonth/dsh-isolated-runtime/blob/v0.2.0-alpha.1/CONTRIBUTING.md)

Apache-2.0 licensed. No removed pre-Cell API or deployment carries a
compatibility promise.
