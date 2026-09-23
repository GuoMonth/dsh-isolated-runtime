# 文档索引

从[AgentEnvironment 设计](https://github.com/GuoMonth/dsh-multi-tenant/blob/main/docs/design/agent-environment.zh-CN.md)、[Issue #104](https://github.com/GuoMonth/dsh-multi-tenant/issues/104)和[当前实现状态](alpha-mvp.zh-CN.md)开始。目标是 K8s 唯一后端的 AgentEnvironment；产品改名前当前源码/制品仍使用 Cell。平台负责用户身份与 OIDC；runtime 负责环境资源机制和已校验 Connector。

- [共享宪法](../CONSTITUTION.md)
- [RuntimePort](design/runtime-port.zh-CN.md) / [Cell adapter](design/cell-adapter.zh-CN.md) —— 旧 Cell 实现/设计参考；AgentEnvironment 设计优先
- [平台接入配置](platform-access.md)
- [Cell Connector](cell-connector.md)
- [Cell MVP 范围与验收](alpha-mvp.zh-CN.md)
- [架构](specs/architecture.zh-CN.md)与[威胁模型](specs/threat-model.zh-CN.md)
- [Go 开发](go-development.md)
- [当前发行说明](distribution.md)与[Cell CLI 源码 README](../packages/cell-cli/README.md)
- [DSH 精确基线](../compat/dsh/README.zh-CN.md)
- [当前路线图](../ROADMAP.zh-CN.md)

W1 计划移除 Process/Docker 产品运行后端、旧 standalone 启动与 snapshot/restore；工作区 Pod 内子进程、OCI 镜像构建及 kind 的 Docker 底座保留。W1 前置选型检查见 runtime [#100](https://github.com/GuoMonth/dsh-isolated-runtime/issues/100)：只装 core `Sandbox` controller，使用普通 Pod 与外部 PVC，不套同义 CRD、不 fork。W2 停止/启动与卷身份保证取决于该检查；当前 Cell 路径仍使用现有实现。W3 发行前联合验证真实授权、持久 HOME 和性能。备份、热池、自动 idle 暂缓。已发布 npm `0.3.0-alpha.1` 及固定清单仍描述该发行版；源码计划须单独验收发行。`archive/` 是历史材料，不是当前验收清单。
