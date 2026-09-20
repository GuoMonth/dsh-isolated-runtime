# R2 internal Cell Connector

`packages/cell-connector` is a private TypeScript module, owned by this runtime
repository and loaded in the platform's Node process. It has no write operations,
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

R2 uses administrator-controlled allocation → namespace/name/UID/origin/template
bindings. `expectedSpec` is the complete defaulted Cell spec from the pinned
fixture; `expectedPodSpec` is the exact defaulted StatefulSet Pod template spec.
Both desired workload and running Pod must match these pinned fields, including
resources/security/storage mounts, not merely the image string. Do not invent partial templates or take this configuration from a browser.
R5 owns create-time immutable owner/template fields. Runtime currently refuses
not-Ready instances instead of maintaining a lifecycle cache.

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

The packer installs the locked build dependencies, builds declarations/JS, includes
the repository LICENSE and source.json with the exact commit, and emits a local
`.tgz`. It refuses uncommitted connector/packer/license inputs. The platform commits
that generated artifact and its SHA-256 as a pinned build input; maintain source
here, never edit extracted/copied code there. Nothing is published to npm.

The platform requires a service-account token file and CA, configured HTTPS API
server, and namespace-scoped GET access to Cells, StatefulSets, Services and Pods,
plus LIST EndpointSlices. Keep credentials outside Cell storage. Binding namespaces
are administrator-managed; do not give the platform write credentials for R2.

This code is an implementation slice, not a native acceptance result. Deferred
regression includes malformed/stale API objects, token rotation and TLS rejection,
blocked reads, incorrect owners/template/ports/endpoints, auth during admission,
stream/WS cancellation, bootstrap cookies, raw query/redirect behavior and actual
CNI/Gateway network isolation. See the current Issue #82 regression checklist.
