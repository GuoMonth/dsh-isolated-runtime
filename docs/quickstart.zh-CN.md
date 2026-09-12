# 本地 MVP 快速开始

支持 Linux x86_64 和 Apple Silicon macOS，请使用原生 arm64 终端。
Mac 需要安装并启动 Apple Silicon 版 Docker Desktop，容器在其 Linux arm64 虚拟机中运行。
Docker 必须对当前用户可用，`demo open` 需要图形桌面。CI 参考环境为 4 CPU、16 GiB 内存，
实际消耗取决于工作负载。需要 Bash、curl、tar、OpenSSL 和 Git；Linux 还需要 sha256sum/flock，
macOS 使用系统自带 shasum/Perl。脚本会私有下载固定版本的原生 kind、kubectl、jq、Node 和
Chromium，无须 Go 编译器、Homebrew 或 Rosetta，不修改系统 hosts 或全局证书信任。

从 [GitHub Releases](https://github.com/GuoMonth/dsh-isolated-runtime/releases) 下载对应平台的压缩包和 `SHA256SUMS`。
Apple Silicon 使用：

```sh
archive=dsh-isolated-runtime-v0.1.2-darwin-arm64.tar.gz
grep -F "  $archive" SHA256SUMS | shasum -a 256 -c -
tar -xzf "$archive"
cd "${archive%.tar.gz}"
```

Linux x86_64 选择 `dsh-isolated-runtime-v0.1.2-linux-amd64.tar.gz`，校验管道使用
`sha256sum -c -`。公开校验和文件包含两个安装包，只选取已下载文件对应的一行。
包内 `release.json` 记录运行架构及精确镜像 digest。

```sh
./demo up
./demo open
```

`up` 创建独立 kind 集群、Calico、Envoy Gateway、Dex、存储和一个 DSH Cell，打印访问地址
及专用 kubeconfig。`open` 使用独立 Chromium profile 处理本地域名和测试证书，不修改 hosts
或系统证书信任。18443 和 15556 端口必须空闲，入口仅绑定 loopback。

测试登录使用 `alice@example.com` / `password`。该测试身份系统仅用于本地演示。
进入 DSH 原生引导界面后确认提示并填写自己的 DeepSeek API Key。点击 **Choose workspace →
Edit path**，填写 `/var/lib/dsh/data/workspace`，按 Enter 后点击 **Open**。
选择模型，发送“创建 hello.txt 并读回”
之类的请求。模型费用由该账户承担；本项目不提供模型服务或另一套模型配置界面。
界面保存的 key 位于 private 卷；也可以用同 namespace 的 Secret 与 `credentialsRef` 注入环境变量。

macOS 默认状态目录为 `~/Library/Application Support/DSH Isolated Runtime`，Linux 为
`${XDG_STATE_HOME:-$HOME/.local/state}/dsh-isolated-runtime`。显式 `XDG_STATE_HOME` 在两种系统上均优先，
也可用 `DSH_DEMO_HOME` 指定另一个目录。重复 `up` 保留现有 Cell 并重新连接转发，关闭浏览器不会删数据。
本地演示不做跨版本原地迁移；需要保留的文件应在显式清理前导出。

v0.1.2 将 DSH 升级到 0.1.5-rc.2，使用 V3 会话格式。上游迁移受支持的旧日志时保留原文件，
但升级后的会话不能由旧版 DSH 读取。已有 Cell 更换镜像前应保留备份；跨版本 CellSnapshot
恢复会被拒绝。试用新版本可另设 `DSH_DEMO_HOME`，并先停止旧 demo 的浏览器和端口转发，
因为两套环境使用相同的本地端口。当前没有自动原地升级或降级 demo 的命令。

```sh
if [ "$(uname -s)" = Darwin ] && [ -z "${XDG_STATE_HOME:-}" ]; then
  export DSH_DEMO_HOME="${DSH_DEMO_HOME:-$HOME/Library/Application Support/DSH Isolated Runtime}"
else
  export DSH_DEMO_HOME="${DSH_DEMO_HOME:-${XDG_STATE_HOME:-$HOME/.local/state}/dsh-isolated-runtime}"
fi
export PATH="$DSH_DEMO_HOME/tools/bin:$PATH"
export KUBECONFIG="$DSH_DEMO_HOME/kubeconfig"
kubectl -n tenant-demo get cells
kubectl -n tenant-demo describe cell assistant
kubectl -n tenant-demo get httproutes
```

使用 Cell Conditions、对应原生对象及 Kubernetes Events 定位问题。外部 hostname 从 HTTPRoute
读取，Cell status 不重复保存。前置命令缺失、端口占用或启动失败会明确报错，私有 runtime 目录保留诊断。

## 可选快照

首次启动使用 `./demo up --snapshots`，安装参考 CSI hostpath test driver 和 snapshot controller。
普通 local-path 卷不能原地改为 CSI StorageClass，因此需要在创建 Cell 前选择。

先按上文设置专用 kubeconfig，然后执行：

```sh
kubectl -n tenant-demo apply -f - <<'YAML'
apiVersion: dsh.isolated.io/v1alpha1
kind: CellSnapshot
metadata: {name: assistant-backup}
spec:
  cellRef: {name: assistant}
  volumeSnapshotClassName: csi-hostpath-snapclass
YAML
kubectl -n tenant-demo wait cellsnapshot assistant-backup --for=condition=Ready --timeout=720s

cell_image="$(jq -r .images.cell release.json)"
kubectl -n tenant-demo apply -f - <<YAML
apiVersion: dsh.isolated.io/v1alpha1
kind: Cell
metadata: {name: restored}
spec:
  image: $cell_image
  storage:
    size: 1Gi
    storageClassName: csi-hostpath-sc
    retentionPolicy: Retain
    restoreFrom: {name: assistant-backup}
YAML
kubectl -n tenant-demo wait cell restored --for=condition=Ready --timeout=300s
restored_uid="$(kubectl -n tenant-demo get cell restored -o jsonpath='{.metadata.uid}')"
kubectl -n tenant-demo create rolebinding restored-access --role="cell-$restored_uid-access" \
  --user='https://dex.dsh-system.svc:15556/dex#CglhbGljZS1zdWISBWxvY2Fs'
kubectl -n tenant-demo get httproute "cell-$restored_uid" -o jsonpath='{.spec.hostnames[0]}'
```


在 `demo open` 打开的 Chromium 中访问 `https://<输出的 hostname>:18443`，重新配置模型凭据，
选择恢复出的会话继续使用。原 Cell 在快照完成后自动恢复运行。

快照提供停止 writer 后的崩溃一致性，不承诺应用 flush。恢复 Cell 的身份和 private 卷全新创建，
需要授权其新的 access Role 并重新配置模型凭据。生产 CSI 和备份生命周期由集群管理员负责。

## 清理与已有集群

`./demo down` 会删除本演示集群及其中的全部数据，包括 disposable 集群内 Retain 的 PVC；
下载的工具保留以供复用。它不切换或删除其他 Kubernetes context。

已有集群可使用 `config/default` 安装核心资源，或用 `config/browser` 安装推荐的可信浏览器访问；
`config/snapshots` 增加快照能力。`config/metrics` 是浏览器/快照安装可选的 Kustomize component。
先在自有 overlay 配置域名、TLS、OIDC 和路由资格，再应用配置；Kubernetes、Gateway 和 CSI
是外部前置能力，Operator 不安装或管理它们。

仅支持当前精确 DSH 基线；提供 Linux amd64/arm64 镜像及 Linux x86_64/Apple Silicon 主机安装包；不承诺历史 API、跨版本恢复、HA、生产容量、多集群或企业策略。

CI 在原生 Linux amd64/arm64 上验证完整的确定性 DSH 链路，并在 macOS arm64 上验证实际工具、
证书、进程归属和 Chromium 配置生命周期。完整 Docker Desktop 端到端测试及真实模型测试留给
维护者在 pre-release 发布后完成，发布证据不会把这两项标记为已通过。
