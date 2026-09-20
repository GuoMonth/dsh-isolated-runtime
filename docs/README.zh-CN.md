# 文档索引

- [R2 internal Cell Connector](cell-connector.md) — source/artifact boundary; regression pending.

- [平台接入配置（R1）](platform-access.md) —— 首个实现切片，集群验证待完成。
- [共享宪法入口](../CONSTITUTION.md)
- [内部 RuntimePort](design/runtime-port.zh-CN.md) / [Cell adapter](design/cell-adapter.zh-CN.md) —— 待实现设计。
- [当前 alpha MVP 方向与验收](./alpha-mvp.zh-CN.md)
- [Go 工具链、生命周期检查与泄漏诊断](./go-development.md)
- [本地安装](./quickstart.zh-CN.md)
- [AI 安装手册](./ai/local-run.md)
- [Alpha 发行与网络依赖](./distribution.md)

- [架构](./specs/architecture.zh-CN.md) —— 资源归属、边界、数据流与非目标。
- [威胁模型](./specs/threat-model.zh-CN.md) —— 保证、假设与攻击面。
- [Namespace 契约](./specs/namespace-contract.zh-CN.md) —— 原生能力与策略边界。
- [指标契约](./specs/metrics.zh-CN.md) —— 有界聚合观测面。
- [DSH 兼容性](../compat/dsh/README.zh-CN.md) —— 精确上游实证与 launcher 决策。
- [Roadmap](../ROADMAP.zh-CN.md) —— 当前集成顺序。

可执行 API 细节以 Go 类型、生成的 CRD 和测试为准，不在文档中复制维护。

按任务读取。archive/、旧发行记录与验收快照是历史证据，不是当前需求；安装现有版本时以其随包文档为准。
