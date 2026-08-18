[English](./repository-layout.md) | 简体中文

# 仓库结构

```text
api/v1alpha1/          版本化 API 类型
cmd/
  gateway/            trusted admission/router 骨架
  scheduler/          Runtime allocation 进程（名称暂定）
  controller/         生命周期编排
pkg/
  runtime/            tenant-owned runtime 契约
    kubernetes/       Kubernetes backend adapter
    runtimetest/      可复用 backend 契约测试
  provisioning/       DSH runtime images + profiles
  scheduling/         Runtime allocation；绝不做 Node placement
  gateway/            trusted principal + admission
  checkpoint/         单一逻辑持久化/恢复权威
  controlplane/       生命周期编排
config/
  crds/               Kubernetes CRD 样例
  rbac/               controller RBAC
```

## 依赖与所有权规则

- `api/` 不依赖具体实现。
- `pkg/runtime` 拥有隔离边界契约以及 tenant-aware create/get/delete 语义。
- `pkg/runtime/kubernetes` 可以使用真实 Kubernetes 概念，但这些细节不能泄漏到无关 package。
- `pkg/scheduling` 分配的是 **Runtime**，不是 Node。
- `pkg/checkpoint` 独占连续性契约；runtime backend 不再定义竞争性的 checkpoint API。
- `pkg/runtime/runtimetest` 是所有 backend 共用的契约测试套件。

M1–M4 完成前，接口保持 prerelease。
