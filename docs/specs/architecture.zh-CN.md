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

CellSnapshot
  → StatefulSet replicas=0
  → 已观察零副本且不存在 owned Pod（writer-stop barrier）
  → CSI VolumeSnapshot（仅 tenant-data PVC）
  → 源 Cell replicas=1
  → fresh Cell data PVC 使用 VolumeSnapshot dataSource

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
4. 保留外部 Host/Origin 供 DSH 校验；HTTPS 出口补 `Secure` 并将 DSH cookie 规范化为
   `SameSite=Lax`；`HttpOnly` 由 DSH 自身提供；
5. 身份认证与 Cell 授权位于本 seam 之前；launcher 会剥离身份 header 以及固定 Envoy
   Gateway v1.9.1 OAuth2 filter 使用的全部凭据 cookie，包括每个 policy 派生的后缀名称，
   同时保留 DSH 与无关应用 cookie。

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
不承诺迁移外来 session format。创建快照前 controller 必须停止唯一 writer，并且绝不能让两个
Cell writer 并发读写同一个 data volume。

`CellSnapshot` 是不可变的一次性 Kubernetes 意图。Cell 上通过 resourceVersion CAS 获得的
UID 注解负责串行化数据操作。接受操作时先记录源 Cell UID 与 data PVC UID，再激活锁。
`Accepted=True` 后，Cell controller 立即将源 StatefulSet 降到零副本；只有观测到零副本，
并通过不经缓存的 namespace 全量检查确认当前 StatefulSet 所有的 Pod，以及任何携带精确
Cell name/UID 的 Pod 均已消失，才写入 `WriterStopped=True`。创建 CSI `VolumeSnapshot` 前还会再次校验 PVC UID 及两侧 CSI
class driver。精确 DSH RC 无法从退出码区分 dispose
成功、拒绝或超时，因此这里只承诺 writer-stopped crash consistency，不宣称应用 flush。
快照失败时先删除 owned Kubernetes snapshot 对象，再恢复源 Cell；后端数据删除/保留由 CSI
driver 和 VolumeSnapshotClass 策略决定。

Restore 总是创建新的 data PVC 与 Cell identity。输入必须是同 namespace、Ready 的 snapshot，
使用其记录的精确 image digest、唯一支持的 DSH RC、不小于 restoreSize 的容量及同一
StorageClass；private PVC 始终全新创建。data PVC 记录 snapshot UID、image digest 与 DSH
version；UID 绑定的 finalizer 会保护来源，直到记录的 image 成为首个 Ready reader。在此之前
不能换 digest，删除中的输入也不能创建不可变 PVC。CSI 已生成 Bound data PVC 后，该 PVC
记录的 provenance 与同一组 finalizer 构成持久屏障：后到的 snapshot 删除请求只能等待，
exact-image 首个 reader 继续启动。之后才可显式 rollout 到同 RC 的另一个 digest；rollback
是从旧 snapshot 创建另一个 fresh Cell，绝不是原地 PVC 降级。

## Fleet 运维

Fleet scale 只是同一个 namespaced 资源图的重复，而不是新的对象或控制面。Namespace 独立提供
capability：核心 Cell 资源只要求普通 namespaced workload 与 PVC admission；公网访问还要求
Gateway route eligibility；snapshot 要求 CSI class/API；sandboxing 要求配置好的 RuntimeClass。
Operator 不 list、watch、持有或解释 Namespace、ResourceQuota、LimitRange、PriorityClass 或
API Priority and Fairness 对象。

两个 reconciler 都有显式 worker 上限。正常进展由 Kubernetes 对象 watch 驱动，API 错误交给
controller-runtime rate limiter，只有真实 writer-stop / snapshot deadline 才安排精确唤醒。
唯一保留的一分钟 retry 用于 cluster-scoped StorageClass / VolumeSnapshotClass 变化；如果没有
项目私有全局 fan-out，就无法把这种变化安全映射到单个 namespaced request。稳定集群因此没有
Cell 轮询循环。

Metrics 默认关闭，也不创建 Service 或 scraper。开启时，Operator 只暴露 controller-runtime
process/work-queue 聚合，authorizer 只暴露以封闭 decision 枚举为 label 的 counter。Cell、
snapshot、namespace、user、hostname、UID、route、Pod、Node、address、provider 或 credential
均不得成为 metric label；Kubernetes 对象仍是唯一 inventory 与诊断权威。

## 非目标

- 替代 kube-scheduler、Gateway API、CSI 或集群 fleet manager；
- 在 Cell 中暴露 Pod、Node、Service、`RuntimeClass` 或 hostname 选择；
- 解释 DSH 应用协议；
- 承诺普通容器可以抵御宿主机失陷；
- 支持已删除的 pre-Cell API 或浮动 DSH 版本。
- 调度备份、跨集群复制 snapshot 或替代 CSI。
- 定义 Fleet CRD、namespace template、quota policy、自定义 scheduler、autoscaler、telemetry
  backend、SLO 产品或 topology inventory。
