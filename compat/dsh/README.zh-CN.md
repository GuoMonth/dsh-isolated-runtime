# DSH 兼容基线

项目只支持 `dsh-v0.1.2-rc.1`，commit
`a66e4702047846cdaa10c66c9d3df3951f5ea70d`，使用 `pnpm@11.7.0` 与
[`baseline.json`](./baseline.json) 记录的 frozen lockfile digest。持久化格式与该版本绑定，
不承诺兼容更新或更旧的 session format。

`make verify-dsh` 会完整 checkout 上游并运行其测试，覆盖 browser token/cookie exchange、
`settings/describe`、remote mux 与 ready stream、Fetch GET/HEAD、Host/Origin 与跨站拒绝、
cookie 跨重启保持、session-format 拒绝、SIGTERM、flush 和 shutdown。本地 Go 套件另行验证
透明代理、脱敏、cookie 加固、信号转发与失败清理，最后用这份精确构建的 DSH CLI 直接驱动
launcher 完成真实 browser exchange。

## Access 决策

| 候选 | 结论 |
| --- | --- |
| 直接暴露 DSH | 拒绝：标准 CLI 有意只监听 loopback，launch URL 还包含 bearer token。 |
| 纯 Gateway 配置 | 拒绝：无法持有进程内 token exchange，也无法加固返回 cookie。 |
| 独立 sidecar | 拒绝：0.1.2 RC 没有跨进程注入或取得 launch token 的支持接口。 |
| Cell-local launcher | 选择：父进程观测 readiness，将 token 留在内存，并透明代理 DSH。 |

源码 checkout 继续作为行为基线。生产 Cell 镜像安装官方
`@deepseek-ai/dsh@0.1.2-rc.1` npm 产物，其 tarball integrity 记录于
`baseline.json`；launcher 是镜像 PID 1。
0.1.2 RC web profile 会启用实时 patch 监听，因此 launcher 通过
`node --expose-internals` 调用官方 CLI；这是 DSH 自身 HMR loader 的要求，不会暴露 launch
token。

精确 RC 不提供应用 flush acknowledgement：SIGTERM 路径在 dispose 成功、拒绝与超时后都可
使用退出码 0，外部无法区分。Phase 3.1 因此删除 `POST /quiesce`；只有 StatefulSet 设为零、
观测到零副本，并证明该 StatefulSet UID 不再拥有任何 Pod 后才允许快照。保证明确收敛为
writer-stopped crash consistency，而非 application consistency。普通 Pod 终止仍使用 launcher
的有界 HTTP/WebSocket drain 与 SIGTERM 转发。

在访问边界，launcher 会在进入 DSH 前删除身份 header 和固定 Envoy Gateway v1.9.1 的全部
默认凭据 cookie。当前名称为 `AccessToken`、`OauthHMAC`、`OauthExpires`、`IdToken`、
`RefreshToken`、`OauthNonce`、`CodeVerifier`，每个名称后都有 policy 的 8 位十六进制后缀；
无后缀旧名称也被保留为禁区。同时保留 DSH 与无关应用 cookie。

## 状态归属

| 状态 | 权威 | 数据快照 |
| --- | --- | --- |
| Session、附件、storage domain | data PVC 上的 DSH | 是 |
| Workspace | Cell data PVC | 是 |
| 配置 | DSH home，排除 secret/signing record | 是 |
| Provider 凭据 | Kubernetes Secret / 独立 DSH credentials mount | 否 |
| Browser signing record | 独立 DSH credentials mount | 否 |

这份映射只对精确版本成立。升级 DSH 必须建立新基线、重跑兼容门并显式决定持久化语义。
