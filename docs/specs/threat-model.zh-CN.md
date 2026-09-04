# 威胁模型

## 安全承诺

当 Kubernetes namespace/RBAC、Gateway 授权、CSI 与 NetworkPolicy 按各自契约工作时，
本项目阻止一个已授权 Cell 用户被路由到、挂载或读取其他 Cell 的资源。`sandboxed` 请求由
集群定义的更强 runtime；`standard` 只是普通容器边界，不能抵御已失陷的 Node、runtime、
kernel、存储驱动、集群管理员或云控制面。

## 信任假设

- 信任 namespace、ServiceAccount、RBAC、admission 与 Secret 投递；
- 只有 system namespace 内带标签的 access Pod 可以访问 launcher Service；参考部署只将
  该标签授予 Envoy。只有带 Operator 标签的 Pod 可以直连没有 Service 的 management port。
  Envoy OIDC 与 `cell-authorizer` 必须位于 Cell route 之前；
- 镜像解析到准入的 digest，CSI 保证卷 identity/access mode，DSH 版本匹配兼容记录；
- DSH 及启用的插件位于 Cell 信任边界内部；任意代码执行可以攻陷该 Cell。

## 威胁矩阵

| 威胁 | 必需控制 / 保证 |
| --- | --- |
| Namespace 混淆 | namespace 是唯一 tenant key；没有可伪造的 `tenant` 字段，也没有跨 namespace 引用。 |
| 过宽 RBAC / ServiceAccount | cluster-wide Operator 只获得其 reconcile 原生资源所需权限；Cell workload 永不挂载 API token。 |
| 网络绕过 | Cell ingress policy 只允许带 access 标签的 Pod 访问 proxy port、Operator 访问 management port；management 没有 Service；DSH 保持 loopback。 |
| 路由 / 身份混淆 | authorizer 重新读取 metadata 选中的 HTTPRoute 与 Cell，并在不缓存 SAR 前校验 owner、UID、hostname、authority、parent 与 backend。 |
| Host/Origin 伪造 | 保留外部 Host/Origin 给 DSH 校验；拒绝非信任 authority 与跨站请求；不合成身份 header。 |
| Token/cookie 泄漏 | launch/OIDC token 在进入 DSH 前剥离且不进入诊断；外部 URL 干净；DSH cookie 为 HttpOnly、Secure、SameSite=Lax。 |
| 过期授权 | authorizer 不缓存 SAR；RoleBinding 增删在下一个 HTTP/WebSocket 请求生效。 |
| 授权故障 | 身份缺失/无效返回 401，授权拒绝返回 403，OIDC/JWKS/Kubernetes/authorizer 故障返回 503；Envoy 不 fail open。 |
| Provider 凭据泄漏 | `credentialsRef` 只能同 namespace；值只进环境变量，不进 Cell status、日志或数据快照；DSH 内部 credential/signing store 与 data PVC 分离。 |
| PVC/snapshot 泄漏 | namespace/RBAC 与 CSI identity 控制访问；snapshot 只含数据；private/provider/signing 状态使用新的 PVC 或 Secret；默认 Retain。 |
| 镜像替换 | Cell 强制 `name@sha256:<digest>`；admission 与 status 对比解析后的 digest。 |
| 并发写 | Cell 上由 CAS 获得的注解串行化 snapshot；DSH 必须正常退出且 StatefulSet 观测为零副本，才能创建 CSI snapshot。 |
| 快照部分失败 | 源 Cell 恢复前必须删除未完成 VolumeSnapshot；无法确认删除时以 `CleanupBlocked` 保持零副本。 |
| Restore 混淆 | Restore 只允许同 namespace、精确 image、精确 DSH、同 StorageClass 与 fresh Cell；非法输入不创建 data PVC 或 workload。 |
| 过期或外来格式 | 持久数据绑定精确 DSH 版本；不兼容 session format fail closed。 |
| 关闭丢数据 | PID 1 launcher 先 drain 入口，再发 SIGTERM 等 DSH flush，只在有界超时后 kill。 |

## 剩余风险与验证

kind 浏览器实证覆盖 HTTPS/OIDC、route 绑定、跨 Cell 拒绝、授权与即时撤权、故障关闭、
NetworkPolicy、有界 DSH quiesce、CSI data-only restore、精确 RC rollout 与 fresh-Cell
rollback。本项目不负责 DNS、证书、IdP 或备份 lifecycle，不提供 WAF，也不承诺抵御已失陷
的集群管理员、Gateway、Node、runtime、kernel、CSI driver 或云控制面。
