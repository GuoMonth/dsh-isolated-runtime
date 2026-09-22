# Agent Workspace 架构方向

> **这是设计目标，不是当前实现。** 权威设计与阶段顺序见[平台 Agent Workspace 设计](https://github.com/GuoMonth/dsh-multi-tenant/blob/main/docs/design/agent-workspace.zh-CN.md)和[Issue #104](https://github.com/GuoMonth/dsh-multi-tenant/issues/104)。当前源码及已发行制品仍使用 `Cell`；W1 计划破坏性改名为 `AgentWorkspace`，并移除 Process/Docker 产品运行后端、standalone 启动及 snapshot/restore。不要把计划描述成已实现或已发行。

## 权责边界

`AgentWorkspace` 是计划中的 namespaced Agent 实例及持久 HOME/数据边界。Kubernetes 是唯一产品运行后端。薄 runtime-owned CRD/Operator 可以调谐原生 StatefulSet、PVC、Service 和策略；runtime 直接管理这些原生资源也是有效选择。平台/runtime 分工隔离用户身份与应用协议、基础设施机制，但不要求把 StatefulSet/PVC 所有权推给平台。namespace 是管理员配置的基础设施 scope，不是 OIDC tenantId；参考部署预配置 per-user namespace，不建设自动 namespace 租户系统。平台授权是唯一 owner 权威，runtime 只保存不可变 owner 关联用于资源匹配，不实现 OIDC。

| 关注点 | 负责人 |
| --- | --- |
| 用户身份、OIDC、成员、授权和用户会话 | multi-tenant 平台 |
| AgentWorkspace 资源、镜像、生命周期与已校验的内部传输 | runtime 仓库（目标命名；当前仍为 Cell） |
| 应用协议、会话、工具和模型调用 | DSH |
| Pod 调度、Service、网络策略和卷 | Kubernetes 及其已安装的提供方 |
| TLS 终止和入口路由 | Gateway API 实现 |

目标 AgentWorkspace API 不接受用户指定的 Pod 名称、UID、IP、Node、route 或 session 作为目标。Runtime 从 Kubernetes 状态解析实例，并在转发前校验 identity。**当前实现事实：**源码使用 `Cell` 和标准 Kubernetes Pod 边界；main 当前只接受 `securityClass: standard`。已发布 npm `0.3.0-alpha.1` 及固定清单仍描述其旧发行版，本设计文字不改变制品。

## 请求路径

```text
Browser → Gateway（TLS 与路由）→ multi-tenant 平台（OIDC 与授权）
        → Connector → 已校验的 Cell Pod → DSH
```

集成链路中的 Gateway 负责 TLS 终止和路由；OIDC 登录及用户授权由平台负责。Connector 解析平台授权的 Cell 引用，校验当前 Kubernetes identity 链（包括 Cell UID 与 Pod owner），再代理 HTTP、WebSocket 和流式流量到选定 Pod。Go proxy 将 DSH launch token 保存在进程内存中，用于本地 bootstrap exchange，不放入公网 URL 或日志。DSH 继续负责应用协议和会话行为。

NetworkPolicy 只允许平台 Connector 路径访问 Cell proxy port。DSH listener 仍仅监听 Pod 内 loopback。策略是否生效取决于实际 CNI；仅生成 NetworkPolicy 不构成隔离已生效的证据。

## 身份与数据边界

目标工作区由 Kubernetes namespace 与不可变资源 UID 标识；同名但 UID 不同的重建资源是新实例。当前 Connector 代码会依据实时 Kubernetes 状态校验 Cell 与其 Pod identity；过期或不匹配目标 fail closed。

data 与 private runtime state 使用不同 PVC。停止时保留两卷；删除时 data 可保留，private 仅在显式删除凭据范围内删除。W3 目标是在核验客户端精确路径后，把 HOME/XDG auth 放 private，把 DSH session/工作文件放 data。**当前实现只把 DSH 指定凭据放在 private；HOME 仍在 data，不能声称当前已隔离所有 CLI 凭据。** 平台 OIDC Secret 独立保存，不得挂载进工作区。资源状态及隔离是否生效仍以 Kubernetes 对象和已安装提供方为准。

普通 Pod 不能防御已失陷的 Node、kernel、集群管理员或存储/网络提供方；本 POC 不作此类保证。

## 计划生命周期与暂缓项

W2 计划在健康节点范围，通过显式 StatefulSet 和显式 data/private PVC 增加 `Running`/`Stopped` 与正常停止/启动，保留两卷身份；不承诺节点分区 fencing，节点不健康或分区时强删不属于停止保证。备份、热池和自动 idle 暂缓。W3 要在发行前联合验证真实授权、持久 HOME 和性能。

W1 还计划移除 Process/Docker 产品运行后端、旧 standalone 启动及 snapshot/restore。这不影响工作区 Pod 内的子进程、OCI 镜像构建，也不影响 kind 使用 Docker 作为集群底座。

## 当前 Cell 历史实现

当前源码仍包含 CellSnapshot/restore 与 standalone 历史文件；本文档调整不删除代码、不修改发行制品。这些路径不属于 AgentWorkspace 目标。版本背景见[归档的 standalone alpha 文档](../archive/standalone-alpha1/README.md)。
