# Kubernetes alpha baseline

The next administrator-managed alpha targets Kubernetes 1.36 and 1.37.
Version numbers alone do not establish compatibility: acceptance is limited to
the reference stacks exercised below, not arbitrary CNI, CSI or Gateway combinations.
This is an MVP regression boundary, not an enterprise availability or capacity promise.

## SDK and security boundaries

- `controller-runtime v0.25.1`, matching `k8s.io/* v0.37.0` and Go 1.27.1.
- `controller-gen v0.22.0`; generated Cell/CellSnapshot schemas are unchanged.
- `setup-envtest v0.25.1`, default API server 1.37.0.
- `kind v0.33.0`; default node 1.37.0, alternate node 1.36.4. Exact OCI
  digests are recorded in `hack/lib/kubernetes-test.sh`; other test versions fail closed.
- Use kubectl 1.37.0 for both reference versions (at most one minor skew).

Production Kubernetes operations use the Go clients, not shell kubectl.
Controller-runtime continues to own watches, queues and bounded workers.
Authorization reads, writer fencing and restore ownership checks retain their
direct API readers and concurrency protection. Experimental read-your-writes
cache consistency is not enabled. Events use `events.k8s.io/v1`; operator RBAC
also retains core Event permissions for leader-election recording.
CLI commands remain appropriate for administrator instructions and test orchestration.

## Infrastructure prerequisites

- A CNI that actually enforces NetworkPolicy, including cross-Cell denial. The
  reference is Calico 3.32.2; default kind networking alone is insufficient.
- A provisionable StorageClass with persistent volumes and the access modes
  required by Cells. Pod recreation must retain workspace and DSH state.
- Browser access requires the reference Envoy Gateway stack, Gateway API CRDs,
  wildcard DNS/TLS, OIDC issuer/client configuration and explicit Cell access RBAC.
  Kubernetes version does not supply these prerequisites.
- Snapshots are optional. Enabling them requires the snapshot CRDs/controller,
  a compatible CSI driver and VolumeSnapshotClass, and restore provisioning.
  The hostpath CSI fixture is test-only, not a production storage recommendation.
- Sandboxed RuntimeClass, namespace labels, quotas and admission policy remain
  administrator-owned. A nominally supported cluster can still lack required capabilities.

## Local verification

```sh
make verify lint vuln test-envtest
make test-envtest ENVTEST_K8S_VERSION=1.36.2
DSH_TEST_K8S_VERSION=1.37.0 make verify-cell verify-kind-phase4
DSH_TEST_K8S_VERSION=1.36.4 make verify-cell verify-kind-phase3
```

Phase 4 includes Phase 2 browser/authentication/isolation/persistence and Phase 3
snapshot/restore/rollout tests, plus the bounded fleet regression. Phase 3 on the
second minor repeats the browser and lifecycle proofs. Record exact commits,
results and missing checks in the PR; an unpassed version is not supported.
Envtest checks API/controller behavior only, not CNI, CSI or browser behavior.
Run the browser/lifecycle gates serially: they reserve fixed local forwarding
ports (including 18443 and 15556). A busy port is a prerequisite failure, not
evidence that the new test's forwarding process is ready.
The upstream CSI fixture directory named `kubernetes-1.34` is an immutable
fixture layout, not this project's declared Kubernetes support version.

Test registries use containerd `certs.d/hosts.toml` on test-owned nodes only.
No host daemon registry configuration or unrelated clusters are changed.
Automatic CI remains Source standards only; kind workflows stay manually triggered.

### Verified reference matrix

Local Linux amd64 regression completed on 2026-09-18; detailed commands and
failed-attempt diagnostics are recorded in [PR #81](https://github.com/GuoMonth/dsh-isolated-runtime/pull/81).

| Reference | CRD validation | Browser, OIDC/RBAC, isolation and persistence | Snapshot/restore lifecycle | Bounded fleet |
| --- | --- | --- | --- | --- |
| Kubernetes 1.37.0 / kind 0.33.0 | Passed | Passed | Passed | Passed |
| Kubernetes 1.36.4 / kind 0.33.0 | Passed | Passed | Passed | Not repeated |
| Envtest 1.37.0 and 1.36.2 | Passed | Not applicable | Controller/API tests only | Not applicable |

Both cluster runs use the same runtime images built from `1da9a84`. The 1.37
Phase 4 runner is `cb93d9b`; the 1.36 Phase 3 runner is `9c2bca8`, which adds
an explicit Gateway data-plane readiness wait before port forwarding. Production
Go code is unchanged between these runner commits. No Mac Docker Desktop or
live-model acceptance is implied.

The 1.37 fleet fixture converged 50 Cells in 59 seconds and 8 overlapping
snapshots in 59 seconds; operator peak working set was 39,325,696 bytes on a
32-CPU, 64,903,213,056-byte-memory host. These are regression observations, not
an SLO or a minimum hardware specification.

## Published local alpha

The existing v0.2.0-alpha.1 local installer retains its separate 1.34 image and
tool identity. This work does not rewrite published bundles, bypass archive
identity checks or claim that legacy path validates the new cluster baseline.
Pure installation and optional seed separation are tracked in issue #79.
No images, packages or releases are published by these checks.

References: [controller-runtime compatibility](https://github.com/kubernetes-sigs/controller-runtime),
[controller-tools v0.22.0](https://github.com/kubernetes-sigs/controller-tools/releases/tag/v0.22.0),
[kind v0.33.0 node digests](https://github.com/kubernetes-sigs/kind/releases/tag/v0.33.0),
[kind local registry](https://kind.sigs.k8s.io/docs/user/local-registry/).
