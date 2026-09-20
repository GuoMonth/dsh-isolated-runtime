# R5 allocation implementation

The private connector now exports `AllocationRuntime.create` and
`inspectAllocation(intent, expectedIdentity, context)` in addition to access.
Intent contains allocationKey, owner (tenantId/principalId) and template; platform
business code never derives Kubernetes names, UIDs or origins.

`CellAllocationOptions` maps each trusted tenant to one distinct namespace and
pins profiles (`template`, exact defaulted `expectedSpec`, exact defaulted
`expectedPodSpec`). The adapter hashes the UUID allocation key into a deterministic
Cell name. Namespace is tenant authority; `spec.allocation` binds key, principal,
template and the canonical profile digest. CRD transition rules forbid adding,
removing or changing this intent after creation. Admission rejects spec/profile
or image/version drift; there is no patch/adoption operation.

The existing Operator still owns workloads, storage and network resources. Apply
the current generated CRD before using R5; old CRDs may prune allocation fields
and must not be treated as compatible. Platform RBAC adds only `create` for Cells
to R2 reads; no Pod/PVC writes, patch or deletion permissions are needed for R5.

`create` performs a bounded lookup then a single POST if missing. Existing/409
resources must match all intent fields and the pinned spec. A timeout, transport
failure, malformed accepted response or ambiguous server error reports uncertainty.
It never retries writes. The platform must persist a submitted marker first and
must never invoke create again after submission, binding or record disappearance.
There is no exactly-once promise after deliberate state destruction.

`inspectAllocation` returns Pending while current Ready observations/workloads
are absent, Deleting for deletionTimestamp, or verified Ready after the full R2
resource chain check. Missing records, changed identity and configuration/template
errors fail closed. No wait loop: each read returns current facts within a bounded
API budget; Pending is not failure and never means resources are already usable.
The caller may explicitly query again. No background polling/recovery is installed.

Origins follow existing `cell-<UID>.<domain>` convention, derived only inside the
adapter. Profile Pod strings may use `${INSTANCE_ID}` and `${ORIGIN_HOST}`;
all other fields must match API defaulted values for the fixed deployment.
No restoreFrom is supported in allocation profiles; new allocations must not
implicitly reuse old writer data. Profiles cannot include `spec.allocation`.

## Regression inventory

- Real CRD admission rejects adding/removing/changing allocation intent; same key
  and same intent deduplicate, different principal/template/profile reject.
- Permission/configuration failures, 409 races, response loss, cancellation after
  dispatch, missing/pruned allocation fields and non-JSON responses are diagnosed.
- Pending/current generation, template defaults, status version/digest and complete
  workload identity validation; stale UID and missing bound records never recreate.
- Namespace mapping remains distinct; Gateway/TLS wildcard origin and CNI enforce
  the same platform-only path for newly created Cells.

The fixed combination passed core cluster regression; see the [shared regression report](https://github.com/GuoMonth/dsh-multi-tenant/blob/4ba252765bccb41314c0bdc6b11bcf60cc0b33ef/docs/evidence/cell-regression-2026-09-20.md) for actual coverage and limitations. The inventory above is not an exhaustive pass claim.
Precise deletion is described in [R6](r6-deletion.md); allocation itself never deletes or recycles data.
