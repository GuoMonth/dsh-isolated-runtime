[English](./architecture.md) | 简体中文

# 架构 —— 六层

本项目是一个隔离 Runtime 控制平面。隔离在 Runtime 边界强制执行；边界之上的层负责
Runtime allocation、可信路由、逻辑持久化与生命周期编排。

## 核心不变量

> **每个 Runtime 恰好属于一个 Tenant；一个 Tenant 可以拥有多个 Runtime；
> 任何 Runtime 都不能跨租户共享。**

这个不变量必须同时出现在 API 类型、runtime backend 契约、allocation 与端到端测试中，
而不是只写在 README。

## 六层

| # | 层 | 工件 | 职责 |
| --- | --- | --- | --- |
| ① | **隔离边界** | tenant-owned Runtime / Pod | 独享文件系统/进程/网络边界；支持 `standard` / `sandboxed` 安全等级。 |
| ② | **供给** | DSH runtime images + profiles | `base`、`data`、`dev` 等 profile，由平台生成安全 posture。 |
| ③ | **Runtime allocation** | allocator | 决定同租户 reuse 还是 create，以及所需约束；永远不选择 Kubernetes Node。 |
| ④ | **可信路由** | Gateway | 服务端认证、tenant/session 授权、Runtime 解析，并代理 DSH HTTP/WS/stream。 |
| ⑤ | **连续性** | 逻辑持久化 / 恢复 | 将 DSH/session/workspace/artifact 状态 + manifest 持久化到对象存储，并恢复到新 Runtime。 |
| ⑥ | **控制平面** | API / CRDs / controllers | reconcile ①–⑤ 的 tenant-owned Runtime 生命周期。 |

## 请求流

```text
Browser / API client
  -> Gateway authentication
  -> 服务端 context 中的 trusted Principal
  -> 授权目标 tenant + conversation/session
  -> 解析已有 Runtime 或请求 allocation
  -> create/reuse tenant-owned Runtime
  -> Kubernetes 创建 Pod；Node 由 kube-scheduler 决定
  -> 代理 DSH HTTP / WebSocket / streaming
```

暂停/恢复：

```text
Runtime
  -> logical state manifest + workspace/session/artifacts
  -> S3 / MinIO 兼容对象存储
  -> Runtime 可删除
  -> 用固定 image/profile 启动全新 Runtime
  -> 恢复逻辑状态
  -> DSH resume
```

## Kubernetes 边界

Kubernetes 是第一个真实实现，不需要假装它不存在。Kubernetes API 细节应放在
`pkg/runtime/kubernetes` 以及 controller/provisioning adapter 后面。本项目**不实现 Node
placement**；只生成 Pod 所需约束，再交给 kube-scheduler。

只有第二 backend 真正实现后，runtime abstraction 才考虑稳定。

## 安全等级

- **standard** —— hardened 普通容器策略：non-root、禁止 privilege escalation、
  drop capabilities、seccomp、资源限制、网络隔离、最小 service-account/token。
- **sandboxed** —— 对任意代码执行按 hostile workload 处理的更强 RuntimeClass；
  具体 backend 由部署环境决定。

独享 Pod 会显著增强租户隔离，但普通容器仍共享宿主 kernel。因此宿主级安全承诺必须明确绑定
security class，而不是宣传成所有 Pod 都“绝对无法逃逸宿主机”。

## 连续性的权威

`pkg/checkpoint` 是 persistence/restore 的唯一权威。`pkg/runtime` 只负责隔离边界的
create/get/delete，不再持有第二套 checkpoint/restore API。

CRIU/runc/container 内存 checkpoint 延后到出现真实 workload 需求时再评估。
