# 运维指标契约

指标默认关闭。为 Operator 或 authorizer 设置 `--metrics-bind-address` 后，才会开放 Pod-local
Prometheus endpoint。本项目不创建 Service、scraper、存储、dashboard 或 alerting 资源。

Operator 输出依赖自带的 controller-runtime、workqueue、Go 与 process 指标；其标签只包含
controller/queue 名称和有限结果类型。Authorizer 额外输出：

```text
dsh_authorizer_decisions_total{decision="..."}
```

decision 只允许 `allow`、`unauthenticated`、`denied`、`route_mismatch` 和
`dependency_error`。未知内部值归并为 `dependency_error`，错误文本永不成为标签。

指标刻意不维护 Cell、Snapshot、PVC、Pod 或 Route inventory。Kubernetes Conditions 与 Events
继续作为对象级诊断入口；集群管理员可复用已有 kube-state-metrics。项目指标标签禁止出现
名称、namespace、UID、image digest、host、route、IP、OIDC claim、凭据或 Secret 值。

自定义指标属于 pre-release 运维契约；controller-runtime 自带指标遵循该依赖的生命周期。
