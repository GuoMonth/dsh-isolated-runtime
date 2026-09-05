# Roadmap

Roadmap 按垂直大里程碑推进。每个大里程碑结束时必须复盘并通过门禁，才能进入下一阶段。

## Phase 0 —— Cell 契约重置与 DSH 实证

本次交付完成：

- 唯一的 namespaced `Cell` API 与可重复生成的 CRD；
- DSH 源码、状态、协议及关闭行为精确基线，当前已推进至 `dsh-v0.1.3-alpha.1`；
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

## Phase 2 —— 可信浏览器访问

本里程碑交付：

- 从每个 Cell UID 派生一个 HTTPRoute 与 access Role，不扩展 Cell API；
- Envoy Gateway 终止公网 HTTPS/OIDC，再将可信 route metadata、实时 Kubernetes 对象与
  不缓存的 SAR 精确绑定；
- 用真实 Chromium 实证跨 Cell 隔离、即时授权/撤权、路由防混淆、503 故障关闭与重启持久化；
- authorizer 与 Operator 共用现有镜像，保持两个镜像的发布面。

证据与里程碑结论记录在
[Phase 2 复盘](https://github.com/GuoMonth/dsh-isolated-runtime/issues/28)。

## Phase 3 —— 数据生命周期

本里程碑交付：

- 增加完全不可变的 `CellSnapshot` 意图，以及只能在创建时指定的
  `storage.restoreFrom`，不生成项目私有备份控制面；
- StatefulSet 降到零副本并通过 Kubernetes writer-stop barrier 后，才通过稳定 CSI API
  快照 data PVC；
- 每个 Cell 串行执行数据操作；清理结果不明确时 fail closed，只有成功或确认清理后才恢复源
  Cell；
- 只允许使用快照记录的精确 image digest 与 DSH RC 创建 fresh Cell，再实证同 RC digest
  rollout 与 fresh-Cell rollback；
- private/provider/browser-signing 状态不进 snapshot；恢复 Cell 拥有新的 UID、hostname、
  private PVC 与 DSH cookie。

参考 kind 夹具只为验证安装 external-snapshotter 与 CSI hostpath test driver；生产 CSI
lifecycle 仍由集群管理员负责。证据与 `GO` 结论记录在
[Phase 3 复盘](https://github.com/GuoMonth/dsh-isolated-runtime/issues/33)。

## Phase 3.1 —— 契约加固

这个纠偏里程碑用可验证结论替换此前过强的表述：

- 在 launcher 边界剥离固定 Envoy OAuth2 的 access、ID、refresh、nonce、expiry 与 HMAC
  cookie，同时保留 DSH cookie；
- 删除无法实证的应用 quiesce 协议，精确声明 writer-stopped、crash-consistent CSI 保证；
- 绑定源 data PVC UID，并以 API 可见的 Kubernetes invariant 关闭 workload lineage、
  stale lock、create/adopt、EndpointSlice、restore 删除和 first-reader 竞态；
- 发布候选只构建一次，全部门禁消费相同 digest，随后把同一带 SBOM/provenance 的 manifest 晋级。

[Phase 3.1 复盘](https://github.com/GuoMonth/dsh-isolated-runtime/issues/44)
已经根据 post-merge 证据记录 `GO`，Phase 4 因此解除阻塞。

## Phase 4 —— Fleet 运维

本里程碑交付：

- 定义基于 capability 的 namespace conformance 契约，但不读取、复制或持有 namespace policy；
- ResourceQuota、LimitRange、admission、APF、调度与垃圾回收继续作为 Kubernetes 原生失败和
  恢复面；
- 用对象 watch、精确 deadline 唤醒与 controller-runtime 指数错误退避替代稳定态轮询，并显式
  限制 Cell / CellSnapshot worker 数量；
- 提供可选私有 Prometheus endpoint；label 只允许 controller-runtime 维度与封闭的授权结果枚举；
- 在文档化参考 runner 上实证 10 namespace / 50 Cell 的收敛与恢复、原生 quota / LimitRange
  拒绝、namespace 重建、controller 重启和 8 个重叠 CSI 操作。

Kubernetes 始终是 fleet 的调度器和状态协调器。Phase 4 不增加 Fleet API、inventory database、
policy engine、自定义队列或第二套集群编排器。证据与结论记录在
[Phase 4 复盘](https://github.com/GuoMonth/dsh-isolated-runtime/issues/39)。

## MVP v0.1.0 — Core closure and release

In progress: [milestone 7](https://github.com/GuoMonth/dsh-isolated-runtime/milestone/7).
Latest exact DSH baseline (#50), complete user journey (#51), local demo (#52),
immutable release bundle (#53), live acceptance and final GO/publication (#54).
No fleet-platform expansion or historical compatibility.
