# 隔离运行时项目宪法入口

本仓库与 dsh-multi-tenant 共用 [DSH 平台与运行时项目宪法](https://github.com/GuoMonth/dsh-multi-tenant/blob/main/CONSTITUTION.md)。共享原则只在该来源维护；本文件记录运行时的适用边界，不复制一份通用契约。

- 架构目标是 Kubernetes 唯一后端的 AgentEnvironment。本地有限接入已通过，W1 采用上游 core `Sandbox`，不增加同义产品 CRD；当前源码仍实现 `Cell`。不承诺 Process/Docker 产品运行后端或多后端兼容。
- namespace 是管理员配置的基础设施 scope，不等同 OIDC tenantId；集成仍预配置 per-user namespace，不建设自动 namespace 租户系统。平台 owner 授权是唯一用户归属权威；runtime 只保存不可变 owner 关联用于资源匹配，不实现 OIDC。
- 平台与 runtime 保持职责隔离，但不要求平台直接管理 Kubernetes 对象：runtime 可以通过薄资源契约管理Pod、PVC、Service 与策略（当前通过 StatefulSet）。
- 固定当前验证的 DSH/镜像/源码组合；可随时破坏 API、配置或状态格式，不承诺历史兼容、升级、迁移或无感恢复。已发布产物不被本次文档修改。
- 资源操作、观测和清理由运行时负责。失败尽早暴露并返回 AI 可解读的结构化诊断；不把 API 接受、超时或记录消失当作执行完成。
- W1 移除产品 Process/Docker 后端、旧 standalone 启动器与 snapshot/restore；保留 Pod 内原生子进程、OCI 镜像构建及 kind 所用 Docker 底座。W2 才增加明确 Running/Stopped 与正常停止/启动，沿用 data/private 两个 PVC 身份并保留两卷；不承诺节点分区 fencing，范围限于健康节点上的正常停止。
- 不为无限期重试增加永久终态 CR、通用退役引擎、Node 控制面或自动修复系统。保留当前 ownership/隔离底线；无法证明停止就不复用旧数据。
- 备份、热池和自动 idle 暂不实现。W3 联合验证真实授权、持久 HOME 与性能后再发行。
- 数据/凭据删除需要明确目标、范围与授权；允许破坏性变更不授权静默清空数据。

当前方向与 W1/W2/W3 设计以[平台 AgentEnvironment 设计](https://github.com/GuoMonth/dsh-multi-tenant/blob/main/docs/design/agent-environment.zh-CN.md)和[主 Issue #104](https://github.com/GuoMonth/dsh-multi-tenant/issues/104)为准。仓库内旧 RuntimePort/Cell adapter 文档仅描述 Cell 实现或历史方案，不覆盖新设计，也不代表目标已实现。

English: the linked shared constitution governs this repository. Target a Kubernetes-only AgentEnvironment through a narrow runtime resource contract; upstream core `Sandbox` is selected for W1 after the bounded local trial, while current source still implements `Cell`. Namespace is administrator-configured infrastructure scope, not OIDC tenant identity; use preconfigured per-user namespaces, with platform authorization as the sole owner authority and immutable runtime ownership links only for resource matching. Remove Process/Docker product backends, standalone launch and snapshot/restore in W1. Keep Pod child processes, OCI image builds and Docker as kind's substrate. Add normal stop/start in W2, retaining both data/private PVC identities, within healthy-node normal-stop scope and without partition fencing. W3 jointly validates real authorization, persistent HOME/XDG auth and performance; target HOME/XDG auth in private storage and DSH sessions/files in data, after verifying exact client paths. Current HOME remains in data and only the DSH-designated credential is in private. Backups, hot pools and automatic idle are deferred. Preserve explicit ownership/data-deletion boundaries.
