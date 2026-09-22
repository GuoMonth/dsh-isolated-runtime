# R2 internal Cell Connector

`packages/cell-connector` is a private TypeScript module, owned by this runtime
repository and loaded in the platform's Node process. R5 adds a bounded Cell
create operation through [AllocationRuntime](design/r5-allocation.md). There is no
independent service, public package release or Go FFI. Node 24 is the current
integration build/runtime requirement.

The neutral entry points are `RuntimeAccess.inspect/connect`. They consume opaque
InstanceRef and an abort context; a single-use Connector forwards HTTP/stream or
WS without exposing a Pod address to platform business code. Each actual request
rechecks the prebuilt binding, Cell UID/defaulted spec/current Ready generation,
DSH/image identity, workload mode, resource owners, Service selectors/port,
Pod and EndpointSlice. Target addresses come only from the verified API chain.
This is bounded, GET-only API access (5 seconds per verification, 4 MiB per API
response); API failures deny access. It is not atomic Kubernetes-to-network fencing.

R2 binds the fixed `cell-mvp-v1` template to operator inputs: a digest-pinned
image, durable storage, required CPU/memory requests and limits, optional
credentials Secret, namespace mapping and domain. The Cell/Pod shape is generated
from the Go controller's `DesiredPodTemplate` and shipped inside this Connector;
there are no administrator `expectedSpec` or `expectedPodSpec` objects. Unknown
configuration keys, old profiles and any other template version are rejected.
CPU quantities accept positive whole cores or 1–999 milli-cores. Memory and
storage accept positive binary quantities (`Ki`, `Mi`, `Gi`, `Ti`) whose amount
does not canonicalize to the next suffix. Requests may not exceed limits.

The Connector compares the complete Cell spec and StatefulSet Pod template,
including the full PodSpec and template metadata. It compares the complete live
PodSpec after adding only the known Kubernetes 1.37 Pod defaults and deterministic
StatefulSet fields; scheduler `nodeName` must be present. UID/owner references,
Cell annotations, selectors, Service and EndpointSlice identities remain live
checks; template digests do not prove ownership. Sidecars, extra volumes,
LimitRange resource mutation and admission webhook changes fail closed.

The generated Pod fixes `DSH_PERMISSION_MODE=danger-full-access` as an explicit
product choice: the ordinary Pod is the execution boundary, so native DSH tools
do not apply a second, nested tool sandbox or approval gate. This only changes
DSH's in-Pod tool permission layer; it does not change the non-root UID,
read-only root filesystem, dropped capabilities, seccomp profile, absent
ServiceAccount token, or cluster network policy.
R5 implements create-time immutable allocation/principal/template fields. Access
still refuses not-Ready instances; allocation queries return current Pending or
Unavailable without maintaining a lifecycle cache.

The proxy preserves the configured application Host/Origin and only DSH auth
cookies; platform/identity headers and credentials do not reach the Cell. It passes
only host-only Secure HttpOnly DSH cookies back to the browser. The launcher owns
native token bootstrap. Parent cancellation tears down pending admission and
active connections; no resource stop or delete method is available.

## Local artifact consumption

Commit source inputs, then run:

```sh
node hack/pack-cell-connector.mjs /absolute/output/directory
```

When changing the Go Pod renderer, regenerate its checked-in template and then
verify it from the runtime repository root:

```sh
GOTOOLCHAIN=go1.27.1 go run ./internal/controller/cmd/generate-cell-template \
  > packages/cell-connector/src/templates/cell-mvp-v1.json
./hack/verify-cell-template.sh
```

The packer installs the locked build dependencies, builds declarations/JS, includes
the repository LICENSE and source.json with the exact commit, and emits a local
`.tgz`. It refuses uncommitted connector/packer/license inputs. The platform commits
that generated artifact and its SHA-256 as a pinned build input; maintain source
here, never edit extracted/copied code there. The Connector is not published independently; its fixed implementation and license are bundled into the platform npm CLI.

The platform requires a service-account token file and CA, configured HTTPS API
server, and namespace-scoped GET access to Cells, StatefulSets, Services and Pods,
plus LIST EndpointSlices. Keep credentials outside Cell storage. Binding namespaces are administrator-managed. Current allocation/deletion adds namespace-scoped Cell create/delete permissions; no Pod/PVC writes. See R5/R6 and the platform RBAC reference.

Current native results are in the [shared regression report](https://github.com/GuoMonth/dsh-multi-tenant/blob/4ba252765bccb41314c0bdc6b11bcf60cc0b33ef/docs/evidence/cell-regression-2026-09-20.md). The broader risk inventory includes malformed/stale API objects, token rotation and TLS rejection,
blocked reads, incorrect owners/template/ports/endpoints, auth during admission,
stream/WS cancellation, bootstrap cookies, raw query/redirect behavior and actual
CNI/Gateway network isolation. Do not treat this inventory as an all-pass claim.
