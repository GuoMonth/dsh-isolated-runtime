[English](./README.md) | 简体中文

# dsh-isolated-runtime

面向 [DeepSeek Harness](https://github.com/deepseek-ai/deepseek-harness)（DSH）的
Kubernetes 原生隔离运行时：一个租户一个 Pod，具备强隔离、可恢复会话与可插拔的运行时镜像。

> **阶段：启动中。** 仓库与文档正在初始化；控制平面组件（Scheduler、Gateway、
> checkpoint/restore、runtime images）已规划但尚未实现。参见 [ROADMAP.md](./ROADMAP.md)。

## 这是什么

`dsh-isolated-runtime` 是 DSH 的**基础设施级**租户隔离层。与
[`dsh-multi-tenant`](https://github.com/GuoMonth/dsh-multi-tenant)
在共享运行时*内部*（应用层）执行隔离不同，本项目给每个租户**自己的运行时边界**——
一个独享的 Pod / 容器运行时——从而让不可信租户（第三方插件、Terminal 会话、
Python / Node 代码执行）无法逃逸到相邻租户或宿主机。

隔离边界是**运行时本身**，而不是共享进程内的一条规则。租户的代码、插件与文件系统
都生活在自己的 Pod 中，拥有自己的网络命名空间、资源限制与容器运行时策略。控制平面
（Scheduler、Gateway）决定会话在*哪里*运行，并通过 checkpoint/restore 在重启后*恢复*它。

## 二者如何选择

| 项目 | 模式 | 隔离边界 | 适用场景 |
| --- | --- | --- | --- |
| [`dsh-multi-tenant`](https://github.com/GuoMonth/dsh-multi-tenant) | Shared Runtime | DSH 应用层 | 高资源利用率、可信插件生态 |
| **`dsh-isolated-runtime`** | Isolated Runtime | Pod / Container Runtime | Strong tenant isolation、插件 / Terminal / 代码执行 |

> 如果你的场景要求 **Strong Tenant Isolation**，包括第三方插件、Terminal、
> Python/Node 代码执行等运行时隔离，请使用 **`dsh-isolated-runtime`**。
>
> 如果你只需要在共享 DSH Runtime 上做应用层隔离——在可信租户之间最大化资源利用率——
> 请使用 [`dsh-multi-tenant`](https://github.com/GuoMonth/dsh-multi-tenant)。

一句话概括：

- **`dsh-multi-tenant`** → 想省资源、共享运行时，就遵守多租户协议。
- **`dsh-isolated-runtime`** → 要 Strong Tenant Isolation，就直接把 Runtime 隔开。

两个项目互补而非竞争：**Shared Runtime** 对 **Isolated Runtime**，
**application-enforced isolation** 对 **infrastructure-enforced isolation**。

## 指导原则

- **基础设施级隔离** —— 边界是真正的运行时原语（Pod / 容器），而非共享代码里的约定；
  租户不可能因疏忽或遗漏而跨越它。
- **一个租户一个运行时** —— 没有两个租户共享同一个运行时；隔离不依赖租户遵守协议。
- **运行时无关的控制平面** —— Scheduler 与 Gateway 面向容器运行时接口，而非仅面向
  Kubernetes；未来 Docker、Firecracker 或 Nomad 都可以作为后端。
- **可恢复会话** —— checkpoint/restore 让会话的运行时状态跨 Pod 重调度保留，
  使隔离不以牺牲会话连续性为代价。

## 组件（初始范围）

| 组件 | 角色 |
| --- | --- |
| Scheduler | 在资源与安全策略约束下，将每个租户会话调度到独享的运行时（Pod）。 |
| Gateway | 将会话路由到其隔离运行时的准入点。 |
| checkpoint/restore | 跨重调度捕获并恢复会话的运行时状态。 |
| runtime images | 按工作负载（Terminal、Python、Node、……）提供的标准运行时镜像与 profile。 |

里程碑与状态参见 [ROADMAP.md](./ROADMAP.md)。本仓库的开发方式参见
[CONTRIBUTING.md](./CONTRIBUTING.md)。完整文档位于 [`docs/`](./docs/README.zh-CN.md)。

## 许可证

MIT
