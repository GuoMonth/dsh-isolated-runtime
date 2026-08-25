[English](./ROADMAP.md) | 简体中文

# 路线图

状态：✅ 已完成 · 🚧 下一步 · 🧭 计划中 · ⏳ 延后。

## 已完成

- ✅ **M0 —— 启动。** 仓库、双语文档、六层架构、Go 控制平面骨架、CI。
- ✅ **M0.1 —— 架构对齐。** 将 Runtime 租户所有权写入契约；把调度收敛为 Runtime
  复用/创建而不是 Node placement；Gateway 身份改为可信 transport context；
  checkpoint 收敛为单一逻辑持久化/恢复权威；增加可复用 Runtime 契约测试；
  修正过度隔离承诺与 Kubernetes adapter 边界。
- ✅ **M1 —— 自托管控制面 + Kubernetes Runtime backend。** 使用 namespaced Runtime CR
  持久化租户 Runtime 期望状态；把每个 Runtime reconcile 成一个 hardened Pod；使用实时
  inventory 做安全的同租户复用；落地 `standard` / `sandboxed` 安全姿态与 Runtime
  deny-all NetworkPolicy；提供单实例 Admin UI/API 和 M1 固定启动账号 `Admin / Admin`；
  提供 Docker/Kustomize 自托管安装。

## 生产可用的定义

**M1 是可自托管的开发者预览版，不是生产版本。** 项目在 M8 完成并通过生产验收门槛后，
才进入 production-ready 阶段。

生产版本至少必须满足：

- 不再依赖公开的固定 `Admin / Admin` 凭据；管理员身份、凭据和授权可安全配置与轮换。
- Runtime/session 路由、逻辑状态与控制面重启解耦；控制面重启或升级不会丢失租户期望状态。
- 控制面具备幂等 reconcile、重试/退避、终止清理、故障恢复和多副本安全运行能力。
- 有结构化日志、metrics、审计事件、健康/就绪检查、告警与最小运维 runbook。
- 有版本化发布、镜像供应链安全、CRD/API 升级与回滚策略。
- 有真实 Kubernetes 集群 E2E、安全隔离、重启/升级、故障注入与基础负载验收。
- 有明确的备份/恢复边界；DSH 逻辑状态与对象存储数据可以恢复，控制面资源可以重建。

## 下一步

- 🚧 **M2 —— Runtime Gateway + Session Binding。** 将 Gateway 从 admission skeleton 变成
  真正的数据入口：接入真实认证/授权，持久化 tenant + conversation/session → Runtime
  绑定，透明代理 DSH HTTP/WebSocket/stream，并保证客户端永远看不到 Pod、Service、
  Namespace、Node 等运行时拓扑。Runtime allocation 继续只做同租户 reuse/create，Node
  placement 仍完全交给 kube-scheduler。

- 🧭 **M3 —— 逻辑持久化 + 恢复。** 将 DSH/session/workspace/artifact 状态与版本化 manifest
  持久化到 S3/MinIO 兼容对象存储；Runtime 可以被删除并在新的 Pod 中恢复。定义明确的
  snapshot/restore API、状态版本兼容策略、失败重试、幂等恢复和数据生命周期。

- 🧭 **M4 —— 标准 DSH Runtime Images + Profiles。** 发布 `base`、`data`、`dev` 等标准镜像
  与版本化 profile；镜像使用不可变 digest，profile 统一生成资源、安全与 RuntimeClass
  策略。补齐镜像构建、漏洞扫描、SBOM 与最小 provenance，为生产供应链打基础。

- 🧭 **M5 —— 端到端隔离与安全套件。** 自动证明 Runtime ownership、文件系统、网络、
  credential/service account、Gateway 路由和 persistence 上的跨租户隔离；验证
  `standard` / `sandboxed` 行为差异、非法 profile/RuntimeClass 拒绝、默认 deny 网络策略，
  并把威胁模型变成持续运行的安全回归测试。

- 🧭 **M6 —— 生产控制面加固。** 移除生产路径上的固定 `Admin / Admin`：支持 Kubernetes
  Secret 配置管理员凭据，并为 OIDC/外部 IdP 留出稳定认证边界；增加 CSRF/secure-cookie/
  API 限流与审计。将 reconcile 从简单轮询提升为可靠的 Kubernetes watch + resync 模型，
  加入 finalizer、指数退避、冲突重试、启动恢复和孤儿资源清理。控制面支持多副本，使用
  leader election/lease 保证写入与 reconcile 安全，并明确升级期间的版本兼容范围。

- 🧭 **M7 —— 可观测性与运维。** 提供结构化日志、Prometheus metrics、Kubernetes Events、
  管理操作审计、dashboard/alerts；区分 liveness/readiness；暴露 Runtime 创建延迟、失败率、
  reconcile 错误、Pod 启动时间、活跃 Runtime/session 数量等核心指标。补充容量/配额策略、
  故障排查、证书/凭据轮换、对象存储故障和 Kubernetes API 不可用等 runbook，并定义初始 SLO。

- 🧭 **M8 —— 发布、升级、灾备与生产验收。** 建立 SemVer release、不可变/签名镜像、
  SBOM 与漏洞门禁；定义 CRD/API migration、向前/向后版本兼容、滚动升级与回滚流程；提供
  控制面资源重建与 DSH 状态恢复演练。CI 增加真实/临时 Kubernetes 集群 E2E，并覆盖
  control-plane restart、Pod crash、Kubernetes API 短暂失败、升级/回滚、跨租户攻击回归和
  基础并发/负载测试。**M8 全部通过后发布首个 production-ready 版本。**

## 延后

- ⏳ **进程/容器内存 checkpoint。** 第一阶段连续性不依赖 CRIU/runc/containerd checkpoint；
  只有出现明确 workload 需求时再评估。
- ⏳ **第二 backend / runtime-agnostic 契约冻结。** 先把 Kubernetes-first adapter 做干净；
  等 Docker、Firecracker、Nomad 或其他第二 backend 证明共同点后再冻结抽象。
- ⏳ **多集群/跨地域控制面。** 首个 production-ready 目标是单 Kubernetes 集群内可靠运行；
  multi-cluster placement、跨地域容灾与 federation 在真实规模需求出现后再设计。
- ⏳ **复杂企业级 IAM。** M6 先提供安全可配置管理员身份与外部 IdP 边界；细粒度组织/项目/
  角色权限、SCIM 等企业 IAM 能力不阻塞首个生产版本。

## 里程碑

- **M0** ✅ 启动。
- **M0.1** ✅ 架构对齐。
- **M1** ✅ 自托管控制面 + Kubernetes Runtime backend —— Developer Preview。
- **M2** 🚧 Runtime Gateway + Session Binding。
- **M3** 🧭 逻辑持久化 + 恢复。
- **M4** 🧭 标准 DSH Runtime Images + Profiles。
- **M5** 🧭 端到端隔离与安全套件。
- **M6** 🧭 生产控制面加固。
- **M7** 🧭 可观测性与运维。
- **M8** 🧭 发布、升级、灾备与生产验收 —— Production Ready Gate。
