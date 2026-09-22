# Cell adapter：旧实现映射参考

> **旧 Cell 实现映射，不是目标架构规范。** 新需求和阶段以[平台 Agent Workspace 设计](https://github.com/GuoMonth/dsh-multi-tenant/blob/main/docs/design/agent-workspace.zh-CN.md)及[Issue #104](https://github.com/GuoMonth/dsh-multi-tenant/issues/104)为准。当前代码仍实现 Cell；W1 才计划破坏性改名为 AgentWorkspace，W2 停止/启动尚未实现。本稿不得用于声称这些目标已落地。

2026-09-20 修订的旧设计稿；旧实现记录见 [R5](r5-allocation.md) / [R6](r6-deletion.md)，原则入口见[项目宪法](../../CONSTITUTION.md)。

## 1. 尽量复用当前实现

当前 Cell Operator 调谐 StatefulSet、Service、NetworkPolicy 与 PVC；launcher 承载原生 DSH。目标 AgentWorkspace 仍可由薄 runtime-owned CRD/Operator 聚合这些原生资源，也可由 runtime 直接管理；平台/runtime 分工不强制平台接管原生对象操作。目标结构见上方规范设计。

旧方案由平台组合根注入 adapter。目标仍隔离平台业务与 Kubernetes 资源细节，但 runtime 可以直接管理 StatefulSet/PVC；namespace、RBAC、存储、Gateway/TLS 由管理员配置。

## 2. 身份与管理调用

| 内部事实 | adapter 内部映射 |
| --- | --- |
| 受信 scope | 管理员配置的 namespace allowlist，不由浏览器选择 |
| 分配 key | 确定性 Cell 名称，只在当前分配内去重 |
| opaque 实例身份 | Kubernetes Cell UID |
| 模板 | 受控、精确 image digest/profile/资源/存储配置 |
| 当前状态 | generation 对应的 Cell conditions 与实际模板核验 |
| 请求删除 | UID 条件 DELETE；返回接受情况，不声称 writer 已停止 |

本文旧方案记录不可变 owner/模板绑定的实现约束。新设计中 namespace 是管理员配置的基础设施 scope，不等于 OIDC tenantId；平台授权为唯一 owner 权威，runtime 只保留不可变 owner 关联用于资源匹配。模板校验由 runtime 负责，漂移时不给 Ready/连接。规范以本文顶部链接为准。

create 遇 AlreadyExists 后读取并核对原归属/模板，不做认领式 apply。known-ref 丢失禁止自动重建；unknown create 只用原 key 查询，未决期间平台拒绝自动删除/重建。没有支持队列重放或永久去重的承诺。

## 3. 集成访问链

```mermaid
flowchart LR
  B[浏览器] --> G[Gateway / TLS]
  G --> P[平台认证入口]
  P --> A[受限 Connector]
  A --> S[精确 Cell Service]
  S --> L[launcher / DSH]
```

- 首期使用明确的 platform 参考配置，不热迁移现有 standalone Cell。启用时检查模式冲突、既有直达路由；不符合即失败，不删除其他部署资源。
- Gateway 把平台及获准实例 origin 送到平台。集成 Cell 不额外生成绕过平台的公网 route；authority 配置与“是否生成直达 route”分开。
- Connector 在实际请求/upgrade 前检查 Cell UID、owner/template、未删除与当前 readiness，以及派生 Service 的 owner/selector。浏览器不能提供任意 URL/port。
- 私有连接上下文留在同进程受限句柄中；无可序列化 bindingId、TTL token 或浏览器 capability。
- launcher 集成目标检查与精确 authority 由 runtime 实现，平台 header 只在受控网络入口中有意义，不单独当作认证。
- NetworkPolicy 限制管理员控制的入口 namespace/pod；租户无该处 Pod/策略管理权限。验证实际 CNI 执行；故障则拒绝启用，不声称普通容器是强沙箱。
- 原生 HTTP/WS/stream/Fetch、Host/Origin 和 DSH cookie 语义必须真实验证。旧 ingress 的 runtime cookie 注入不是新接口义务；平台凭据、外来身份/forwarded header 不进入 Cell。

原独立 OIDC/RBAC 路径描述现有版本，不要求新集成长期兼容。若迭代替换它，应写清破坏点和当前验证方式；无需为了历史部署维护双模式迁移层。

## 4. 删除、保留和停止证据

不新增 `spec.lifecycle=Retired` 或永久终态 Cell。请求删除带确切 UID；同名新 UID 不受旧删除影响。`accepted`、正在删除、记录缺失和“执行已停止”必须分开。

data PVC 默认 Retain、private-state 及外部 Secret 各自按实际资源 ownership 处理；管理员操作说明要逐项写清，不能统称“全部保留”或“全部删除”。本次文档不改变当前执行行为。

当前上层不暴露旧卷自动复用/恢复接口。正常删除路径可观测 owned 资源；节点失联、强制删除或其他无法确认 writer 的情况报 CleanupUnverified，留下目标与检查提示。不得通过强删状态、Node 心跳或 RWO 名称伪造停止证明，不新增 Node watch/fencing 控制面。

明确管理员数据销毁/测试环境重建流程即可，不维护 purge API、升级通道或历史数据迁移。数据删除必须有明确范围与授权，代码不兼容不能自动清空旧卷。

## 5. 错误映射与验证顺序

配置/模板/权限检查尽早执行；Kubernetes 错误翻译成 RuntimePort 的 code/stage/effect/retry/nextAction/correlationId，脱敏后交给平台。API 超时是 unknown，不是资源已释放。正常 Pending 允许最多 30 秒前台等待，超时即交还状态；不改 Operator 原生调谐为平台自动修复。

S1 预建集成 Cell 和真实访问链 → S2 OIDC/父子会话 → S3 创建读取与失败诊断 → S4 当前 UID 删除请求、正常 Pod 重建的数据保留、真实模型验证。

无需等待 Helm、任意基础设施矩阵、永久终态或第二个后端。实际发布/安装前仍要验证当次产物，不能以缩小兼容范围为由跳过当前身份与隔离检查。
