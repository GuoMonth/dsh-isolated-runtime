# 隔离运行时项目宪法入口

本仓库与 dsh-multi-tenant 共用 [DSH 平台与运行时项目宪法](https://github.com/GuoMonth/dsh-multi-tenant/blob/main/CONSTITUTION.md)。共享原则只在该来源维护；本文件记录运行时的适用边界，不复制一份通用契约。

- 当前实现范围是 Kubernetes Cell；平台通过中立内部接口消费，不向上层暴露 Kubernetes 资源操作。正式多后端兼容等待第二个真实需求。
- 固定当前验证的 DSH/镜像/源码组合；可随时破坏 API、配置或状态格式，不承诺历史兼容、升级、迁移或无感恢复。已发布产物不被本次文档修改。
- 资源操作、观测和清理由运行时负责。失败尽早暴露并返回 AI 可解读的结构化诊断；不把 API 接受、超时或记录消失当作执行完成。
- 不为无限期重试增加永久终态 CR、通用退役引擎、Node 控制面或自动修复系统。保留当前 ownership/隔离底线；无法证明停止就不复用旧数据。
- 数据/凭据删除需要明确目标、范围与授权；允许破坏性变更不授权静默清空数据。

当前目标见 [MVP 范围](docs/alpha-mvp.zh-CN.md)，内部设计见 [RuntimePort](docs/design/runtime-port.zh-CN.md) 和 [Cell adapter](docs/design/cell-adapter.zh-CN.md)。这些文档描述开发方向，不代表实现已经通过验收。

English: the linked shared constitution governs this repository. Implement Cell first; keep a neutral internal boundary, fail fast with structured diagnostics, allow breaking changes without historical compatibility or recovery promises, and preserve explicit ownership/data-deletion boundaries. Do not build infrastructure for unbounded guarantees.
