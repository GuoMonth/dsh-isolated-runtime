# AgentEnvironment runtime 契约

[English](../en/contract.md) · [文档目录](index.md)

内部包 `@dsh/environment-connector-internal` 实现 `createAgentEnvironmentRuntime(options)`，平台打包精确 tarball。类型见 [port.ts](../../packages/environment-connector/src/port.ts)，资源使用唯一的[固定模板](../../packages/environment-connector/src/template.ts)。

## 输入与身份

平台授权用户 `(tenantId, principalId)`，生成稳定的 `allocationKey`。管理员配置 Kubernetes server/CA/token 文件、namespace 前缀、域名、镜像 digest、StorageClass/容量、CPU/内存 requests/limits 和平台 namespace。调用方不能提供任意 PodSpec 或资源名称。

runtime 创建带归属标签的 namespace、无 token 的用户 ServiceAccount、网络策略、一个外部 PVC 和上游 Sandbox。同名外来资源会被拒绝。平台保存 `EnvironmentRef`：owner、allocation key、namespace、Sandbox 名称/UID、PVC 名称/UID；连接时核对当次 Pod UID。

策略允许平台 namespace 内带平台标签的 Pod 访问。DNS 使用 kube-system，公网出站排除集群、私网和 metadata 地址。Kubernetes RBAC 将 namespace 创建权授予可信平台，但不能按名称前缀限制该权限。[runtime 角色](../../config/runtime/cluster-role.yaml)不能读取 Secret、写 Pod 或删除 PVC/namespace。

## 数据与传输

PVC 挂载到 `/var/lib/dsh/data`：`workspace/` 是工作目录，`home/` 是 HOME，`dsh/` 是 DSH_HOME。原生凭据位于 `$DSH_HOME/.credentials.yaml`。PVC 不设置 Sandbox ownerReference；删除保留卷和 namespace。申请存储容量的实际限制取决于存储后端。

`connect` 只接受已核对 owner、资源、模板和端点的 Ready 实例。context signal 管理整个连接生命周期：取消后拒绝继续转发，并关闭 HTTP 流和 WebSocket。launcher 保留原生 token/cookie 和 authority 校验，转发 DSH 流量时保持模型及工具协议。

## 查询与修改

| 操作 | 行为 |
| --- | --- |
| create | 返回完整绑定的 Pending/Ready 视图；写入结果不确定时保留原 allocation key |
| inspectAllocation | 绑定前可以返回 null；已知资源缺失返回 StaleInstance |
| inspect | 查询精确绑定，区分观察状态与读取失败 |
| stop | 撤销访问、请求暂停，并要求原写入进程终止的证据 |
| start | 要求已确认 Stopped，且 PVC UID 相同 |
| delete | 要求确认停止，删除精确运行资源并报告 dataRetained |

修改同时比较 Sandbox UID 和 `revision`（Kubernetes resourceVersion）。Pod 缺失、等待取消、超时或节点失联不能证明停止。`StopUnverified` 会阻止 start/delete，重新创建 factory 后仍然生效。停止证据保存在受控 Sandbox 元数据中；卷缺失或被替换时失败，不创建空卷。

`EnvironmentError.failure` 包含 code、stage、effect（`not-submitted`、`accepted`、`unknown`）、observedState、retry、allocationKey、correlationId、nextAction。写入结果未知时，先检查原绑定，再决定是否修改。日志不包含 token、凭据或原始 API 响应。

测试见[开发](development.md)，受测组合见[发行验证](https://github.com/GuoMonth/dsh-isolated-runtime/releases/download/v0.4.0-alpha.1/validation.md)。
