# Cell 架构

## 权责边界

一个 `Cell` 是一个 namespaced DSH 实例及其持久数据边界。namespace 是租户边界。本仓库把 Cell 意图转换为 Kubernetes 资源，不另建调度器或用户身份系统。

| 关注点 | 负责人 |
| --- | --- |
| 用户身份、OIDC、成员、授权和用户会话 | multi-tenant 平台 |
| Cell 资源、镜像、生命周期与已校验的内部传输 | 本仓库 |
| 应用协议、会话、工具和模型调用 | DSH |
| Pod 调度、Service、网络策略和卷 | Kubernetes 及其已安装的提供方 |
| TLS 终止和入口路由 | Gateway API 实现 |

Cell API 不接受用户指定的 Pod 名称、UID、IP、Node、route 或 session 作为目标。运行时从 Kubernetes 状态解析实例，并在转发前校验 identity。POC 使用标准 Kubernetes Pod 作为边界；当前源码仅接受 `securityClass: standard` 并拒绝不支持的值。本次源码修改不改变已发布 npm `0.3.0-alpha.1` 包及固定清单。

## 请求路径

```text
Browser → Gateway（TLS 与路由）→ multi-tenant 平台（OIDC 与授权）
        → Connector → 已校验的 Cell Pod → DSH
```

集成链路中的 Gateway 负责 TLS 终止和路由；OIDC 登录及用户授权由平台负责。Connector 解析平台授权的 Cell 引用，校验当前 Kubernetes identity 链（包括 Cell UID 与 Pod owner），再代理 HTTP、WebSocket 和流式流量到选定 Pod。Go proxy 将 DSH launch token 保存在进程内存中，用于本地 bootstrap exchange，不放入公网 URL 或日志。DSH 继续负责应用协议和会话行为。

NetworkPolicy 只允许平台 Connector 路径访问 Cell proxy port。DSH listener 仍仅监听 Pod 内 loopback。策略是否生效取决于实际 CNI；仅生成 NetworkPolicy 不构成隔离已生效的证据。

## 身份与数据边界

Kubernetes namespace 与不可变 Cell UID 共同标识 Cell 实例。名称相同但 UID 不同的重建资源是不同实例。转发前 Connector 会根据实时 Kubernetes 状态校验 Cell 与其 Pod 的 identity；过期或不匹配的目标 fail closed。

租户数据与私有运行时状态使用不同存储边界。Provider 凭据属于 Cell 的 DSH 私有状态，或显式配置的同 namespace credentialsRef Secret；平台 OIDC Secret 独立保存，不得挂载进用户 Cell。Provider 凭据不得写入 Cell status、日志或共享租户数据。资源状态及隔离是否生效仍以 Kubernetes 对象和已安装的存储/网络提供方为准。

普通 Pod 不能防御已失陷的 Node、kernel、集群管理员或存储/网络提供方；本 POC 不作此类保证。

## 历史实现

较早源码版本包含 standalone OIDC/RBAC 访问、带 SubjectAccessReview 的 `cell-authorizer` 以及 CellSnapshot/restore 流程。这些是历史实现细节，不是当前集成平台的请求路径或当前 MVP 验收门槛。仓库中可能仍保留对应代码和发行制品；本文档调整没有删除或改动它们。版本背景见[归档的 standalone alpha 文档](../archive/standalone-alpha1/README.md)。
