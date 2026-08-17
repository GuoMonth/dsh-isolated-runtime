[English](./repository-layout.md) | 简体中文

# 仓库结构

六层在目录树上的映射如下。`api/` 是版本化的公共契约；`pkg/` 承载各层及其后端；
`cmd/` 承载三个进程。

```text
api/v1alpha1/          版本化 API 类型（契约）
cmd/
  gateway/            ④ 准入 —— HTTP 服务，默认拒绝
  scheduler/          ③ 调度 —— 放置进程
  controller/         ⑥ 控制平面 —— 生命周期编排
pkg/
  runtime/            ① 隔离边界 —— 运行时无关的 seam
    kubernetes/       ① 第一个后端（内存 stub）+ 契约测试
  provisioning/       ② runtime images + profiles
  scheduling/         ③ 放置接口 + first-fit 默认实现
  gateway/            ④ 准入逻辑
  checkpoint/         ⑤ 快照/恢复契约
  controlplane/       ⑥ 生命周期编排
config/
  crds/               CRD 清单（样例）
  rbac/               RBAC 清单
hack/                 本地质量门禁（verify.sh）
```

## 依赖方向

```text
api  ◀──  pkg/*  ◀──  cmd/*
```

- `api/` 是叶子 —— 它不 import `pkg/` 或 `cmd/` 中的任何内容。
- `pkg/runtime` 定义控制平面所依赖的 seam；后端（`pkg/runtime/kubernetes`）实现它。
- `pkg/scheduling` 与 `pkg/controlplane` import `api/` 以使用契约类型。
- `cmd/` 将各包组装为进程。

## 运行时 seam（①）

最重要的边界是 `pkg/runtime.Runtime` —— 所有后端都要实现的运行时无关接口。控制平面
（scheduler、gateway、control plane）只依赖 `runtime.Runtime`，不依赖任何 Kubernetes
特定的东西。任何后端都必须通过的契约测试位于 `pkg/runtime/kubernetes/kube_test.go`。

## 状态

所有组件都是骨架 —— 接口已锁定，实现在内存中 stub。真正的集群集成在 M1–M4 落地；
参见 [ROADMAP](../../ROADMAP.zh-CN.md)。
