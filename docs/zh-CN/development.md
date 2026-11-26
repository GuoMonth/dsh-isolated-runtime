# 开发 DSH runtime

[English](../en/development.md) · [指南](index.md)

使用 Go 1.27.4 和 Node 24.22.3 及以上的 24.x 版本，或 26.13.1 及以上版本。[.node-version](../../.node-version) 固定开发用 Node。Corepack 读取根 [package.json](../../package.json) 的 `packageManager)。[pnpm 锁文件](../../pnpm-lock.yaml) 固定工具链。`go.mod` 为 Make、CI 和构建镜像选择 Go 版本。工作负载构建按摘要固定两个工具链镜像。

```sh
pnpm install --frozen-lockfile
pnpm fmt:check
pnpm lint
make docs-check
make verify
make verify-dsh
```

根 pnpm workspace 负责 Connector 开发。`images/environment` 保留在 workspace 之外，npm 锁文件描述官方已发布 DSH 的运行依赖，`make verify-dsh` 通过 `npm ci` 消费它。[Connector 打包脚本](../../hack/pack-environment-connector.mjs) 使用冻结的 pnpm workspace 构建，再通过 `npm pack` 提供平台消费的 tarball。

TypeScript 和 JavaScript 使用两空格、单引号、无语句末尾分号，多行列表保留尾逗号。TypeScript 相对导入使用 `.ts`，编译器改写产物导入。`pnpm fmt` 应用固定版本 Oxlint 风格修复和 gofmt；`pnpm fmt:check` 只检查 JavaScript/TypeScript，不写入。超过 140 列且无法安全自动修复的行需要手动换行。`make lint` 还检查 Go 格式、UTF-8、LF、空白、JSON、Shell、JavaScript 和 Python 语法，并使用 [.golangci.yml](../../.golangci.yml) 运行固定版本的 `golangci-lint`，执行 `goimports` 和 HTTP 响应体关闭检查。

`pnpm lint` 执行官方严格 TypeScript 规则。[lint-baseline.json](../../hack/lint-baseline.json) 按文件、规则、所属语句和声明及出现次数冻结存量诊断；新增或迁移诊断会失败。`pnpm lint --prune` 只能删除已解决的例外；PR CI 对照受信任的基线提交，拒绝扩大例外。本地 lint 前先获取 `origin/main`；`LINT_BASE_REF` 可以指定受信任的基线 SHA。`pnpm lint:raw` 同时展示未解决的债务，`pnpm test:tooling` 验证门禁的拒绝路径。

`make verify` 执行源码、格式和严格类型检查，构建 Go 与 Connector，运行 race 和 Connector 测试、`go vet`、文档检查和安全扫描。CI 在 Node 24.22.3 和 26.13.1 上运行完整门禁，并检查工作流语法。DSH 集成输入变化时，两条任务还会运行 `make verify-dsh`，安装精确 npm 依赖，并用开启竞态检测的 Go 测试验证 launcher 传输，以及 Session 的创建、流式订阅、导出和重启恢复。集群与发布流程保持手动执行。

`make docs-check` 检查路径、锚点、双语配对以及命令和版本一致性。跨仓链接检查使用 `DSH_PLATFORM_DOCS` 和 `DSH_RUNTIME_DOCS` 指定配套 checkout。技术 harness 的 README 使用英文，公开指南提供双语。

身份按角色使用独立 TypeScript 类型：owner 的 tenant/principal、allocation、namespace、Sandbox/PVC/Pod UID 和资源修订号。外部值先经过导出的解析器，保留精确字符串。Kubernetes JSON 和 watch 事件在使用前解析，保留未知字段供精确模板比较；不完整列表不能证明资源不存在。传输和领域错误保留内部 cause，对外诊断只输出固定契约；写响应失败不代表允许重放。

分配操作可能收到附带平台控制元数据的绑定记录。构造资源引用前只提取 allocationKey 和 owner，不能把平台状态展开进严格身份。首次创建与只读发现都必须返回同一精确引用，不重放已接受的写入。

解析 Kubernetes 原始响应，不能假定 kubectl 打印后的结构。PodList/EndpointSliceList 列表项可能省略 apiVersion/kind，只能根据已验证的列表头补齐；冲突的条目类型和不完整分页仍拒绝，其他字段完整保留供身份与模板验证。

新 Service 可能存在 endpoints 和 ports 为 null 的受控 EndpointSlice。保留这些空集合的原始形式，等待控制器发布就绪目标前报告 Pending；归属校验及非空端点、端口校验保持严格。

launcher 回收子进程、关闭代理传输并负责已接管连接。管理端口在启动子进程前完成绑定，每条退出路径均等待两个服务及子进程清理完成。`os/exec` 负责输出复制，输出读完后才公布进程结束；`Cmd.WaitDelay` 限制子进程退出后继承管道的等待时间。强制 SIGKILL 不算成功停止环境。

Go 的线协议数据使用具体结构体，阻塞操作传递 context，返回错误时添加操作上下文。每个 goroutine 都需要明确的退出和等待路径。Node 工程脚本使用根目录严格 `tsconfig.json` 和固定版本的 Oxlint。

使用官方 `go fix -diff ./...` 审核适用的现代化改写；`errors.AsType`、`strings.SplitSeq`、原子类型和 `WaitGroup.Go` 明确表达意图。连接排空测试使用 `testing/synctest` 虚拟时钟，避免靠睡眠猜测调度顺序。选择新 API 时保持 JSON 线协议语义。

launcher 和命令测试在清理完成后同时运行 goleak 与 Go 1.27 原生 `goroutineleak` profile。原生 profile 主动触发检测 GC，发现不可达且永久阻塞的 goroutine；goleak 也检查仍由全局变量持有的遗留 goroutine。子进程负向控制证明两种检测都会拒绝泄漏。race 检查覆盖实际执行路径，不能证明所有调度组合。profile 保留在测试诊断中，工作负载不开放 pprof HTTP 端口。

Connector 固定 Zod 4.6.7，x/net 升至 0.62.0。goleak 1.3.0 已是当前正式 tag；WebSocket 探针保留更晚的 Gorilla pseudo-version，不降回较老的 latest tag。依赖评审同时参考模块更新和漏洞报告，全部官方 DSH 包仍保持精确 RC 版本针；仅为三个传递 SDK 固定安全版本，依据见 [compat/dsh](../../compat/dsh/README.zh-CN.md)。

真实 Kubernetes 生命周期、卷和网络检查使用 [test/environment](../../test/environment/README.md)。浏览器和上游包 smoke 使用 [test/rc2](../../test/rc2/README.md)；[compat/dsh](../../compat/dsh/README.md) 说明精确 DSH 基线与原生设置行为。使用一次性 fixture，同一集群仅一个写入负责人。模型/工具授权、OIDC 和双用户验收使用[平台](https://github.com/GuoMonth/dsh-multi-tenant) harness。

提交使用 `git commit -s`。PR 记录实际命令、制品身份和未测试项。包与镜像准备见[发行流程](releasing.md)。
