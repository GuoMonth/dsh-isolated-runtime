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
      └─ NetworkPolicy

identity + Cell authorization (Phase 2)
  → Gateway API route
  → launcher
  → DSH HTTP / WebSocket / streams / Fetch
```

The graph uses Kubernetes owner references and reconciliation; it introduces no
project scheduler, runtime inventory, checkpoint service, or shadow desired-state
database.

## Access seam

DSH alpha.4 creates a launch token in process memory, prints it once in the
loopback readiness URL, and exchanges it for an authority-bound browser cookie.
There is no supported token injection interface. Therefore the selected design
is a launcher in the same container:

1. The launcher starts DSH as a child and captures its readiness URL.
2. The token stays in launcher memory and is used only on the internal first
   root request; it never enters the public URL, arguments, or logs.
3. HTTP, WebSocket, streams, and Fetch are proxied opaquely. Typert is not parsed.
4. External Host and Origin remain intact for DSH validation. HTTPS egress adds
   `Secure` to the DSH cookie.
5. Authentication and Cell authorization happen before this seam. The launcher
   trusts only the ingress path constrained by NetworkPolicy.

A detached sidecar cannot safely obtain the process token. Direct exposure
conflicts with the loopback-only CLI and leaks the launch URL. Pure Gateway
configuration cannot perform the in-memory token exchange or cookie rewrite.

## State

The data PVC contains workspace, sessions, attachments, and DSH storage domains.
DSH's `.credentials.yaml` lives on the distinct private-state PVC;
provider keys normally arrive from the same-namespace `credentialsRef` Secret as
environment variables. Neither provider material nor browser-signing records are
part of data snapshots.

The persistence format is bound to the exact DSH version in
`compat/dsh/baseline.json`. Restore is a data operation, not a promise to migrate
foreign session formats. Before a snapshot the controller must quiesce DSH and
must never attach one read-write data volume to concurrent Cell writers.

## Non-goals

- Replacing kube-scheduler, Gateway API, CSI, or a cluster fleet manager.
- Exposing Pod, Node, Service, `RuntimeClass`, or hostname choices in Cell.
- Interpreting DSH application protocols.
- Promising host-compromise resistance for ordinary containers.
- Supporting removed pre-Cell APIs or floating DSH versions.
