# Production runtime local cluster checks

Run only as the dedicated cluster write owner. These scripts use real upstream core, the actual RC image and kube-apiserver. They are not installation/E2E acceptance.

Prerequisites: core v1.0.3, a local digest-imported RC image, Calico policy enforcement and a RWO StorageClass. Apply `config/runtime/cluster-role.yaml`; bind it to a service account in a dedicated platform test namespace. A probe Pod in that namespace needs label `app.kubernetes.io/name=dsh-platform`, mounted platform SA token, Node 24 and the RC image's `ws` module. It is not a user workload.

Build the Connector and transfer `dist/` contents to `/tmp/connector` in the probe; include a `package.json` with `type: module`. Use tar streaming to overwrite contents, not directory-copy nesting. Copy scripts into `/tmp/`.

- `lifecycle.mjs`: create, wait Ready, positive stop, new-factory start. Uses one external PVC and saves `/tmp/binding.json` in the probe. Set `ALLOCATION` to a fresh explicit fixture ID.
- `scenarios.mjs`: reads `/tmp/options.json` containing ordinary `EnvironmentRuntimeOptions` and in-Pod SA file paths. Checks real owner/UID mismatch, concurrent stop, retained-data delete and no reallocation. A temporary TLS proxy on loopback9443 forwards genuine kube-apiserver requests, drops a successful activation response, rejects the stop watch, and cancels the read after accepted start. Supply test-only `/tmp/fault.key` and `/tmp/fault.crt` with localhost SAN; the proxy does not fabricate resources. Unverified stop remains gated across a new factory. It saves a ready binding for access tests.
- `access.mjs`: uses that binding, native DSH cookie exchange/HTTP settings/session creation and native session-follow WebSocket through the production Connector. Grant cancellation closes the established WebSocket, further access fails, and explicit stop closes a second stream. `RESUME_SESSION=1` checks the saved native session after restart/rebuild. No credential values are logged.

Host-side fault checks delete/rebuild only the exact B fixture Pod, verify markers in all three durable directories, then explicitly stop before removing a disposable fixture PVC and creating a different UID under its name. Connector must reject both missing and replacement PVC. Never use force deletion or remove data when normal stop is unverified.

Test scopes are identified by allocation key, namespace, Sandbox/PVC/Pod UID and image digest in evidence. No public publication, real model call, external tool approval or complete platform OIDC/browser flow is claimed by these checks; those remain E's joint acceptance.

The committed host checks `rebuild.py`, `network.py` and `volume-fault.py` use `KUBECONFIG` and `EVIDENCE` (existing output directory). All target the named B platform probe and its saved exact binding. Run rebuild/network while that binding is Ready. The destructive volume check additionally requires the dedicated `b-volume-fault-1` allocation to be positively Stopped with no Pods. It deliberately deletes that disposable fixture's data to prove missing/replacement rejection; do not use a valuable environment. `action.mjs` provides explicit inspect/start/stop/race-start operations and negative volume assertions for these checks.

`background.py` exercises an explicit test background writer across stop/start. `aged-writer.py` holds the same real Pod until its item resourceVersion actually yields Kubernetes ERROR/410, then verifies normal stop using the current list RV and the original writer UID. It may wait up to ten minutes for actual cache expiry; it never fabricates an old item identity or mutates another task's resources.
