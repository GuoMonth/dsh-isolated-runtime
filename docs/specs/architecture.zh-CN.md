# 架构

## 唯一边界

一个 `Cell` 是一个 namespaced DSH 信任、执行与持久状态边界；namespace 就是 tenant
identity。Cell 不是缩小版 Kubernetes，而是一层翻译为原生资源的窄意图 API。

| 关注点 | 权威系统 |
| --- | --- |
| Fleet、placement、restart、rollout、quota、RBAC | Kubernetes |
| 外部监听与 HTTP 路由 | Gateway API |
| 持久卷与快照 | CSI |
| Session、附件、storage domain、应用协议 | DSH |
| Cell 到原生资源的翻译、access seam | 本项目 |

Cell API 不选择 Node，也不携带 session 状态、拓扑、路由或 workload 实现细节。Operator
根据 namespace、Cell identity、集群策略与观测状态派生这些资源。

## 目标资源图

```text
Cell
  └─ operator
      ├─ tenant-data PVC
      ├─ private-state PVC
      ├─ ServiceAccount（不挂载 workload API token）
      ├─ StatefulSet（1 replica）：launcher（PID 1）→ DSH 子进程
      ├─ Headless Service（StatefulSet 网络身份）
      ├─ ClusterIP Service（Cell 访问入口）
      ├─ NetworkPolicy
      ├─ Role（单个 Cell 的 `access` verb）
      └─ HTTPRoute（UID 派生 hostname）

Browser
  → Envoy Gateway（HTTPS + OIDC）
  → cell-authorizer（route 校验 + SubjectAccessReview）
  → HTTPRoute
  → launcher
  → DSH HTTP / WebSocket / stream / Fetch
```

该资源图使用 Kubernetes owner reference 与 reconcile，不引入项目私有 scheduler、runtime
inventory、checkpoint service 或影子 desired-state 数据库。

## Access seam

DSH 0.1.2 RC 在进程内生成 launch token，只在 loopback readiness URL 中打印一次，再将其交换
为 authority-bound browser cookie；它没有支持的 token 注入接口。因此正式方向选择同容器
launcher：

1. launcher 启动 DSH 子进程并捕获 readiness URL；
2. token 仅留在 launcher 内存，只用于内部首次根请求，不进入外部 URL、参数或日志；
3. HTTP、WebSocket、stream 与 Fetch 全部透明转发，不解析 Typert；
4. 保留外部 Host/Origin 供 DSH 校验；HTTPS 出口将 DSH cookie 规范化为 `Secure`、
   `HttpOnly` 与 `SameSite=Lax`，以完成外部 OIDC provider 返回后的安全顶层导航；
5. 身份认证与 Cell 授权位于本 seam 之前；launcher 只信任由 NetworkPolicy 限定的入口。

独立 sidecar 无法安全获得进程 token；直接暴露既违背 loopback-only CLI，也会泄漏 launch
URL；纯 Gateway 配置无法完成内存 token exchange 与 cookie rewrite。

## 可信浏览器访问

公网 hostname 与 route 由不可变 Cell UID 和集群 base domain 派生，不进入 Cell API。
Envoy 终止 TLS、完成 OIDC 登录后调用 `cell-authorizer`。authorizer 只信任 Envoy route
metadata，并重新读取精确 HTTPRoute 与 Cell，逐项校验 owner、UID、hostname、parent 与
backend，最后对 Cell `access` verb 发起不缓存的 SubjectAccessReview。RoleBinding 完全由
管理员持有，因此授权或撤权在下一个 HTTP/WebSocket 请求生效。缺少身份返回 401，路由错误
或 RBAC 拒绝返回 403，依赖故障 fail closed 为 503。

## 状态

data PVC 保存 workspace、session、附件与 DSH storage domain。DSH 的
`.credentials.yaml` 位于独立的 private-state PVC；provider key 通常由同 namespace
的 `credentialsRef` Secret 作为环境变量提供。数据快照不包含 provider 凭据或浏览器签名记录。

persistence format 与 `compat/dsh/baseline.json` 中的精确 DSH 版本绑定。Restore 是数据操作，
不承诺迁移外来 session format。创建快照前 controller 必须 quiesce DSH，并且绝不能让两个
Cell writer 并发读写同一个 data volume。

## 非目标

- 替代 kube-scheduler、Gateway API、CSI 或集群 fleet manager；
- 在 Cell 中暴露 Pod、Node、Service、`RuntimeClass` 或 hostname 选择；
- 解释 DSH 应用协议；
- 承诺普通容器可以抵御宿主机失陷；
- 支持已删除的 pre-Cell API 或浮动 DSH 版本。
