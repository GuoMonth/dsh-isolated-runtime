# DSH 兼容基线

Phase 0 只支持 `dsh-v0.1.2-alpha.4`，commit
`4e84901e6471b79ec0338099867ebb4606d12bb5`，使用 `pnpm@11.7.0` 与
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
| 独立 sidecar | 拒绝：alpha.4 没有跨进程注入或取得 launch token 的支持接口。 |
| Cell-local launcher | 选择：父进程观测 readiness，将 token 留在内存，并透明代理 DSH。 |

Phase 0 的 launcher 是内部契约实验；生产 PID 1 打包与生命周期属于 Phase 1。

## 状态归属

| 状态 | 权威 | 数据快照 |
| --- | --- | --- |
| Session、附件、storage domain | data PVC 上的 DSH | 是 |
| Workspace | Cell data PVC | 是 |
| 配置 | DSH home，排除 secret/signing record | 是 |
| Provider 凭据 | Kubernetes Secret / 独立 DSH credentials mount | 否 |
| Browser signing record | 独立 DSH credentials mount | 否 |

这份映射只对精确版本成立。升级 DSH 必须建立新基线、重跑兼容门并显式决定持久化语义。
