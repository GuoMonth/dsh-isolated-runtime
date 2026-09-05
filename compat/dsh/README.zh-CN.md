# DSH 兼容基线

项目只支持 `dsh-v0.1.3-alpha.1`，commit
`d347e703908d0406b7a7ef80e3a0e594d86b2215`，使用 `pnpm@11.7.0` 与
[`baseline.json`](./baseline.json) 记录的 frozen lockfile digest。持久化格式与该版本绑定，
不承诺兼容更新或更旧的 session format。

`make verify-dsh` 会完整 checkout 上游并运行其测试，覆盖 browser token/cookie exchange、
`settings/describe`、remote mux 与 ready stream、Fetch GET/HEAD、Host/Origin 与跨站拒绝、
cookie 跨重启保持、session-format 拒绝、SIGTERM、dispose 和 shutdown。本地 Go 套件另行验证
透明代理、脱敏、cookie 加固、信号转发与失败清理，最后用这份精确构建的 DSH CLI 直接驱动
launcher 完成真实 browser exchange。

## Access 决策

| 候选 | 结论 |
| --- | --- |
| 直接暴露 DSH | 拒绝：标准 CLI 有意只监听 loopback，launch URL 还包含 bearer token。 |
| 纯 Gateway 配置 | 拒绝：无法持有进程内 token exchange，也无法加固返回 cookie。 |
| 独立 sidecar | 拒绝：0.1.3-alpha.1 没有跨进程注入或取得 launch token 的支持接口。 |
| Cell-local launcher | 选择：父进程观测 readiness，将 token 留在内存，并透明代理 DSH。 |

当前 GitHub release 尚无对应 npm 发布。Cell 镜像从 baseline.json 的精确源码归档和校验和构建
`build:official`，再使用上游 runtime closure 部署已编译产物，并补齐该 closure 漏列的传递 workspace peer。
不从旧 npm 版本补依赖；唯一源码修改是下面记录并校验的 Cell 设置补丁。launcher 是镜像 PID 1。

DSH 仍不提供可区分的应用 flush acknowledgement，快照保持 writer-stopped crash consistency。
上游 0.1.3 引入 session format v2 和自身迁移逻辑；本项目不维护旧版本恢复或迁移层。
凭据过滤保留固定 Envoy cookie 家族及可能伪造的无后缀名称；这些拒绝规则属于凭据隔离保证。

## 状态归属

| 状态 | 权威 | 数据快照 |
| --- | --- | --- |
| Session、附件、storage domain | data PVC 上的 DSH | 是 |
| Workspace | Cell data PVC | 是 |
| 配置 | DSH home，排除 secret/signing record | 是 |
| Provider 凭据 | Kubernetes Secret / 独立 DSH credentials mount | 否 |
| Browser signing record | 独立 DSH credentials mount | 否 |

这份映射只对精确版本成立。升级 DSH 必须建立新基线、重跑兼容门并显式决定持久化语义。

## Cell 原生设置补丁

上游 UI 仅在 loopback hostname 加载持久设置。`patches/cell-settings.patch` 在已由
Gateway OIDC 和 Cell 授权保护的 Cell hostname 上启用现有设置镜像，不新增 UI、API，
也不改变服务端授权或透明代理。`baseline.json` 分别记录源码与补丁校验和，镜像与兼容门
应用同一补丁。MVP 浏览器验收通过原生编辑器配置密钥，并验证凭据不进入数据快照。
