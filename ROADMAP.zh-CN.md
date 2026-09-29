# 当前路线图

**产品定位：**开源 Alpha 阶段的企业内网持久 AI 工作环境。首版聚焦 OIDC 登录、原生 DSH、每用户单 PVC、显式启停与一条安装路径；允许破坏性变更，不维护旧版本兼容或迁移。企业规模、灾备和受支持升级后置，详见[产品边界](https://github.com/GuoMonth/dsh-multi-tenant/blob/main/docs/design/enterprise-positioning.zh-CN.md)。

当前顺序和验收由 multi-tenant 平台维护：[AgentEnvironment 设计](https://github.com/GuoMonth/dsh-multi-tenant/blob/main/docs/design/agent-environment.zh-CN.md)、[主 Issue #104](https://github.com/GuoMonth/dsh-multi-tenant/issues/104) 与[平台 roadmap](https://github.com/GuoMonth/dsh-multi-tenant/blob/main/docs/roadmap.md)。

目标是 K8s 唯一后端的 AgentEnvironment。当前源码仍实现 Cell，W1 源码调整尚未落地。W1 正式采用上游 core `Sandbox` CRD/controller 作为唯一基础设施控制路径：普通 Pod、每用户一个外部管理的 PVC，不套同义 CRD、不 fork。`Sandbox` 只是基础设施对象，平台只暴露窄的 AgentEnvironment 契约。runtime [#100](https://github.com/GuoMonth/dsh-isolated-runtime/issues/100) 的有界真实验证已通过，覆盖 DSH HTTP/WS、停止/启动证据、PVC 身份、删除和负例；见[本地证据报告](https://github.com/GuoMonth/dsh-isolated-runtime/blob/main/docs/evidence/agent-sandbox-local-2026-09-23.md)。W1 仍需实现生产 adapter，不能声称 W1/W2/W3 已完成。W2 保留生产停止证据、跨 resourceVersion 并发、删除结果未知屏障和旧 UID 精确校验门槛；不提供分区 fencing。W3 联合验证 OIDC 授权、真实工具、持久 HOME 和发行制品后再发行。备份恢复与受控升级后置，不阻塞本轮 Alpha MVP，记录于平台 [#112](https://github.com/GuoMonth/dsh-multi-tenant/issues/112) 企业验收；热池和自动 idle 继续暂缓。见[当前 MVP 实现状态](docs/alpha-mvp.zh-CN.md)。

[源码评估、测试范围与采用条件](https://github.com/GuoMonth/dsh-multi-tenant/blob/main/docs/design/agent-sandbox-evaluation.zh-CN.md)。
