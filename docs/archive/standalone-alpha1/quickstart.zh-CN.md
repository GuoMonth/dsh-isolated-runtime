# 本地安装

目标版本：**v0.2.0-alpha.1**。当前分支正在准备 alpha；下面涉及下载该版本的命令在发布后可用。
已有 v0.1.2 安装包继续使用其原有命令。

## 前置准备

支持 Linux x86_64、Apple Silicon macOS 原生 arm64 终端。提前安装并启动 Docker，
Mac 使用 Docker Desktop；当前用户必须能够访问 Docker。
需要 Bash、curl、tar、OpenSSL、Git；Linux 还需要 sha256sum/flock，Mac 使用 shasum/Perl。
打开浏览器需要图形桌面。CI 参考配置为 4 CPU、16 GiB 内存，不是实测最低要求。

不用预先安装 Kubernetes、kind、kubectl、Go 或 DSH。安装器管理私有工具和独立 kind 集群。
18443、15556 端口需要空闲；不修改 hosts 或系统证书信任。
[Docker 安装说明](https://docs.docker.com/get-started/get-docker/)

## npm 一行入口

npm 发布后，准备 Node.js 22+ 和 npm：

```sh
npx dsh-isolated-runtime@0.2.0-alpha.1 start
```

无图形桌面使用 `start --no-open`；首次创建 Cell 前可选择 `start --snapshots`。
npm 只下载并校验同一份固定版本发行包，不在本机编译镜像或安装 Docker。
首次启动仍需要联网下载工具和镜像。npm 包名在实际取得发布权限前为拟定名称。

后续操作沿用同一 npm 前缀。浏览器打开时，可以在另一个终端运行
`npx dsh-isolated-runtime@0.2.0-alpha.1 credentials` 私下查看登录信息，
或运行 `npx dsh-isolated-runtime@0.2.0-alpha.1 status --json`、
`npx dsh-isolated-runtime@0.2.0-alpha.1 stop`。
下面的 ./dsh-runtime 命令是直接下载方式的对应写法。

## 直接下载

这种方式不要求预装 Node。从
[发行页](https://github.com/GuoMonth/dsh-isolated-runtime/releases/tag/v0.2.0-alpha.1)
下载对应压缩包和 SHA256SUMS。Apple Silicon 使用：

```sh
archive=dsh-isolated-runtime-v0.2.0-alpha.1-darwin-arm64.tar.gz
grep -F "  $archive" SHA256SUMS | shasum -a 256 -c -
tar -xzf "$archive"
cd "${archive%.tar.gz}"
./dsh-runtime doctor
./dsh-runtime up
./dsh-runtime open
```

Linux 使用 `linux-amd64` 包，校验命令换成 `sha256sum -c -`。必须先校验再解压。
包内 release.json 记录源码与镜像的精确身份。

## 首次登录

在本机私下运行 `./dsh-runtime credentials` 查看随机生成的登录信息。
不要把输出发给 AI 或贴进 issue。每次新安装生成独立密码和 OIDC 客户端密钥，
本地文件权限为私有，Dex 配置通过 Kubernetes Secret 提供。

进入 DSH 原生引导界面后填写自己的模型 API Key。选择 workspace，编辑路径为
`/var/lib/dsh/data/workspace`，然后让 DSH 创建文件并读回。模型费用由你的账户承担。
模型密钥保存在 private 卷，不包含在快照里。

## 停止、恢复与卸载

```sh
./dsh-runtime status --json
./dsh-runtime doctor --json
./dsh-runtime stop
./dsh-runtime up
# 下面会删除数据，导出所需文件并明确确认后才执行。
./dsh-runtime uninstall --yes
```

stop 停止浏览器、转发进程和本安装所属的 kind 节点，保留数据；关闭浏览器也不会删除数据。
uninstall --yes 删除整个所属集群，包括其中设置 Retain 的 PVC。
不卸载 Docker，也不删除其他集群；下载工具与发行包缓存保留。

Mac 默认状态目录为 `~/Library/Application Support/DSH Isolated Runtime`；
Linux 为 `${XDG_STATE_HOME:-$HOME/.local/state}/dsh-isolated-runtime`。
两种系统都优先使用显式 XDG_STATE_HOME；也可以设置 DSH_RUNTIME_HOME。
旧 DSH_DEMO_HOME 仍兼容，两个变量不一致时拒绝操作。
历史集群名和 namespace 标识保留以识别资源归属，不代表可以随意删除数据。

不会自动复用、升级或删除其他版本的状态。先用旧版本导出文件，再决定是否卸载，
或停止旧安装后选用独立状态目录。DSH V3 会话不支持降级，跨版本快照恢复不支持。

## 自有集群

通过 config/default、config/browser、config/snapshots 安装；管理员提供网络、存储、
域名、TLS、OIDC 与访问授权。本地身份系统和 CA 不等于生产环境身份与证书方案。
[快照示例](https://github.com/GuoMonth/dsh-isolated-runtime/blob/v0.2.0-alpha.1/config/samples/dsh_v1alpha1_cellsnapshot.yaml)中的 hostpath CSI 是本地参考驱动。
此 alpha 不承诺 k3d/k3s、Windows 或 Intel Mac 安装支持。

[AI 运行手册](ai/local-run.md) | [发行与网络依赖](https://github.com/GuoMonth/dsh-isolated-runtime/blob/v0.2.0-alpha.1/docs/distribution.md)

完整 Mac Docker Desktop 与真实模型验收由维护者在 alpha 发布后进行，
不能用 Linux 确定性测试结果替代。
