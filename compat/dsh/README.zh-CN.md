# DSH 0.2.2-rc.3 兼容说明

[English](README.md)

运行镜像安装官方 npm 发行物，对应源码 [38a6a5c](https://github.com/deepseek-ai/deepseek-harness/tree/38a6a5c965aa99ab525161efb7cc5cf1fe62fced)。[baseline.json](baseline.json) 记录版本、包完整性、传输和数据路径；已提交的 npm 锁文件固定完整依赖。launcher 的 `/version` 和镜像标签携带同一针。runtime 0.5.0-rc.3、平台 0.12.0-rc.2 是新候选，已发布的 0.5.0-rc.2 / 0.12.0-rc.1 组合仍使用 DSH 0.2.1-rc.5。

## 复用官方能力

遵循上游[架构](https://github.com/deepseek-ai/deepseek-harness/blob/38a6a5c965aa99ab525161efb7cc5cf1fe62fced/docs/architecture.md)：受支持应用通过 profile 启动，插件经 Cordis 提供能力。消费发布制品，不 fork 应用，也不在这里重建插件树。

| 能力 | 取舍 |
| --- | --- |
| 启动与组合 | 普通 Node 启动官方 `dsh web` profile。profile、bundle、插件安装和配置重载均由上游负责。真实包启动检查后删除旧的 `--expose-internals` 要求。 |
| UI、worktree、子代理与团队 | 使用官方 profile 和 Plugin Manager 交付的能力；实验功能保留上游状态。平台不另建 Agent 循环，也不自动额外启用插件。 |
| 设置、凭证与 Session | 使用原生 Host API 和存储，不加 settings client 补丁、第二套 Session 存储或数据转换器。 |
| Unary RPC 与流式传输 | 透传原生 HTTP 和 `/api/remote.mux` WebSocket，不实现模型/工具协议、codec、流恢复或心跳策略。 |
| 组织身份 | OIDC 成员与撤权由平台负责；DSH 账号或浏览器 cookie 不能授权访问其他环境。 |
| 资源生命周期 | runtime 核对精确 owner、Sandbox/PVC/Pod 身份及写入者终止的正面证据。这些 Kubernetes 部署职责与 DSH 应用生命周期分开。 |

唯一部署 overlay 把 `workspace-controller.documentsDirectory` 指向持久工作目录。无桌面的 Linux 没有 Documents 查询，工作区布局和持久化仍由官方 controller 负责。

launcher 保留小范围 cookie 适配：官方浏览器认证仍发出 `SameSite=Strict`，没有 SameSite 配置项。外部 OIDC 回跳链需要 `Lax` 才能完成安全的顶层导航。代理为平台 HTTPS 入口补 `Secure`；cookie 值、签名、authority 校验与认证仍由原生实现负责，不重建上游认证。原生 token 仅留在进程内，保留 Host/Origin，拒绝未授权或其他 authority 的请求。

## 验证精确发行物

```sh
make verify
make verify-dsh
```

原生门禁消费已提交的 npm 依赖，启动真实 web profile，不调用模型。DSH 集成输入变化时，PR CI 在 Node 24 执行该门禁。浏览器设置、Session 与 WebSocket 检查使用 [test/rc2](../../test/rc2/README.md)，Linux 运行镜像检查使用 [test/environment](../../test/environment/README.md)。平台授权和真实 provider 验收有独立 harness。

全新安装、相同版本重启与旧数据迁移是不同证据。保持上游 Session 代际与存储原样，本候选不代表支持原地升级或降级。SIGTERM 成功退出不等于持久化 flush 确认。[runtime 契约](../../docs/zh-CN/contract.md)保留这些边界。
