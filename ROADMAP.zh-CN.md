# Roadmap

Roadmap 按垂直大里程碑推进。每个大里程碑结束时必须复盘并通过门禁，才能进入下一阶段。

## Phase 0 —— Cell 契约重置与 DSH 实证

本次交付完成：

- 唯一的 namespaced `Cell` API 与可重复生成的 CRD；
- DSH `dsh-v0.1.2-alpha.4` 的源码、状态、协议及关闭行为精确基线；
- 可执行的 Cell-local launcher 实验；
- kind 契约测试、轻量 CI 与按路径触发的完整 DSH CI；
- 删除旧 Runtime/control-plane 架构。

只有本地、kind 和精确上游兼容门都通过，复盘结论才是 `GO`。

## Phase 1 —— Single-Cell 垂直切片

本里程碑交付：

- 将一个 Cell reconcile 为两个 PVC、ServiceAccount、StatefulSet、headless/access 两个
  Service、Ingress NetworkPolicy 与当前 generation 的 status；
- 将精确 DSH npm 产物打入 digest 固定、以 launcher 为 PID 1 的 Cell 镜像；
- 在 kind 中实证 ownership、rollout、admission、网络访问、retention、故障恢复，以及 Pod
  替换后的 DSH/browser 状态持久性；
- 发布带 SBOM 的 linux/amd64 Cell 与 Operator 镜像。

本地、镜像、精确上游与可重复 kind 实证均已通过；
[Phase 1 复盘](https://github.com/GuoMonth/dsh-isolated-runtime/issues/23)结论为 `GO`。

## Phase 2 —— 可信访问

将外部身份认证与 Cell 授权组件接入 Gateway API；路由由集群 base domain 与 Cell identity
派生；保持 DSH Host/Origin 语义；launcher 只能从批准的 ingress 路径访问。首轮工作见
[Phase 2 milestone](https://github.com/GuoMonth/dsh-isolated-runtime/milestone/3)。

## Phase 3 —— 数据生命周期

围绕数据 PVC 定义 CSI snapshot/restore，明确排除 provider 凭据与 DSH 浏览器签名状态；
证明 quiesce 语义并拒绝并发写入。

## Phase 4 —— Fleet 运维

补充 policy、升级、可观测性、quota 与多 namespace 运维。Kubernetes 始终是 fleet 的调度器
和状态协调器；本项目不生长出第二套集群编排器。
