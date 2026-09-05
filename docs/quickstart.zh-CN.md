# 本地 MVP 快速开始

首发支持 Linux x86_64；需要当前用户可访问的 Docker，打开浏览器还需要图形桌面。
建议按参考环境准备 4 CPU、16 GiB 内存，实际消耗取决于工作负载。基础命令需要 Bash、curl、
tar/xz、sha256sum、OpenSSL、flock 和 Git。演示会把缺少的固定版本 kind、kubectl、jq、Node、
Chromium 下载到演示私有目录，不需要 Go 编译器，不修改系统配置。

从项目 GitHub Release 下载 linux-amd64 压缩包及 SHA256SUMS，运行
`sha256sum -c SHA256SUMS`，解压并进入目录。包中的 `release.json` 记录本次验收的精确镜像。

```sh
./demo up
./demo open
```

`up` 创建独立 kind 集群、Calico、Envoy Gateway、Dex、存储和一个 DSH Cell，打印访问地址
及专用 kubeconfig。`open` 使用独立 Chromium profile 处理本地域名和测试证书，不修改 hosts
或系统证书信任。18443 和 15556 端口必须空闲，入口仅绑定 loopback。

测试登录使用 `alice@example.com` / `password`。该测试身份系统仅用于本地演示。
进入 DSH 原生引导界面后填写自己的 DeepSeek API Key、选择模型，发送“创建 hello.txt 并读回”
之类的请求。模型费用由该账户承担；本项目不提供模型服务或另一套模型配置界面。
界面保存的 key 位于 private 卷；也可以用同 namespace 的 Secret 与 `credentialsRef` 注入环境变量。

默认状态目录为 `${XDG_STATE_HOME:-$HOME/.local/state}/dsh-isolated-runtime`，可用
`DSH_DEMO_HOME` 指定另一个目录。重复 `up` 保留现有 Cell 并重新连接转发，关闭浏览器不会删数据。
本地演示不做跨版本原地迁移；需要保留的文件应在显式清理前导出。

```sh
export KUBECONFIG="${XDG_STATE_HOME:-$HOME/.local/state}/dsh-isolated-runtime/kubeconfig"
kubectl -n tenant-demo get cells
kubectl -n tenant-demo describe cell assistant
kubectl -n tenant-demo get httproutes
```

使用 Cell Conditions、对应原生对象及 Kubernetes Events 定位问题。外部 hostname 从 HTTPRoute
读取，Cell status 不重复保存。前置命令缺失、端口占用或启动失败会明确报错，私有 runtime 目录保留诊断。

## 可选快照

首次启动使用 `./demo up --snapshots`，安装参考 CSI hostpath test driver 和 snapshot controller。
普通 local-path 卷不能原地改为 CSI StorageClass，因此需要在创建 Cell 前选择。

参考 `config/samples/`：先创建 CellSnapshot，等待 `Ready`，再创建一个新 Cell，将
`storage.restoreFrom` 指向该快照，使用快照记录的精确镜像、同一个 StorageClass 和足够容量。
本地演示使用 `csi-hostpath-sc` 和 `csi-hostpath-snapclass`；示例 namespace 需改为 `tenant-demo`。

快照提供停止 writer 后的崩溃一致性，不承诺应用 flush。恢复 Cell 的身份和 private 卷全新创建，
需要授权其新的 access Role 并重新配置模型凭据。生产 CSI 和备份生命周期由集群管理员负责。

## 清理与已有集群

`./demo down` 会删除本演示集群及其中的全部数据，包括 disposable 集群内 Retain 的 PVC；
下载的工具保留以供复用。它不切换或删除其他 Kubernetes context。

已有集群可使用 `config/default` 安装核心资源，或用 `config/browser` 安装推荐的可信浏览器访问；
`config/snapshots` 增加快照能力。`config/metrics` 是浏览器/快照安装可选的 Kustomize component。
先在自有 overlay 配置域名、TLS、OIDC 和路由资格，再应用配置；Kubernetes、Gateway 和 CSI
是外部前置能力，Operator 不安装或管理它们。

仅支持当前精确 DSH 基线和 linux/amd64；不承诺历史 API、跨版本恢复、HA、生产容量、多集群或企业策略。
