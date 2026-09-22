# 文档索引

从[平台集成配置](https://github.com/GuoMonth/dsh-multi-tenant/blob/main/docs/reference/quickstart.md)、[共享回归实证](https://github.com/GuoMonth/dsh-multi-tenant/blob/4ba252765bccb41314c0bdc6b11bcf60cc0b33ef/docs/evidence/cell-regression-2026-09-20.md)和[Cell MVP 范围](alpha-mvp.zh-CN.md)开始。当前用户身份与 OIDC 路径由 multi-tenant 平台负责。Gateway 提供 TLS 和路由；Connector 校验选定 Cell 实例并代理请求。

- [共享宪法](../CONSTITUTION.md)
- [RuntimePort](design/runtime-port.zh-CN.md) / [Cell adapter](design/cell-adapter.zh-CN.md) —— 内部接口与资源边界
- [平台接入配置](platform-access.md)
- [Cell Connector](cell-connector.md)
- [Cell MVP 范围与验收](alpha-mvp.zh-CN.md)
- [架构](specs/architecture.zh-CN.md)与[威胁模型](specs/threat-model.zh-CN.md)
- [Go 开发](go-development.md)
- [当前发行说明](distribution.md)与[Cell CLI 源码 README](../packages/cell-cli/README.md)
- [DSH 精确基线](../compat/dsh/README.zh-CN.md)
- [当前路线图](../ROADMAP.zh-CN.md)

已发布 npm `0.3.0-alpha.1` 及其固定 `operator.yaml` / `release.json` 描述的是该发行版。PR #93 等源码变更在单独发行前都只是候选。`archive/` 保存各版本 standalone 安装、snapshot/restore 和较早的实现/计划材料，不是当前入口或验收清单。
