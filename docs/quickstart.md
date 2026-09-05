# Local MVP quickstart

The first MVP targets Linux x86_64 with Docker and a graphical desktop for the
browser. Reserve the reference envelope of 4 CPUs and 16 GiB memory; actual
requirements depend on your workload. Bash, curl, tar/xz, sha256sum, OpenSSL,
flock and Git are prerequisites. The demo privately downloads missing pinned
kind, kubectl, jq, Node and Chromium tools. No Go compiler is needed.

Download the linux-amd64 archive and SHA256SUMS from the project's GitHub Release,
run `sha256sum -c SHA256SUMS`, extract the archive and enter its directory. The
archive includes `release.json` binding every project image to its tested digest.

```sh
./demo up
./demo open
```

The first command creates a local kind cluster with Calico, Envoy Gateway, Dex,
storage and one DSH Cell. It prints the Cell address and dedicated kubeconfig.
The second opens an isolated Chromium profile with local name resolution and
test TLS handling. It does not modify system hosts or certificate trust.
Ports 18443 and 15556 must be free; listeners bind only to loopback. Test login:
`alice@example.com` / `password`. These credentials and the test identity server
are only for the local demonstration.

In DSH's own first-run dialog, enter your DeepSeek API key, select a model and
send a request such as “create hello.txt and read it back.” Model calls use your
account. The project supplies no model service or separate provider settings UI.
Keys configured in DSH live on the private volume; provider keys may alternatively
be injected through a same-namespace Secret referenced by `credentialsRef`.

State lives in `${XDG_STATE_HOME:-$HOME/.local/state}/dsh-isolated-runtime`.
Set `DSH_DEMO_HOME` to select another state directory. Repeating `up` retains the
Cell and reconnects browser forwarding. Closing Chromium retains data. Do not
change the demo release in place: export needed files before deleting a demo.

```sh
export KUBECONFIG="${XDG_STATE_HOME:-$HOME/.local/state}/dsh-isolated-runtime/kubeconfig"
kubectl -n tenant-demo get cells
kubectl -n tenant-demo describe cell assistant
kubectl -n tenant-demo get httproutes
```

Use Cell Conditions, the referenced native objects and Kubernetes Events for
diagnosis. The HTTPRoute contains the external hostname; Cell status deliberately
does not duplicate it. Missing prerequisites, occupied ports and startup failures
are reported by the demo; its private runtime directory retains diagnostics.

## Optional snapshots

Choose `./demo up --snapshots` on the first start. This adds the reference CSI
hostpath test driver and snapshot controller; the Cell uses that StorageClass.
The basic local-path volume cannot be changed to the CSI class in place.

Create a CellSnapshot using the sample, wait for `Ready`, then create a new Cell
with that snapshot as `storage.restoreFrom`, the recorded exact image digest,
the same StorageClass and sufficient capacity. Use `csi-hostpath-sc` and
`csi-hostpath-snapclass` in this local demonstration. Read the namespace-scoped
examples under `config/samples/` before applying them.

Snapshots stop the writer and provide crash consistency, not an acknowledged
application flush. The fresh Cell has new identity and private storage: authorize
its new access Role and configure its model credentials again. Snapshots remain
optional; production CSI and backup lifecycle are owned by the cluster operator.

## Cleanup and existing clusters

`./demo down` deletes this demo's cluster **and all its data**, including retained
PVCs inside that disposable cluster. Downloaded tools are retained for reuse.
It never switches or deletes another Kubernetes context.

For an existing cluster use `config/default` for core resources or
`config/browser` for the recommended authenticated access setup; `config/snapshots`
adds the snapshot capability. `config/metrics` is an optional Kustomize component
for browser/snapshot installations. Configure domain, TLS, OIDC and route eligibility
in your own overlay before applying. Kubernetes, Gateway and CSI remain external
prerequisites; the operator does not install or own those systems.

Only the exact current DSH baseline and linux/amd64 are supported. There is no
historical API or cross-version restore commitment. HA, production capacity,
multi-cluster operation and enterprise policy are outside this MVP.
