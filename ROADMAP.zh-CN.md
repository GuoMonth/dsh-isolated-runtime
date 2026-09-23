# 当前路线图

当前顺序和验收由 multi-tenant 平台维护：[AgentEnvironment 设计](https://github.com/GuoMonth/dsh-multi-tenant/blob/main/docs/design/agent-environment.zh-CN.md)、[主 Issue #104](https://github.com/GuoMonth/dsh-multi-tenant/issues/104) 与[平台 roadmap](https://github.com/GuoMonth/dsh-multi-tenant/blob/main/docs/roadmap.md)。

目标是 K8s 唯一后端的 AgentEnvironment。当前源码仍实现 Cell，源码调整尚未落地。W1 移除 Process/Docker 产品后端、standalone 启动与 snapshot/restore。W1 前由 runtime [#100](https://github.com/GuoMonth/dsh-isolated-runtime/issues/100) 做有界的上游选型检查：只装 core `Sandbox` CRD/controller，使用普通 Pod、外部管理的 data/private PVC，不套同义 CRD、不 fork；`Sandbox` 只是基础设施对象，平台只暴露窄的 AgentEnvironment 契约。若薄 adapter 无法保留存储归属、启停证据、实例身份、凭据和网络边界，则保留当前实现。这是前置检查，不是新的长期开发轮次。W2 在选定实现上增加健康节点正常停止/启动，保持环境及两卷身份，不提供分区 fencing。W3 联合验证真实授权、持久 HOME 和性能后再发行。备份、热池、自动 idle 暂缓。见[当前 MVP 实现状态](docs/alpha-mvp.zh-CN.md)。

[源码评估、测试范围与采用条件](https://github.com/GuoMonth/dsh-multi-tenant/blob/main/docs/design/agent-sandbox-evaluation.zh-CN.md)。
