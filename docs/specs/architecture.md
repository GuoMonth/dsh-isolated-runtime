# Architecture

## The boundary

A `Cell` is one namespaced DSH trust, execution, and durable-state boundary.
The namespace is the tenant identity. Cell is not a small Kubernetes clone: it
is a narrow intent API translated into native resources.

| Concern | Authority |
| --- | --- |
| Fleet, placement, restart, rollout, quota, RBAC | Kubernetes |
| External listeners and HTTP routing | Gateway API |
| Durable volumes and snapshots | CSI |
| Sessions, attachments, storage domains, protocol | DSH |
| Cell-to-native-resource translation and access seam | This project |

The Cell API never selects Nodes or carries session state, topology, routes, or
workload implementation details. The operator derives those from namespace,
Cell identity, cluster policy, and observed state.

## Target resource graph

```text
Cell
  └─ operator
      ├─ tenant-data PVC
      ├─ private-state PVC
      ├─ ServiceAccount (no workload API token)
      ├─ StatefulSet (1 replica): launcher (PID 1) → DSH child
      ├─ headless Service (StatefulSet network identity)
      ├─ ClusterIP Service (Cell access)
      ├─ NetworkPolicy
      ├─ Role (one Cell `access` verb)
      └─ HTTPRoute (UID-derived hostname)

CellSnapshot
  → StatefulSet replicas=0
  → observed zero replicas + no owned Pod (writer-stop barrier)
  → CSI VolumeSnapshot (tenant-data PVC only)
  → source Cell replicas=1
  → fresh Cell data PVC with VolumeSnapshot dataSource

Browser
  → Envoy Gateway (HTTPS + OIDC)
  → cell-authorizer (route validation + SubjectAccessReview)
  → HTTPRoute
  → launcher
  → DSH HTTP / WebSocket / streams / Fetch
```

The graph uses Kubernetes owner references and reconciliation; it introduces no
project scheduler, runtime inventory, checkpoint service, or shadow desired-state
database.

## Access seam

DSH 0.1.2 RC creates a launch token in process memory, prints it once in the
loopback readiness URL, and exchanges it for an authority-bound browser cookie.
There is no supported token injection interface. Therefore the selected design
is a launcher in the same container:

1. The launcher starts DSH as a child and captures its readiness URL.
2. The token stays in launcher memory and is used only on the internal first
   root request; it never enters the public URL, arguments, or logs.
3. HTTP, WebSocket, streams, and Fetch are proxied opaquely. Typert is not parsed.
4. External Host and Origin remain intact for DSH validation. HTTPS egress
   adds `Secure` and normalizes the DSH cookie to `SameSite=Lax`; DSH itself
   supplies `HttpOnly`. Lax is required for the safe top-level return from an
   external OIDC provider.
5. Authentication and Cell authorization happen before this seam. The launcher
   strips identity headers and every credential cookie used by the pinned Envoy
   Gateway v1.9.1 OAuth2 filter, including its per-policy suffixed names, while
   preserving DSH and unrelated application cookies. It trusts only the ingress
   path constrained by NetworkPolicy.

A detached sidecar cannot safely obtain the process token. Direct exposure
conflicts with the loopback-only CLI and leaks the launch URL. Pure Gateway
configuration cannot perform the in-memory token exchange or cookie rewrite.

## Trusted browser access

The public hostname and route are derived from the immutable Cell UID and a
cluster base domain; they are not Cell API inputs. Envoy terminates TLS and
validates the OIDC login before calling `cell-authorizer`. The authorizer trusts
only Envoy route metadata, rereads the exact HTTPRoute and Cell, verifies their
owner, UID, hostname, parent and backend, then submits an uncached
SubjectAccessReview for the Cell `access` verb. RoleBindings remain wholly
administrator-owned, so a grant or revocation applies to the next HTTP or
WebSocket request. Missing identity is 401, a bad route or denied RBAC check is
403, and dependencies fail closed with 503.

## State

The data PVC contains workspace, sessions, attachments, and DSH storage domains.
DSH's `.credentials.yaml` lives on the distinct private-state PVC;
provider keys normally arrive from the same-namespace `credentialsRef` Secret as
environment variables. Neither provider material nor browser-signing records are
part of data snapshots.

The persistence format is bound to the exact DSH version in
`compat/dsh/baseline.json`. Restore is a data operation, not a promise to migrate
foreign session formats. Before a snapshot the controller must stop the sole
managed writer and must never attach one read-write data volume to concurrent
Cell writers.

`CellSnapshot` is an immutable, one-shot Kubernetes intent. A UID-bound Cell
annotation acquired with resource-version compare-and-swap serializes data
operations. Acceptance records both the source Cell UID and data PVC UID before
the lock becomes active. Once `Accepted=True`, the Cell controller sets the
StatefulSet to zero. Only an observed-zero StatefulSet plus an uncached,
namespace-wide check proving that neither a Pod owned by the current StatefulSet
nor any Pod carrying the exact Cell name and UID remains establishes
`WriterStopped=True` and permits creation of the CSI `VolumeSnapshot`. The
controller revalidates the PVC UID and both CSI
class drivers immediately before creation. DSH 0.1.2 RC maps successful disposal, disposal
rejection, and timeout to externally indistinguishable process termination, so
the public contract deliberately claims crash consistency and never application
flush. Snapshot errors delete the owned Kubernetes snapshot object before the
source resumes; backend retention remains the CSI driver and
VolumeSnapshotClass policy.

A restore always creates a new data PVC and Cell identity. It requires a Ready,
same-namespace snapshot, its exact recorded image digest, the one supported DSH
RC, compatible size, and the same StorageClass. The private PVC is always new.
The data PVC records the snapshot UID, image digest, and DSH version. UID-bound
finalizers keep the snapshot alive until that recorded image becomes the first
Ready reader; another digest cannot enter first and deleting inputs cannot create
an immutable PVC. Once CSI has materialized a Bound data PVC, that PVC's recorded
provenance and the same finalizers form the durable barrier: a later snapshot
delete request waits while the exact-image first reader continues. Rollout to
another digest of the same RC is explicit only after this first-reader barrier.
Rollback is another fresh Cell from an older
snapshot, never an in-place PVC downgrade.

## Fleet operations

Fleet scale is repetition of the same namespaced graph, not a new object or
control plane. A namespace supplies capabilities independently: core Cell
resources require ordinary namespaced workload and PVC admission; public
access additionally requires Gateway route eligibility; snapshots require CSI
classes and APIs; sandboxing requires the configured RuntimeClass. The operator
does not list, watch, own, or interpret Namespace, ResourceQuota, LimitRange,
PriorityClass, or API Priority and Fairness objects.

Both reconcilers have explicit worker limits. Kubernetes object watches drive
normal progress, controller-runtime rate limiting handles API errors, and only
real writer-stop/snapshot deadlines schedule exact wakeups. A one-minute retry
is retained solely for cluster-scoped StorageClass/VolumeSnapshotClass changes
that cannot be mapped safely to individual namespaced requests without a
cluster-wide fan-out. A quiet cluster therefore produces no Cell polling loop.

Metrics are disabled by default and have no Service or scraper. When enabled,
the operator exposes controller-runtime process/work-queue aggregates and the
authorizer exposes a counter labeled only by a closed decision enum. Cell,
snapshot, namespace, user, hostname, UID, route, Pod, Node, address, provider,
or credential values are never metric labels. Kubernetes objects remain the
only inventory and diagnostic authority.

## Non-goals

- Replacing kube-scheduler, Gateway API, CSI, or a cluster fleet manager.
- Exposing Pod, Node, Service, `RuntimeClass`, or hostname choices in Cell.
- Interpreting DSH application protocols.
- Promising host-compromise resistance for ordinary containers.
- Supporting removed pre-Cell APIs or floating DSH versions.
- Scheduling backups, copying snapshots across clusters, or replacing CSI.
- Defining a Fleet CRD, namespace template, quota policy, custom scheduler,
  autoscaler, telemetry backend, SLO product, or topology inventory.
