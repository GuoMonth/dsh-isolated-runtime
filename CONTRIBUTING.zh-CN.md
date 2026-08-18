[English](./CONTRIBUTING.md) | 简体中文

# 贡献指南

## 开发模型：规格驱动，然后测试驱动

本仓库按 **先规格、再测试、最后实现** 的顺序开发。

1. **规格** —— 先写清边界和不变量，再写实现。
2. **测试** —— 可复用契约测试证明 backend 行为；端到端隔离测试证明安全属性。
3. **实现** —— 只实现满足规格与测试的最小内容。

## 组件边界

- **Runtime ownership 不可协商。** 每个 Runtime 恰好属于一个 Tenant；一个 Tenant 可以拥有
  多个 Runtime；跨租户 Runtime reuse 永远非法。
- **不要重新实现 Kubernetes 已经负责的基础设施职责。** Runtime allocation 决定
  reuse/create/profile/resources/security posture；Pod 到 Node 的调度仍交给 Kubernetes。
- **允许 Kubernetes 特定实现，但必须放在明确 adapter 后面。** Kubernetes 是第一个真实
  backend；K8s API 细节应留在 backend/controller adapter 中，不要在第二 backend 出现前
  强行抽象最低公分母。
- **Principal 是可信 transport state。** 调用方身份由服务端认证建立；公开 request payload
  可以描述目标资源，但不能建立调用者身份。
- **连续性优先采用逻辑状态。** `pkg/checkpoint` 是 persistence/restore 的唯一权威；
  不要再在 `pkg/runtime` 建第二套 checkpoint contract。
- **不要预建没有真实边界的组件。** 新 package/process 必须有已证明的安装、替换、
  生命周期或安全边界。

## 测试

- **Runtime Contract Suite** —— 位于 `pkg/runtime/runtimetest` 的共享套件；每个 backend
  都必须证明 ownership 不可变、外租户不可枚举、冲突语义与生命周期行为。
- **组件测试** —— allocation、admission、persistence、control-plane 契约。
- **隔离测试** —— 端到端验证跨租户文件系统、网络、credential 与路由隔离；
  更强宿主隔离声明必须明确绑定具体 `SecurityClass` / `RuntimeClass`。

## 双语文档

所有面向用户的架构/文档改动必须同步更新英文与简体中文。

## 许可证

贡献即表示你同意按 MIT 许可证授权。
