# DSH Isolated Runtime

[English](README.md)

为自托管、多用户 [DeepSeek Harness](https://github.com/deepseek-ai/deepseek-harness) 提供推荐的 Kubernetes 运行层，维护 [dsh-multi-tenant](https://github.com/GuoMonth/dsh-multi-tenant) 使用的运行镜像、launcher 和同进程 Connector。

推荐的 [Kubernetes 参考部署](https://github.com/GuoMonth/dsh-multi-tenant/blob/main/docs/zh-CN/installation.md) 在参考平台镜像中单独安装精确 Connector。平台提供 CLI 和 Helm Chart。已发布 RC 组合为 runtime **0.5.0-rc.3**、平台 **0.12.0-rc.2**、DSH **0.2.2-rc.3**。[验证报告](https://github.com/GuoMonth/dsh-isolated-runtime/releases/download/v0.5.0-rc.3/validation.md)记录精确制品、已接受的 Picomatch 问题及模型/工具未测边界。不做兼容性承诺，部署时固定精确版本和制品。

当前源码以 runtime **0.5.0-rc.4** 和 DSH **0.2.2-rc.5** 为目标。[DSH 兼容说明](compat/dsh/README.zh-CN.md)

## runtime 做什么

runtime 为已授权用户创建固定 Kubernetes 资源，核对 owner 和 UID，处理显式停止/启动/删除，并转发原生 DSH 流量。上游 Agent Sandbox core 是唯一的 Pod/Service 控制器；平台负责 OIDC 和用户授权。

每环境一个 PVC，挂载到 `/var/lib/dsh/data`，包含 `workspace/`、`home/`、`dsh/`。正常启停和 Pod 重建保留文件、对话、配置及工具凭据，删除环境保留用户存储。状态和失败处理见[契约](docs/zh-CN/contract.md)。

RC 支持 Linux/amd64、单节点、平台单副本和可信组织成员，仅支持全新安装。节点分区恢复、备份、高可用和数据迁移另行建设。

## 文档

- [中文指南](docs/zh-CN/index.md) · [English guides](docs/en/index.md)
- [开发](docs/zh-CN/development.md) · [发布](docs/zh-CN/releasing.md)
- [源码与镜像发行](https://github.com/GuoMonth/dsh-isolated-runtime/releases)
- [平台安装](https://github.com/GuoMonth/dsh-multi-tenant/blob/main/README.zh-CN.md) · [AI 助手说明（英文）](AI.md)

MIT 许可证。
