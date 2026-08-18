[English](./README.md) | 简体中文

# dsh-isolated-runtime

面向 [DeepSeek Harness](https://github.com/deepseek-ai/deepseek-harness)（DSH）的
Kubernetes 原生隔离运行时：显式的租户运行时所有权、可恢复的逻辑状态，以及可插拔的
运行时 profile。

> **阶段：启动中 / M0.1 架构对齐。** 仓库已经具备 Go 控制平面骨架。在真正接入 Pod
> 生命周期之前，先把租户所有权、准入、运行时分配、持久化与威胁模型契约对齐。
> 参见 [ROADMAP.md](./ROADMAP.zh-CN.md)。

## 这是什么

`dsh-isolated-runtime` 是 DSH 的**基础设施强制**租户隔离方案。与
[`dsh-multi-tenant`](https://github.com/GuoMonth/dsh-multi-tenant)
在共享 DSH Runtime 内做应用层隔离不同，本项目让每个 Runtime 都有一个显式的租户所有者，
并把该 Runtime 实现为独享的 Pod / 容器边界。

真正的基数关系是：

> **1 个 Runtime → 恰好属于 1 个 Tenant；1 个 Tenant → 可以拥有 0..N 个 Runtime。**

一个租户可以针对通用助手、数据分析、开发等任务启动不同 Runtime，但任何 Runtime 都不能
跨租户共享。

这比应用层多租户提供更强的边界，但项目不会宣传为“绝对无法逃逸宿主机”。普通容器仍共享
宿主 Linux kernel；对于任意代码执行等 hostile workload，可以选择更强的 `sandboxed`
安全等级，并由合适的 Kubernetes `RuntimeClass` 承载。

## 两种方案如何选择

| 项目 | 模式 | 隔离边界 | 适用场景 |
| --- | --- | --- | --- |
| [`dsh-multi-tenant`](https://github.com/GuoMonth/dsh-multi-tenant) | Shared Runtime | DSH 应用层 | 高资源利用率、可信插件生态 |
| **`dsh-isolated-runtime`** | Isolated Runtime | 独享 Runtime / Pod 边界 | 更强租户隔离、插件 / Terminal / 代码执行 |

二者互补：**Shared Runtime** 对 **Isolated Runtime**，
**application-enforced isolation** 对 **infrastructure-enforced isolation**。

## 指导原则

- **所有权必须进入数据模型。** Runtime ownership 是 API 与 runtime seam 的一部分；
  外租户查询/删除默认拒绝，而不是只靠文档约定。
- **分配 Runtime，不重新实现 kube-scheduler。** 本项目决定复用还是创建 Runtime、
  profile、资源与安全等级；Pod 最终落在哪个 Node 由 Kubernetes 负责。
- **身份来自可信 transport。** Gateway 的 Principal 必须由服务端认证边界建立，
  不能从普通请求 body 接收。
- **状态持久，Pod 可丢弃。** 第一阶段连续性采用 DSH/session/workspace/artifact 的逻辑状态
  持久化到 S3/MinIO 兼容对象存储，然后恢复到新 Runtime；M0 不承诺 CRIU/进程内存快照。
- **Kubernetes-native first。** Kubernetes 特定代码放在明确 adapter 边界后面；只有第二个
  backend 真正证明共同点后，才冻结 runtime-agnostic 公共契约。
- **暴露安全等级，而不是一堆底层旋钮。** 平台 profile 选择 hardened `standard` 或更强的
  `sandboxed`，而不是让调用方自己拼 seccomp/capabilities 等安全策略。

## 组件

| 组件 | 角色 |
| --- | --- |
| Runtime boundary | 租户独占的 Pod / 容器 Runtime，绝不跨租户共享。 |
| Provisioning | 标准 DSH runtime images + workload/security profiles。 |
| Runtime allocation | 决定复用/创建、profile、资源与安全等级；不选择 Kubernetes Node。 |
| Gateway | 可信认证 + tenant/session 解析；后续演进为 DSH HTTP/WS/stream 的 runtime router/reverse proxy。 |
| Persistence / restore | 将 DSH/session/workspace/artifact 逻辑状态持久化到对象存储并恢复到新 Runtime。 |
| Control plane | API / CRDs / controllers 编排生命周期。 |

## 仓库结构

```text
api/v1alpha1/          版本化 API 类型
cmd/
  gateway/            可信准入 / router 骨架
  scheduler/          Runtime allocation 进程（名称暂定）
  controller/         生命周期编排
pkg/
  runtime/            租户所有权进入契约的 runtime seam
    kubernetes/       第一个 backend adapter
    runtimetest/      可复用 backend 契约测试
  provisioning/       DSH runtime images + profiles
  scheduling/         Runtime allocation（不是 Node scheduling）
  gateway/            trusted principal + admission
  checkpoint/         逻辑持久化/恢复的唯一权威
  controlplane/       生命周期编排
config/               CRD + RBAC 清单
```

全局模型见 [docs/specs/architecture.md](./docs/specs/architecture.zh-CN.md)，
开发顺序见 [ROADMAP.md](./ROADMAP.zh-CN.md)。

## 许可证

MIT
