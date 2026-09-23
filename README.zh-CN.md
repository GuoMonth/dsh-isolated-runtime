# dsh-isolated-runtime

原生 DeepSeek Harness 的 Kubernetes Cell 生命周期与隔离运行时。[多租户平台](https://github.com/GuoMonth/dsh-multi-tenant/blob/main/README.zh-CN.md) 负责 OIDC、成员、用户协议和会话；本仓库负责 Cell 资源、镜像、精确实例校验与受限内部 Connector。DSH 负责原生 Web 和工具。

[English](README.md)

**当前是 Cell MVP alpha。** 固定版本下的双用户 OIDC + Cell、真实模型文件操作已通过[集成回归](https://github.com/GuoMonth/dsh-multi-tenant/blob/4ba252765bccb41314c0bdc6b11bcf60cc0b33ef/docs/evidence/cell-regression-2026-09-20.md)。这批已验收的 Linux/amd64 Cell/Operator 镜像已按相同 digest 公开于 [v0.3.0-alpha.1](https://github.com/GuoMonth/dsh-isolated-runtime/releases/tag/v0.3.0-alpha.1)；平台 npm 独立发行。当前 `main` 源码已加入固定模板 `cell-mvp-v1`，并通过 2026-09-22 内部候选回归（[报告](https://github.com/GuoMonth/dsh-multi-tenant/blob/main/docs/evidence/cell-mvp-2026-09-22.md)）。此源码候选尚未重新发行：公开 runtime npm `0.3.0-alpha.1` 及其 Cell/Operator 镜像仍对应旧发行。部署新模板候选请使用[平台候选指南](https://github.com/GuoMonth/dsh-multi-tenant/blob/main/docs/reference/cell-mvp-v1-candidate.zh-CN.md)，不要用下方已发布 npm 清单命令。后续迭代允许破坏性变更；已发行制品身份保持不变。

**架构方向：**下一阶段目标是 K8s 唯一后端的 AgentEnvironment；当前源码/包仍使用 Cell，源码调整尚未落地。W1 移除 Process/Docker 产品运行后端、standalone 启动与 snapshot/restore。详见[权威设计](https://github.com/GuoMonth/dsh-multi-tenant/blob/main/docs/design/agent-environment.zh-CN.md)和[Issue #104](https://github.com/GuoMonth/dsh-multi-tenant/issues/104)。工作区 Pod 内子进程、OCI 镜像构建及 kind 的 Docker 底座保留。

## 固定发行边界

依赖的 DSH 明确为 **0.1.5-rc.2**，源码 **`fb2c4b9e698e30edb738bca4cf0618587db7d203`**。每次发行锁定可公开拉取的 Cell、Operator 镜像 `@sha256` digest，并在平台 `cell-release.json` / runtime `release.json` 中记录匹配的运行时源码与 DSH 身份；实际部署的平台镜像也固定 digest。npm `latest` 只用于安装时选择包，不让运行中的镜像标签或 DSH 版本范围漂移。

允许破坏性更新：新迭代明确新的固定组合，按需修改配置/状态要求并验证受影响链路，不要求兼容层、历史升级或迁移承诺。已发布制品身份不改写。公开的 [v0.3.0-alpha.1 清单](https://github.com/GuoMonth/dsh-isolated-runtime/releases/download/v0.3.0-alpha.1/release.json) 已记录本次 Linux/amd64 的 Cell/Operator 组合。空值或不匹配 digest 仍会阻止平台发行。


## 与平台配合

管理员准备 K8s、执行 NetworkPolicy 的 CNI、存储、预配置的基础设施 scope namespace、Gateway API 和 DNS/TLS。下方命令安装已发行 runtime `0.3.0-alpha.1` 并渲染该发行版的固定清单，不会部署较新的 `cell-mvp-v1` 源码候选；新候选请按[平台候选指南](https://github.com/GuoMonth/dsh-multi-tenant/blob/main/docs/reference/cell-mvp-v1-candidate.zh-CN.md)操作。已发行版清单固定了当时验收的公开镜像；审阅并设置环境域名后部署：

固定 platform 模式部署要求平台 Pod 位于 Operator 的 system namespace（默认 `dsh-system`），并带 `dsh.isolated.io/access: platform` 标签；详见[平台启动指南](https://github.com/GuoMonth/dsh-multi-tenant/blob/main/docs/reference/quickstart.zh-CN.md)。使用 `latest` 选择包后，先核对 runtime 发行与平台 `cell-release.json` 匹配，再应用清单。

```bash
npx dsh-isolated-runtime@latest release
npx dsh-isolated-runtime@latest manifests > /private/operator-rendered.yaml
# 审阅并替换示例域名；Operator 镜像已固定为公开 digest。
kubectl apply --server-side -f /private/operator-rendered.yaml
```

`--access-mode=platform` 不创建 standalone 用户认证器或直达 Cell 的 HTTPRoute。授权由平台执行，再通过 Connector 访问已校验的具体实例。见[平台接入配置](docs/platform-access.md)及联动的[内测启动指南](https://github.com/GuoMonth/dsh-multi-tenant/blob/main/docs/reference/quickstart.md)。

平台 npm 已发布，Node 24+ 的入口是：

```bash
npx dsh-multi-tenant@latest start --config /private/config.json
```

进程必须能直达 API 和 Pod IP，推荐在集群内运行；它不自动建集群。运行时 npm `0.3.0-alpha.1` 只交付 Cell 版本和部署清单；已发布 npm 包不提供旧 standalone `start/up/stop/uninstall` 命令，也不创建本地集群；旧实现仍以历史源码保留在 `packages/cli`。

| 层 | 权威边界 |
| --- | --- |
| multi-tenant | OIDC、可信成员、父子会话、分配意图与协议准入 |
| isolated-runtime | Cell Operator、镜像、资源生命周期/归属、已校验访问通道 |
| DSH | 原生应用会话、工具、模型调用和应用私有状态 |

当前回归使用一个集群、上层单副本和固定版本。未知写结果查原 key；接受删除不等于 writer 已停止。见[项目宪法](CONSTITUTION.md)、[RuntimePort](docs/design/runtime-port.zh-CN.md)和[MVP 边界](docs/alpha-mvp.zh-CN.md)。

## 真实集成画面

平台 OIDC 后的原生 DSH，运行在本仓库管理的 Cell；deepseek-flash 完成文件写入/读取及附件读取。项目不提供模型凭据。

![Cell 中的真实模型及文件验证](docs/images/cell-native.png)

![展开的原生工具调用](docs/images/cell-tools.png)

[完整证据与限制](https://github.com/GuoMonth/dsh-multi-tenant/blob/4ba252765bccb41314c0bdc6b11bcf60cc0b33ef/docs/evidence/cell-regression-2026-09-20.md) · [发行分工](docs/distribution.md) · [开发贡献](CONTRIBUTING.md)。

## 历史 standalone 发行

已发布 `v0.2.0-alpha.1` 是 standalone 本地启动器，不是本次集成 alpha。请使用它的[版本化说明](https://github.com/GuoMonth/dsh-isolated-runtime/tree/v0.2.0-alpha.1)。当前 npm `0.3.0-alpha.1` 在 `latest` 通道，以 Cell 清单入口替代旧启动器。旧实现仍以历史源码保留在 `packages/cli`；这是破坏性更新，不提供自动迁移。

[文档索引](docs/README.md) · [DSH 精确基线](compat/dsh/README.md) · [架构](docs/specs/architecture.md)。许可证见 [LICENSE](LICENSE)。
