# DSH Isolated Runtime

Kubernetes-only 的 DSH 运行时。平台负责登录、授权和归属绑定；本仓库提供窄 Connector、单 PVC 固定模板与 DSH launcher，上游 Agent Sandbox core 是唯一控制器。

固定 DSH **0.2.0-rc.2** / `639ed015397290b3745d163aafe02ffee4aa3f84`。每个环境只有一个外部 PVC，挂载 `/var/lib/dsh/data`，包含 `workspace`、`home`、`dsh`；仅一个容量值，CPU/Memory 使用原生 requests/limits。

- [契约](docs/design/environment-contract.zh-CN.md)；入口 `createAgentEnvironmentRuntime(options)` 位于 [Connector](packages/environment-connector/src/runtime.ts)。
- [固定模板](packages/environment-connector/src/template.ts)、[平台 RBAC](config/runtime/cluster-role.yaml)、[开发检查](CONTRIBUTING.md)、[制品说明](docs/distribution.md)。
- [MVP 验收 Issue](https://github.com/GuoMonth/dsh-multi-tenant/issues/104)。

没有独立 Process/Docker 后端、自有 Cell operator、安装 CLI 或旧状态迁移。停止必须确认精确 writer 正常退出与节点健康；未知结果先查询原分配。删除默认保留 PVC、namespace 与隔离策略；不承诺节点分区自动恢复。

当前为本地 RC 候选，不是已发布版本；平台安装制品固定消费 Connector、镜像及上游 core 资源。[English](README.md)。
