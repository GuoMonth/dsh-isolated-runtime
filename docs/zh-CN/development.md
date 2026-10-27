# 开发 DSH runtime

[English](../en/development.md) · [文档目录](index.md)

使用 Go 1.27.1、Node 24+ 和已提交的 npm 锁文件。Make 和 launcher 镜像构建固定 Go 工具链。`dev-run` 是可选的本机工具链包装器，也可使用标准 Go/Node 安装。

```sh
npm ci --prefix packages/environment-connector
make docs-check
make verify
make verify-dsh
```

`make verify` 构建 Go 和 Connector，运行 Go race/Connector 测试、vet、源码标准和文档检查。`make verify-dsh` 安装固定 DSH 包，校验精确包身份与 launcher 传输。CI 运行源码/文档标准及工作流语法检查。

`make docs-check` 检查路径、锚点、语言配对及命令/版本一致性。将 `DSH_PLATFORM_DOCS` 和 `DSH_RUNTIME_DOCS` 设置为配套工作区路径，可检查跨仓库链接。技术测试 README 使用英文，公开指南提供两种语言。

launcher 回收子进程、关闭代理传输和已接管的连接。每个 goroutine 都要有明确的生命周期和退出条件，强制 SIGKILL 不能计为环境成功停止。Go 测试使用 race detector 和 goleak；可选 pprof 采集留在私有目录。

真实 Kubernetes 生命周期、卷和网络检查见 [test/environment（英文）](../../test/environment/README.md)。浏览器与上游包验证见 [test/rc2（英文）](../../test/rc2/README.md)，精确 DSH baseline 与原生设置行为见 [compat/dsh（英文）](../../compat/dsh/README.md)。使用一次性夹具，由一个操作者负责集群写入。模型/工具授权、OIDC 和双用户验收使用[平台](https://github.com/GuoMonth/dsh-multi-tenant/blob/main/README.zh-CN.md)测试入口。

提交使用 `git commit -s`。PR 记录实际命令、制品身份和未测项。包及镜像准备见[发布](releasing.md)。
