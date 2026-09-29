# RuntimePort：旧 Cell 内部契约参考

> **旧 Cell 设计，不是当前规范。** 当前目标以[平台 AgentEnvironment 设计](https://github.com/GuoMonth/dsh-multi-tenant/blob/main/docs/design/agent-environment.zh-CN.md)和[Issue #104](https://github.com/GuoMonth/dsh-multi-tenant/issues/104)为准。本文只帮助理解现有 Cell 代码，不承诺 Process/Docker 多后端或未来兼容实现。当前源码仍是 Cell；产品改名及 W2 停止/启动尚未实现。

2026-09-20 修订的旧设计稿；旧实现记录见 [R5](r5-allocation.md) / [R6](r6-deletion.md)。原则入口见[项目宪法](../../CONSTITUTION.md)，旧资源映射见 [Cell adapter](cell-adapter.zh-CN.md)。

本稿曾用于 Cell 内部调用设计；类型、错误码和状态格式不是稳定公共协议。平台/runtime 职责隔离仍适用，但 K8s 是唯一目标后端，不再承诺中立多后端边界。

## 1. 权威和对象

平台拥有用户身份、Environment 归属和授权；runtime 拥有实例身份、资源事实和连接机制。用户侧 OIDC、邮箱/组/角色不进入内部资源模型。

- `AllocationKey`：平台第一次写入前持久生成的随机分配标识，加受信的 runtime/scope 配置；不能来自浏览器 namespace 或 URL。
- `InstanceRef`：分配标识 + runtime 颁发的 opaque 实例身份。平台只比较，不解析 Cell UID；已知 ref 不得自动重绑同名新实例。
- `TemplateRef`：当前配置中获准的精确构建/profile 修订，不接受任意镜像、路径、参数或可变 latest。
- `InstanceView`：精确 ref、经 runtime 验证的 owner/template、`Pending | Ready | Unavailable | Deleting` 及 reason。不存在的记录是错误/事实，不是 `Retired` 或“writer 已停止”。

平台不持有 Pod 状态副本；无公共 namespace、PID、containerId、socketPath 或 Cookie 字段。不同应用进程/Pod 在同一 Cell 内重建不改变 ref；新 Cell 必须有新身份。

## 2. 最小调用面

以下是逻辑操作，不预设独立包、JSON schema 或 HTTP 路径：

| 操作 | 前提 | 返回/后置条件 |
| --- | --- | --- |
| create(key, owner, template) | 平台已授权、保存 key，当前分配尚未绑定后又丢失 | 资源持久接受后返回 Pending/当前 view；同 key 同意图且资源仍存在则复用；冲突拒绝接管 |
| inspect(key, expectedIdentity?) | 来自受信 scope 与环境绑定 | fresh 读取的 view 或明确错误；若已知身份不匹配则 StaleInstance |
| connect(ref, context) | 平台会话与环境授权仍有效 | 单次受限 Connector handle，含规范应用 origin；请求/upgrade 前检查精确目标，清洗凭据并透明传输 |
| requestDelete(ref) | 显式管理员操作，当前分配无未决 create | 返回“删除请求已接受”，而非资源已释放；必须带精确身份前提 |

context 的取消信号/截止时间是当前语言绑定的本地控制，不是线协议。连接句柄不持久保存、不转交浏览器，不分配额外 bindingId/TTL 注册表。connect 不给浏览器任意 endpoint/port，不替平台做用户认证。

平台 close/logout/revoke 仅 abort 连接；不调用 requestDelete。尚未实现的暂停、快照、exec、恢复、升级不通过万能 `stop()` 或 `extensions:any` 暗中引入。

## 3. 有限去重与并发边界

1. 当前资源存在期间，同 key/owner/template 并发 create 只产生一个资源；冲突返回 IntentConflict，不能 patch 接管。
2. create 超时/响应丢失：effect=unknown，用原 key inspect，不换 key 建第二份。已经找到原记录可绑定其身份；未找到也不能断言原写入永远不会到达。
3. 当前平台按分配串行化变更；在途/结果未知的 create 不得自动进入删除或新分配。保存必要的未决标记并给出诊断即可，不建设通用操作队列。平台重启遇到未决标记只重读，不自动 replay 写请求。
4. 已绑定实例消失时不能再 create 同 key。管理员决定重建时使用新分配和新数据边界；禁止自动挂载仍可能被旧 writer 使用的卷。
5. 删除/环境重置/版本切换后，旧调用与旧状态不受支持。无持久消息重放、无无限后台重试、无无限期 exactly-once。正常调用方须停止该分配的后续写入；任意外部重放不在内部接口保证内。
6. 以上范围不足以构成永久删除去重保证，因此不引入 tombstone、Cell Retired 字段或通用操作日志。后续如真有外部异步调用/重放需求，必须重新设计，不能沿用此内部范围冒称支持。

## 4. 删除与数据

requestDelete 使用精确身份/条件请求；迟到的旧实例删除不能删除同名新实例。操作超时返回 unknown，读取原目标判断后续，不重复对新身份发删除。

成功仅说明 API 接受当前身份的删除意图。找不到记录返回 `RecordMissing`，指出无法据此证明旧执行已停止；已在 Deleting 可返回“已接受”。平台不使用“清理成功”或“存储可复用”字样。

保留/删除策略在当前模板和管理操作说明中明确：data、private-state、外部 Secret 分开核对。保留数据不承诺未来可导入，销毁数据必须有明确目标及授权。人工清理要可重试、不波及外来资源，但本期不增加 purge API。

正常 Pod 生命周期与节点失联/强制删除的故障前提不同。API 对象消失、RWO 标签或 Node 心跳不是普适物理停止证明；无法确认时 fail fast 返回 CleanupUnverified，管理员处理。该错误不创建无限恢复任务，不允许复用可能仍被写入的数据。

## 5. Fast fail 与 AI 可读错误

当前构建内错误结构：

```json
{
  "code": "CreateOutcomeUnknown",
  "stage": "create",
  "target": {"allocationKey": "example-allocation"},
  "observedState": "unknown",
  "effect": "unknown",
  "retry": "read-first",
  "message": "创建请求超时，尚不能确认是否已接受",
  "nextAction": "用同一分配标识 inspect；不要换标识重建或立即删除",
  "correlationId": "example-request"
}
```

这是示意，不是发布的 JSON 标准。target 仅含调用方有权看到的标识；不含内部地址、cookie、Token、secret 内容或用户提示原文。日志保留同一 correlationId；对外错误屏蔽原始 Kubernetes 错误中的敏感值。

| code | effect / retry | 处理 |
| --- | --- | --- |
| InvalidConfiguration / VersionMismatch / UnsupportedTemplate | not-submitted / never | 指出配置字段及期望修订；立即拒绝，不兼容降级 |
| Forbidden | not-submitted / never | 拒绝，不泄漏其他租户资源存在性 |
| IntentConflict / StaleInstance | not-submitted / never | 核对原绑定，不认领另一实例 |
| NotReady / ReadUnavailable | not-submitted / read-first | 返回观测状态、失败阶段与查询建议；不使用缓存伪造健康 |
| WaitDeadlineExceeded | accepted（已知）或 unknown / read-first | 结束等待，说明创建是 Pending 还是结果未知 |
| CreateOutcomeUnknown / DeleteOutcomeUnknown | unknown / read-first | 查询同一目标，不能当作取消成功 |
| RecordMissing | not-submitted / never | 停止自动推进；不把 404 解释为安全释放 |
| CleanupUnverified | accepted 或 unknown / never | 给出未证实项和管理员核验步骤，不复用旧数据 |

effect 必须反映本次写入事实，不能仅由 HTTP 状态码猜测。retry 是允许的建议，不是自动重试承诺；写请求默认不自动重试，读取可以有界重试。已知 Pending 就绪等待最多 30 秒（部署可缩短），到期交还诊断；Operator 原生调谐继续，但平台不另起补偿/恢复循环。

nextAction 只能给明确的检查/操作建议，不可夹带自动执行的模型指令。错误可读性以当前故障用例检查，不引入错误本体系统或自动修复 agent。

## 6. 最小行为验收

下表保留契约要求；实际已测项与未覆盖项以 [2026-09-20 集成回归](https://github.com/GuoMonth/dsh-multi-tenant/blob/4ba252765bccb41314c0bdc6b11bcf60cc0b33ef/docs/evidence/cell-regression-2026-09-20.md)为准。新组合验收由 [P3 #100](https://github.com/GuoMonth/dsh-multi-tenant/issues/100)统筹，不把历史设计清单当作全部待测或全部已通过。

| 场景 | 预期 |
| --- | --- |
| 资源存在期间同意图重复/并发创建 | 同一实例；不同归属/模板拒绝 |
| 写入响应丢失或取消等待 | unknown + 原标识查询，不新 key 重建，不隐式删除 |
| 已绑定实例缺失或身份改变 | 明确拒绝，不自动认领/恢复 |
| 旧身份的迟到删除 | 不影响新实例；404 不作为 writer-stop 证明 |
| 平台退出/父子会话撤权 | 已有连接关闭，之后新连接拒绝，Cell 不被删 |
| 未认证/跨环境/伪造 endpoint | 拒绝，凭据不进入用户 Host |
| 模板/版本/实际产物漂移 | 立即拒绝及可解读诊断，不启动兼容适配 |
| 真实 HTTP/WS/stream/Fetch | 原生语义保真，正确 origin/cookie，abort 生效 |
| 正常 Pod 重建 | 同 Cell ref，当前版本的文件和会话仍可用 |
| 未证实清理、节点失联等 | 明确未完成/人工核验，无假成功、强制复用或无限恢复 |

上述接口可以随首条真实链路破坏性调整，但权威划分、授权和资源身份底线不因快速迭代省略。
