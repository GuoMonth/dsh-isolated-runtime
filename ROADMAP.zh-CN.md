[English](./ROADMAP.md) | 简体中文

# 路线图

状态：✅ 已完成 · 🚧 下一步 · ⏳ 延后。

## 已完成

- ✅ **M0 —— 启动。** 仓库、双语文档、六层架构、Go 控制平面骨架、CI。
- ✅ **M0.1 —— 架构对齐。** 将 Runtime 租户所有权写入契约；把调度收敛为 Runtime
  复用/创建而不是 Node placement；Gateway 身份改为可信 transport context；
  checkpoint 收敛为单一逻辑持久化/恢复权威；增加可复用 Runtime 契约测试；
  修正过度隔离承诺与 Kubernetes adapter 边界。

## 下一步

- 🚧 **M1 —— Kubernetes Runtime backend + allocation。** 将租户所有的 Runtime 真正
  reconcile 成 Pod，应用 profile/资源/安全约束，并把 Node 选择交给 kube-scheduler；
  同时建立安全的同租户 Runtime inventory / reuse。
- 🚧 **M2 —— Gateway router / reverse proxy。** 接入真实认证授权，解析
  tenant + conversation/session → Runtime，然后透明代理 DSH HTTP/WebSocket/stream，
  不向客户端暴露 Pod/Service/Namespace/Node 等拓扑。
- 🚧 **M3 —— 逻辑持久化 + 恢复。** 将 DSH/session/workspace/artifact 状态和版本化 manifest
  持久化到 S3/MinIO 兼容对象存储，并恢复到全新的 Runtime。
- 🚧 **M4 —— 标准 DSH runtime images。** 发布 `base`、`data`、`dev` 等 profile，
  并由平台生成 `standard` / `sandboxed` 安全策略。

## 延后

- ⏳ **进程/容器内存 checkpoint。** 第一阶段连续性不依赖 CRIU/runc/containerd checkpoint；
  只有出现明确 workload 需求时再评估。
- ⏳ **第二 backend / runtime-agnostic 契约冻结。** 先把 Kubernetes-first adapter 做干净；
  等 Docker、Firecracker、Nomad 或其他第二 backend 证明共同点后再冻结抽象。
- ⏳ **公共契约冻结。** M1–M4 完成前，名称与 surface 仍是预发布状态。

## 里程碑

- **M0** ✅ 启动。
- **M0.1** ✅ 架构对齐。
- **M1** Kubernetes Runtime backend + allocation。
- **M2** Gateway router / reverse proxy。
- **M3** 逻辑持久化 + 恢复。
- **M4** 标准 DSH runtime images + profiles。
- **M5 —— 端到端隔离套件。** 证明 Runtime ownership、文件系统、网络、
  credential/service account 与路由上的跨租户隔离。是否提供更强宿主边界，明确取决于
  security class 与 RuntimeClass，而不是默认宣称所有 Pod 都具备绝对宿主隔离。
