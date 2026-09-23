# Bounded agent-sandbox lab

Test-only upstream selection experiment for [#100](https://github.com/GuoMonth/dsh-isolated-runtime/issues/100). It does not change the production Cell Connector or implement platform OIDC, EnvironmentBinding persistence, or a second reconciler. Results and limits: [local report](../../docs/evidence/agent-sandbox-local-2026-09-23.md).

Requires an administrator-approved disposable Kubernetes namespace, enforced NetworkPolicy, the `standard` local-path StorageClass, Node 24+, Python 3 and the repository Go toolchain. The namespace is deliberately fixed to `sandbox-spike`; it must be unused for a fresh run. Scripts delete only their synthetic fixtures/private PVC after stop verification, and retain the main data PVC. They do not reset an existing run or migrate old Cell data.

Install upstream core `v1.0.3` using its release `sandbox.yaml` (SHA256 `725fafdabe6aac202a89dc57f1cfe0e2e92f3164c8c2bd343fffca52f7039d96`). Pin the controller's Linux/amd64 image to `registry.k8s.io/agent-sandbox/agent-sandbox-controller@sha256:b2160ee08dd4f2285b382d4b5073948891adfacbc9808a4ba9cf247766243c8c`, explicitly disable extensions, and wait for its deployment. No fork or altered CRD is used. For kind with a single-platform Docker cache, `kind load docker-image` can fail on missing arm64 content; import a `docker save --platform linux/amd64` archive with `ctr -n k8s.io images import --platform linux/amd64 --digests` and tag the verified platform digest. This is local image preloading, not a custom controller build.

Set `KUBECONFIG` to the approved local cluster, `DSH_SPIKE_IMAGE` to the exact accepted DSH Cell image digest, and `SANDBOX_EVIDENCE` to an absolute task-owned directory outside the checkout. Its parent is the lab directory. Store private native cookies under that parent's mode-0700 `private/` directory; never publish it. Put `versions.json` with the actual `dshImage`, controller, DSH and cluster versions in the evidence directory. Reserve local port 30500 before forwarding.

From the repository root:

```sh
npm ci --ignore-scripts --prefix packages/cell-connector
npm run build --prefix packages/cell-connector
# Build these to <lab>/dshprobe and <lab>/ws-hold using the repository Go version:
go build -o /path/to/lab/dshprobe ./test/e2e/dshprobe
go build -o /path/to/lab/ws-hold ./test/agent-sandbox/ws-hold.go
python3 test/agent-sandbox/lab.py
python3 test/agent-sandbox/prepare-proxy.py
kubectl --kubeconfig "$KUBECONFIG" -n sandbox-spike port-forward pod/probe 30500:30500 --address 127.0.0.1
```

Keep that task-owned port-forward in its own terminal. Then run the compiled `dshprobe --connect 127.0.0.1:30500 --authority sandbox-spike.cells.test --state-file /path/to/lab/private/probe-state.json`, followed by:

```sh
python3 test/agent-sandbox/lifecycle.py
SANDBOX_SNAPSHOT="$SANDBOX_EVIDENCE/verified-snapshot.json" node test/agent-sandbox/verify-target.test.mjs
python3 test/agent-sandbox/faults.py
python3 test/agent-sandbox/deletion.py
```

`deletion.py` writes `replaced-pvc-snapshot.json`; passing that snapshot to `verifyTarget` must throw `StorageMismatch`. The optional final `stop-gates.py` runs with `SANDBOX_EVIDENCE` set to a fresh subdirectory: it allocates separate disposable claims, checks real CNI denial, denies the watch via impersonation, verifies stale start/delete races, recovers an exact-UID terminal event before cleanup, and deletes those synthetic claims.

A failed/expired watch is not stop proof. The normal stop path calls `require_terminal`, which rejects empty events, another Pod UID, or missing container termination. If any stop fails, leave it stopped and inspect the recorded evidence; do not bypass the guard to resume/delete volumes. Watch-cache recovery is bounded and may fail after compaction; it is not a recovery service or a production resume implementation.

The probe Pod has a namespace-scoped read-only service account solely to read runtime objects. The DSH Pod has no Kubernetes token. `proxy.mjs` reuses the existing `proxy.js` HTTP/WS transport and a test-only target verifier; it exposes only the bound origin and has a loopback administration socket for revocation. Stop the port-forward and remove the probe when finished. Keep the retained data claim until explicitly cleaning up its contents. Do not uninstall shared CRDs or alter unrelated Cell resources.
