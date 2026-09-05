# Namespace 能力契约

Kubernetes Namespace 是 Cell 的租户边界，但本项目不拥有 Namespace 生命周期，也不定义
通用租户模板。集群管理员分别决定一个 namespace 是否具备以下能力。

| 能力 | 必要的集群状态 | 原生失败入口 |
| --- | --- | --- |
| Core Cell | Active Namespace；StorageClass 与 admission policy 能容纳每个 Cell 的两个 PVC、一个 StatefulSet Pod、两个 Service、一个 ServiceAccount 和一个 NetworkPolicy | Cell Conditions、PVC/StatefulSet 状态与 Events |
| 公开浏览器访问 | Gateway API 与 Gateway 配置；namespace 被 Gateway `allowedRoutes` policy 选中 | HTTPRoute parent Conditions |
| 快照与恢复 | 稳定 VolumeSnapshot API；相容的 StorageClass、VolumeSnapshotClass 与 CSI driver | CellSnapshot Conditions 与 VolumeSnapshot 状态 |
| Sandboxed Cell | 由 Operator 配置选择的集群自有 RuntimeClass | 既有 Cell WorkloadReady Condition |

ResourceQuota、LimitRange、Pod Security admission、RuntimeClass、StorageClass、Gateway policy
和 API Priority and Fairness 都是集群策略。Operator 不创建、修改、list、watch 或解释这些
策略对象，也不会解析原生 admission 错误文本来生成新的项目 API。管理员修正 quota 或其他
前置条件后，系统通过 Kubernetes 原生控制循环与 Operator workqueue 自动收敛。

Operator 永不为租户 Namespace 打标签。
[`tenant-namespace.yaml`](../../config/samples/tenant-namespace.yaml) 只展示参考 Gateway 所需的
opt-in label，刻意不提供通用 quota 或安全参数。

## 非目标

- 不增加 `NamespaceConformance`、Fleet、placement 或 policy CRD；
- 不增加 namespace controller、admission webhook 或 policy engine；
- 不假定能运行 core Cell 的 namespace 一定支持 Gateway、snapshot 或 sandboxed execution；
- 不对集群管理员、故障 CNI/CSI、强制删除、宿主机失陷或被授予策略管理权限的租户提供保证。
