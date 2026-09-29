# DSH Isolated Runtime

[English](README.md) · [安装平台](https://github.com/GuoMonth/dsh-multi-tenant/blob/main/README.zh-CN.md) · [AI 引导](AI.md)

[DSH multi-tenant](https://github.com/GuoMonth/dsh-multi-tenant) 的 Kubernetes 资源与执行层，提供固定 DSH 运行镜像、launcher 和同进程环境 Connector。**部署请从平台仓库开始**；平台已打包 Connector，并提供 npm 安装 CLI 和 Helm Chart。

DSH 固定 **0.2.0-rc.2**。每环境一个独立 PVC，挂载到 `/var/lib/dsh/data`：`workspace/` 存文件，`home/` 存用户工具和配置，`dsh/` 存原生对话及凭据。正常启停和 Pod 重建保留原卷。CPU/Memory 使用 Kubernetes requests/limits，存储仅一个申请容量参数。

## 职责

| 组件 | 负责 |
| --- | --- |
| 本仓库 | 固定运行镜像与 launcher、namespace/PVC/Sandbox 身份、创建/查询/启停/删除及核验后的访问通道 |
| [平台仓库](https://github.com/GuoMonth/dsh-multi-tenant) | OIDC、成员、用户授权、分配绑定、连接撤销、npm CLI 和安装 |
| 上游 Agent Sandbox core | 唯一控制器，将 Sandbox 调谐为 Pod/Service |
| DeepSeek Harness | 原生界面、对话、模型和工具执行、用户凭据 |

Connector 在平台进程内运行，是内部打包组件，无需单独安装 npm 依赖或部署网络服务。用户授权由平台负责，资源标签和 namespace 不能代替鉴权。

## 让 AI 帮你安装

把[平台 AI 安装指引](https://github.com/GuoMonth/dsh-multi-tenant/blob/main/packages/multi-tenant/AI.md)交给助手。它会先核对集群、OIDC、DNS/TLS、存储及发行镜像，再运行 npm 包内的安装器。开发 runtime 时阅读[本仓库 AI 引导](AI.md)和[可执行契约](docs/design/environment-contract.zh-CN.md)。

## 生命周期与边界

停止必须在健康节点上取得精确 writer 的正面退出证据。未知结果按原分配检查，卷缺失或 UID 替换时拒绝访问。删除须先证实停止，并保留 PVC、namespace 和隔离策略。自动节点分区恢复、备份、HA 和数据迁移不在当前 Alpha 保证内。

- [Connector 实现](packages/environment-connector/src/runtime.ts)与[固定模板](packages/environment-connector/src/template.ts)
- [平台 RBAC](config/runtime/cluster-role.yaml)
- [开发与检查](CONTRIBUTING.md)
- [镜像与 Connector 分发](docs/distribution.md)
- [发行版本](https://github.com/GuoMonth/dsh-isolated-runtime/releases)

MIT 许可证。
