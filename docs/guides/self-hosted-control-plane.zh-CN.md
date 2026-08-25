[English](./self-hosted-control-plane.md) | 简体中文

# 自托管控制面（M1）

M1 在 Kubernetes 之上提供一个单实例业务控制面。Kubernetes 继续负责 Node placement 与底层容器生命周期；本服务负责租户/Runtime 期望状态、Runtime → Pod reconcile、实时 Runtime inventory，以及刻意保持简单的 Admin UI/API。

## 安装

构建并推送镜像；如果使用其他镜像名，先修改 `config/deploy/deployment.yaml`，然后执行：

```sh
kubectl apply -k config
kubectl -n dsh-isolated-system rollout status deployment/dsh-isolated-control-plane
kubectl -n dsh-isolated-system port-forward service/dsh-isolated-control-plane 8080:8080
```

打开 `http://127.0.0.1:8080`，使用 M1 启动阶段固定账号登录：

```text
Admin / Admin
```

默认 Service 是 `ClusterIP`。M1 的账号密码是有意写死的，因此不要把这个后台直接暴露到公网。真正的身份系统、凭据管理和管理员角色留到后续里程碑。

## Runtime 流程

创建 Runtime 会写入 namespaced `Runtime` CR。Controller 轮询期望状态，并为每个 Runtime 实现一个受管理 Pod；Pod 状态会投影回 Runtime status 和 Admin UI。

`standard` 会应用 hardened 容器策略：non-root、禁止 privilege escalation、drop Linux capabilities、`RuntimeDefault` seccomp、不挂载 ServiceAccount token，以及可选资源限制。`sandboxed` 必须显式指定由部署环境提供的 Kubernetes `RuntimeClass`。

`networkIsolation` 开启时（默认开启），Controller 为 Runtime 创建独立 deny-all NetworkPolicy。M2 后续可以只放通 Gateway → Runtime 的必要流量，而不用削弱租户边界。

## Admin API

M1 API 使用 HTTP Basic authentication，账号仍然是同一套启动凭据。

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

## M1 边界

M1 不实现 DSH HTTP/WebSocket/stream reverse proxy、持久化 checkpoint、标准 DSH runtime images、多管理员身份、HA/leader election 或第二 Runtime backend；这些继续留在 M2+。
