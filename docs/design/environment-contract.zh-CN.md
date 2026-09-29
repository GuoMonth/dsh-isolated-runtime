# RC2 AgentEnvironment 契约（A / G1）

固定 DSH 0.2.0-rc.2 / `639ed015397290b3745d163aafe02ffee4aa3f84`，仅服务本轮 MVP。
类型与唯一 Pod 模板在 `packages/environment-connector/src`；包名
`@dsh/environment-connector-internal`，精确 tarball 由平台 vendor 消费。
A 提供可编译契约、可执行模板与真实 RC smoke；B 实现
`createAgentEnvironmentRuntime(options: EnvironmentRuntimeOptions): AgentEnvironmentRuntime`。
A/G1 不提供伪装为生产实现的 factory；B 已实现生产 factory 并删除旧 Cell 包。当前入口和真实验证见 [B 阶段证据](../evidence/mvp-b-runtime-2026-09-29.md)，没有旧接口回退。

## 输入、归属与供应

平台授权后生成稳定 allocationKey，owner 是 tenantId/principalId，不是 namespace。
create 不接受调用方 PodSpec、模板选项、凭据、namespace 或 PVC 名称。
管理员配置 API server/CA/token 文件、namespacePrefix、domain、固定 image digest、
一个 storage.size/StorageClass、原生 CPU/Memory requests/limits、platformNamespace。
资源不足由 Kubernetes 报告，申请容量不是目录硬配额，不增加 min/max 或扩容。

runtime 按原 allocationKey 派生 namespace/资源名（例如 SHA256 的固定截断），
按需供应带不可变 owner/allocation 元数据的独立 namespace、无 token 的 workload SA、
固定 NetworkPolicy、一个外部 PVC 和上游 Sandbox。不得认领同名外来 namespace/资源。
初次分配只允许同 key 同 owner 的有限去重；原绑定已存在后资源缺失必须拒绝，不重建。
管理员授予 runtime 供应命名空间及这些固定资源的 RBAC，平台用户不取得 Kubernetes 权限。
namespace 仅为基础设施隔离边界，不能代替平台登录/用户授权。
固定 ingress 仅允许 platformNamespace 内标记 `app.kubernetes.io/name=dsh-platform` 的 Pod。
DNS 使用 kube-system DNS，公网出站排除集群/节点/私网/metadata 地址；具体规则由 B 独占维护，D 引用。

只有一个 PVC，挂载 `/var/lib/dsh/data`；子目录 workspace、home、dsh 分别用于 cwd、HOME、DSH_HOME。
凭据默认 `$DSH_HOME/.credentials.yaml`，工具授权可写 HOME；无平台密钥/集群 token 注入。
PVC 不设 Sandbox ownerReference，删除运行资源默认保留 PVC 和 namespace。
模板调用 `environmentTemplate`，不复制 JSON 到 Helm 或平台。工作负载初始 cwd 为卷根，
launcher 创建子目录后在 workspace 启动 DSH；这样新空卷不需要额外 init 控制路径。

## 查询、访问和正常/未知结果

完整 EnvironmentRef 包含 owner、allocationKey、namespace、Sandbox name/UID、唯一 PVC name/UID。
平台保存此精确绑定，不能从 origin 或 namespace 反推授权。runtime 每次比对 owner、CR/PVC UID、
模板、真实 Pod/Service/Endpoint；Pod UID 随重建变化，由 runtime 当次读取核验，不由平台作为长期身份。
读到 Pending/Unavailable 与读取失败分开；已绑定后 404 是 StaleInstance。
未绑定 inspectAllocation(intent, undefined) 可返回 null；部分分配返回 AllocationUnresolved
并要求查原 key，不能换 key。create 返回已完整绑定的 Pending/Ready view；未能确认完整绑定时返回结构化错误。

connect 只接受 Ready 的精确实例。context.signal 拥有返回对象的整个生命周期：
abort 后拒绝新请求，并关闭已有 HTTP 流和 WebSocket，不能只取消 connect 初次查询。
DSH HTTP/WS 内容透明转发；launcher 保留原生 token/cookie 换取与 authority 边界，不重写模型/插件协议。

stop/start/delete 必须同时比较 Sandbox UID 与调用方读到的 revision（Kubernetes resourceVersion）。
stop 先撤销访问，等待上游停止；Stopped 要求同一 writer 的正面终止证据。
Pod 404、等待取消、超时或节点失联不能证明终止；返回 StopUnverified 并阻断 start/delete。
B 将必要停止证据保存在原 Sandbox 的受控元数据中，不另建状态数据库或恢复控制器。
start 仅接受正面 Stopped 与原 PVC UID；缺卷/替换拒绝，不建空卷。
delete 只在正常停止证实后删原 UID 运行资源；Deleted 不表示删卷或永久 tombstone。

所有错误为 EnvironmentError / EnvironmentFailure，显式 code、stage、effect
(not-submitted/accepted/unknown)、observedState、retry、allocationKey、correlationId、nextAction。
未知提交先查原 key/UID；无无限重试、后台自动恢复或更换 key。日志/错误不包含 token、凭据或原始 API 响应。

## G1 后文件所有权

| Owner | 独占路径与责任 |
| --- | --- |
| B runtime | runtime 仓库实现全部；新 `packages/environment-connector/`、launcher/compat/image、runtime RBAC/network、测试；删除 Cell/Operator/snapshot/standalone；不改 A 接口，缺口经协调者裁决 |
| C platform | 平台 `packages/`、`scripts/`（除下行安装专用）、`integration/`（除安装专用）、`vendor/`、根 package.json/pnpm-lock.yaml/exports；EnvironmentBinding/OIDC/访问/生命周期；后续精确 Connector pin 由 C 独占 |
| D install | 平台 `charts/`、`integration/installation/`、`docs/installation/`；仅 Helm/安装 fixture/安装说明；不复制 workload/RBAC 实现，不改 package/lock/pin 或平台入口 |
| E integration | 协调者放行后统一双方 pin、安装和真实 E2E/双语发行文档；最终组合由 E 单负责人更新 |

A 串行期间独占双方 package/lock/pin。G1 后 B 独占 runtime、C 独占平台包/锁和 vendor；
D 所需运行配置通过上述 options 和 runtime 模板消费，不另加模板注册框架。
唯一集群写入权 A → B → E；C/D 开发阶段离线检查，不能并发升级 CRD/controller。
