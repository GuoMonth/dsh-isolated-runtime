# 当前路线图

当前顺序和验收由 multi-tenant 平台维护：[Agent Workspace 设计](https://github.com/GuoMonth/dsh-multi-tenant/blob/main/docs/design/agent-workspace.zh-CN.md)、[主 Issue #104](https://github.com/GuoMonth/dsh-multi-tenant/issues/104) 与[平台 roadmap](https://github.com/GuoMonth/dsh-multi-tenant/blob/main/docs/roadmap.md)。

目标是 K8s 唯一后端的 Agent Workspace，由 runtime 的薄 CRD/Operator 管理。W1 将破坏性地把 `Cell` 改名为 `AgentWorkspace`，并移除 Process/Docker 产品后端、standalone 启动与 snapshot/restore；当前源码仍实现 Cell，W1 尚未落地。W2 在健康节点通过显式 StatefulSet 与 data/private PVC 正常停止/启动，保留两卷身份，不承诺节点分区 fencing。W3 联合验证真实授权、持久 HOME 和性能后再发行。备份、热池、自动 idle 暂缓。见[当前 MVP 实现状态](docs/alpha-mvp.zh-CN.md)。
