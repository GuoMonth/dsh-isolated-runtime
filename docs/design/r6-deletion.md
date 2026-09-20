# R6 exact deletion request

`AllocationRuntime.requestDelete(intent, expectedIdentity, context)` is the
internal deletion boundary. It re-reads the configured Cell, verifies namespace,
UID and the immutable allocation/profile plus current spec, then issues one
DELETE with UID and resourceVersion preconditions and Foreground propagation.
It does not require Ready, read a Pod endpoint or wait for the writer to stop.

A changed resource produces a conflict, never an unconditional second delete.
Transport uncertainty reports DeleteOutcomeUnknown; definitive rejection reports
not-submitted. API acceptance or an existing deletionTimestamp returns accepted /
Deleting. A missing record returns not-submitted / Missing. Every successful
result explicitly carries `writerState: unverified`. The runtime neither retries
deletes nor interprets absence as safe storage reuse. Waiting is bounded to ten
seconds including the pre-read; normal Kubernetes reconciliation/GC continues.

The caller must first durably close access and record deletion intent, serialize
with create/query, and reject deletion of unresolved creation. Runtime close,
logout and session expiry still never invoke resource deletion. The private
platform administrative socket is the supported R6 caller; regular OIDC member
permissions do not grant deletion.

Before deployment, add namespace-scoped `delete` on `cells` to the platform
service account. Keep child-resource/Secret deletion and patch permissions absent.
Verify this RBAC before issuing an administrator request; a rejected request still
leaves the platform deletion barrier closed.

## Actual ownership and data

- Data PVC: `reconcileDataPVC` in `internal/controller/resources.go` attaches a
  Cell owner only for the template's `Delete` retention policy. `Retain` leaves
  data without that controller owner. This is retention intent, not proof of
  completed CSI/backend destruction or a stopped writer.
- Private PVC: `reconcilePrivatePVC` uses Cell ownership regardless of data
  retention, so it participates in Kubernetes garbage collection. Private
  credentials/state must not be described as retained just because data is Retain.
- Provider Secret: the workload references the administrator-provided
  `credentialsRef`; deleting a Cell does not itself delete that external Secret.
  Separate explicit administrative authorization is required for its disposal.
- StatefulSet, Service and other Cell-owned resources use existing ownership and
  garbage collection. R6 adds no resource finalizer, force-delete, purge, fencing,
  snapshot import or old-volume reuse path.

Normal Pod replacement persistence must still be verified in the current image /
CSI combination. Node loss, forced deletion or API object absence does not prove
physical writer termination. Unverified cleanup remains manual and never opens a
new writer on old storage.

## Consolidated regression

Exact UID and resourceVersion races, deletion of Pending/Unavailable instances,
wrong owner/template, Forbidden, 404, existing deletionTimestamp, timeout before
and after dispatch, API 5xx and loss of the response. Verify no repeated DELETE,
no replacement deletion, and no writer-stopped claim. Verify real Retain/Delete,
private PVC and external Secret ownership in the actual cluster.

TypeScript build/static checks and three local TLS API regression cases pass
(`npm test --prefix packages/cell-connector`; requires openssl). The API fixture
proves request/precondition/error handling, not Kubernetes admission/GC. Real
cluster deletion/data behavior is not yet verified. This document is not authorization to delete data.
