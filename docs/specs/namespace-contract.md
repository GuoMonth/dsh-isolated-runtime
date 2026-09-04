# Namespace capability contract

A Kubernetes Namespace is the Cell tenant boundary, but the project does not
own Namespace lifecycle or define a universal tenant profile. Cluster
administrators decide whether a namespace has each independent capability.

| Capability | Required cluster state | Native failure surface |
| --- | --- | --- |
| Core Cell | Active Namespace; StorageClass and admission policy that allow two PVCs, one StatefulSet Pod, two Services, one ServiceAccount and one NetworkPolicy per Cell | Cell Conditions, PVC/StatefulSet status and Events |
| Public browser access | Gateway API and Gateway configuration; namespace selected by the Gateway `allowedRoutes` policy | HTTPRoute parent Conditions |
| Snapshot and restore | Stable VolumeSnapshot APIs; compatible StorageClass, VolumeSnapshotClass and CSI driver | CellSnapshot Conditions and VolumeSnapshot status |
| Sandboxed Cell | Cluster-owned RuntimeClass selected by operator configuration | Existing Cell WorkloadReady Condition |

ResourceQuota, LimitRange, Pod Security admission, RuntimeClass, StorageClass,
Gateway policy and API Priority and Fairness are cluster policy. The operator
does not create, mutate, list, watch or interpret those policy objects. A native
admission rejection is not translated by parsing error prose into a new API.
Correcting quota or another prerequisite is expected to converge through the
normal Kubernetes control loops and the operator workqueue.

The operator never labels a tenant namespace. The
[`tenant-namespace.yaml`](../../config/samples/tenant-namespace.yaml) sample
shows only the opt-in label used by the reference Gateway. It intentionally
contains no recommended quota or security values.

## Non-goals

- no `NamespaceConformance`, Fleet, placement or policy CRD;
- no namespace controller, admission webhook or policy engine;
- no claim that a namespace suitable for a core Cell also supports Gateway,
  snapshots or sandboxed execution;
- no security guarantee against a cluster administrator, broken CNI/CSI,
  force deletion, node compromise or a tenant granted policy-management rights.
