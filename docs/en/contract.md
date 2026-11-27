# AgentEnvironment runtime contract

[简体中文](../zh-CN/contract.md) · [Guides](index.md)

The recommended Kubernetes reference image installs the exact `@dsh/environment-connector-internal` archive and selects a fixed adapter at platform startup. The Connector implements `createAgentEnvironmentRuntime(options)` using the Kubernetes resource contract in [port.ts](../../packages/environment-connector/src/port.ts) and the [fixed template](../../packages/environment-connector/src/template.ts). The [platform reference adapter](https://github.com/GuoMonth/dsh-multi-tenant/blob/main/charts/kubernetes-reference.ts) translates this interface to the platform environment contract v2.

## Inputs and identity

The platform authorizes an owner `(tenantId, principalId)` and creates a stable `allocationKey`. Administrator options supply Kubernetes server/CA/token files, namespace prefix, domain, image digest, StorageClass/size, CPU/memory requests/limits and platform namespace. Callers cannot supply arbitrary PodSpec or resource names.

Runtime creates an owner-labelled namespace, tokenless workload service account, network policy, one external PVC and an upstream Sandbox. It rejects foreign resources with the same name. The Connector receives an exact `EnvironmentRef` with owner, allocation key, namespace, Sandbox name/UID and PVC name/UID. The reference adapter preserves both UIDs as `instanceId` and `storageId` and stores the remaining lookup details in its `locator`. Connect verifies the current Pod UID.

The policy admits platform-labelled Pods from the platform namespace. DNS uses kube-system; public egress excludes cluster, private and metadata addresses. Kubernetes RBAC grants the trusted platform namespace creation, but cannot limit that permission by a name prefix. The [runtime role](../../config/runtime/cluster-role.yaml) cannot read Secrets, write Pods or delete PVCs/namespaces.

## Data and transport

The PVC mounts at `/var/lib/dsh/data`: `workspace/` is the working directory, `home/` is HOME, and `dsh/` is DSH_HOME. Native credentials use `$DSH_HOME/.credentials.yaml`. The PVC has no Sandbox ownerReference; deletion retains it and the namespace. Requested storage capacity depends on the storage backend.

`connect` accepts a Ready instance after owner, resource, template and endpoint verification. The context signal owns the connection lifetime: abort rejects further forwarding and closes HTTP streams/WebSockets. The launcher preserves native token/cookie and authority checks. DSH application traffic is forwarded without changing model/tool protocols.

The launcher advertises the environment HTTPS origin and receives the loopback listener and launch token over a private child-process pipe. The Connector admits cross-site top-level GET navigations to the native MCP OAuth callback path through the existing authorized environment connection. DSH owns callback validation and provider tokens.

## Queries and mutations

| Operation | Behavior |
| --- | --- |
| create | Returns a fully bound Pending/Ready view; unresolved writes retain the original allocation key |
| inspectAllocation | May return null before binding; missing known resources return StaleInstance |
| inspect | Reads the exact binding and separates observed state from read failure |
| stop | Revokes access, requests suspension and requires evidence that the exact writer terminated |
| start | Requires confirmed Stopped and the same PVC UID |
| delete | Requires confirmed stop, deletes exact runtime resources and reports dataRetained |

Mutations compare both Sandbox UID and `revision` (Kubernetes resourceVersion). A missing Pod, cancelled wait, timeout or disconnected node cannot prove stop. `StopUnverified` blocks start/delete across factory recreation. Stop evidence is stored in controlled Sandbox metadata. Missing/replaced PVCs fail rather than create an empty replacement.

`EnvironmentError.failure` includes code, stage, effect (`not-submitted`, `accepted`, `unknown`), observedState, retry, allocationKey, correlationId and nextAction. An unknown write requires inspection of the original binding before another mutation. Logs exclude tokens, credentials and raw API responses.

See [development](development.md) for tests and [release validation](https://github.com/GuoMonth/dsh-isolated-runtime/releases/download/v0.5.0-rc.1/validation.md) for the tested combination.
