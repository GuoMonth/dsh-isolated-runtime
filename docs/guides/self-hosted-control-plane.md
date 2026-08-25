[简体中文](./self-hosted-control-plane.zh-CN.md) | English

# Self-hosted control plane (M1)

M1 ships a single-instance business control plane on top of Kubernetes. Kubernetes remains responsible for node placement and low-level container lifecycle; this service owns tenant/runtime desired state, Runtime-to-Pod reconciliation, live runtime inventory, and a deliberately small Admin UI/API.

## Install

Build and push the image, update `config/deploy/deployment.yaml` if you use a different image name, then install:

```sh
kubectl apply -k config
kubectl -n dsh-isolated-system rollout status deployment/dsh-isolated-control-plane
kubectl -n dsh-isolated-system port-forward service/dsh-isolated-control-plane 8080:8080
```

Open `http://127.0.0.1:8080` and sign in with the M1 bootstrap credentials:

```text
Admin / Admin
```

The Service is `ClusterIP` by default. The bootstrap credentials are intentionally hard-coded for M1, so do not expose this UI directly to the public Internet. Real identity, credential management, and role-based administration are later work.

## Runtime flow

Creating a runtime writes a namespaced `Runtime` CR. The controller polls desired state and realizes exactly one managed Pod for each Runtime. Pod status is projected back into Runtime status and the Admin UI.

`standard` applies a hardened container posture: non-root, no privilege escalation, dropped Linux capabilities, `RuntimeDefault` seccomp, no mounted ServiceAccount token, and optional resource limits. `sandboxed` requires an explicit Kubernetes `RuntimeClass` supplied by the deployment environment.

When `networkIsolation` is enabled (the default), the controller creates a per-runtime deny-all NetworkPolicy. M2 can later add narrowly scoped Gateway-to-Runtime traffic without weakening the tenant boundary.

## Admin API

The M1 API uses HTTP Basic authentication with the same bootstrap credentials.

```sh
curl -u Admin:Admin http://127.0.0.1:8080/api/v1/runtimes

curl -u Admin:Admin \
  -H 'Content-Type: application/json' \
  -d '{"name":"demo","tenant":"tenant-a","image":"nginxinc/nginx-unprivileged:alpine","securityClass":"standard","networkIsolation":true}' \
  http://127.0.0.1:8080/api/v1/runtimes

curl -u Admin:Admin \
  'http://127.0.0.1:8080/api/v1/runtimes/demo?tenant=tenant-a'

curl -u Admin:Admin -X DELETE \
  'http://127.0.0.1:8080/api/v1/runtimes/demo?tenant=tenant-a'
```

## M1 boundary

M1 does not implement the DSH HTTP/WebSocket/stream reverse proxy, persistent checkpoints, standard DSH runtime images, multi-admin identity, HA/leader election, or a second runtime backend. Those remain M2+ concerns.
