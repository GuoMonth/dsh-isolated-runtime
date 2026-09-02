# dsh-isolated-runtime

Kubernetes-native isolation for [DeepSeek Harness (DSH)](https://github.com/deepseek-ai/deepseek-harness).
The project defines one durable boundary—`Cell`—and lets Kubernetes, Gateway
API, and CSI keep ownership of the infrastructure they already model.

**Current state: Phase 1 single-Cell vertical slice.** The operator translates a
Cell into native Kubernetes storage, identity, workload, service, and ingress
policy resources. The Cell image runs the access launcher as PID 1 and persists
DSH data across Pod replacement.

[中文](./README.zh-CN.md)

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
grow but not shrink, and `storageClassName` and `retentionPolicy` are immutable. The API does not
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
make verify-dsh         # exact upstream DSH compatibility suite
golangci-lint run
```

The supported DSH baseline is exactly `dsh-v0.1.2-alpha.4` at commit
`4e84901e6471b79ec0338099867ebb4606d12bb5`; it is not a semver range. The
[compatibility record](./compat/dsh/README.md) explains why the selected access
seam is a Cell-local launcher that owns the DSH child process.

Install the CRD, cluster-wide operator, and its `dsh-system` namespace with
`kubectl apply -k config/default`. Public routing and user authentication remain
Phase 2 work.

## Design

- [Architecture](./docs/specs/architecture.md)
- [Threat model](./docs/specs/threat-model.md)
- [Roadmap](./ROADMAP.md)
- [Contributing](./CONTRIBUTING.md)

Apache-2.0 licensed. No removed pre-Cell API or deployment carries a
compatibility promise.
