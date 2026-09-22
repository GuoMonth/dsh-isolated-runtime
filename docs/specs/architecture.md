# Cell architecture

## Ownership

A `Cell` is a namespaced boundary for one DSH instance and its persistent data. The namespace is the tenant boundary. This repository translates Cell intent into Kubernetes resources; it does not provide a second scheduler or user identity system.

| Concern | Owner |
| --- | --- |
| User identity, OIDC, membership, authorization and user sessions | multi-tenant platform |
| Cell resources, images, lifecycle and verified internal transport | this repository |
| Application protocols, sessions, tools and model calls | DSH |
| Pod scheduling, services, network policy and volumes | Kubernetes and its installed providers |
| TLS termination and ingress routing | Gateway API implementation |

The Cell API does not accept a Pod name, UID, IP, Node, route or session as a user-selected target. The runtime resolves an instance from Kubernetes state and checks its identity before forwarding. The standard Kubernetes Pod is the POC execution boundary. The source-only sandbox reduction in [runtime PR #93](https://github.com/GuoMonth/dsh-isolated-runtime/pull/93) is a candidate change; it is not part of the published npm `0.3.0-alpha.1` package or its fixed manifests.

## Request path

```text
Browser → Gateway (TLS and routing) → multi-tenant platform (OIDC and authorization)
        → Connector → verified Cell Pod → DSH
```

Gateway provides TLS termination and routing in the integrated flow. OIDC login and user authorization belong to the platform. The Connector resolves the platform-authorized Cell reference, validates the current Kubernetes identity chain (including the Cell UID and Pod ownership), and proxies HTTP, WebSocket and streaming traffic to the selected Pod. The Go proxy keeps the DSH launch token in process memory and uses it for the local bootstrap exchange; it does not place it in a public URL or logs. DSH retains ownership of its application protocol and session behavior.

The network policy permits the platform Connector path to reach the Cell proxy port. The DSH listener remains loopback-only within the Pod. NetworkPolicy enforcement depends on a CNI that enforces the policy; a rendered policy alone is not proof of isolation.

## Identity and data boundaries

The Kubernetes namespace and immutable Cell UID identify a Cell instance. Recreated resources with the same name but a different UID are different instances. Before proxying, the Connector checks the Cell and owned Pod identity against live Kubernetes state; stale or mismatched targets fail closed.

Tenant data and private runtime state use separate storage boundaries. Provider credentials belong to the Cell's DSH private state or an explicitly configured same-namespace credentialsRef Secret; the platform's OIDC secret is separate and must never be mounted into a user Cell. Keep provider credentials out of Cell status, logs and shared tenant data. Kubernetes objects and the installed storage/network providers remain the authority for resource state and enforcement.

Ordinary Pods do not protect against a compromised node, kernel, cluster administrator, or storage/network provider. This POC does not claim that boundary.

## Historical implementation

Earlier source versions included standalone OIDC/RBAC access, a `cell-authorizer` with SubjectAccessReview, and CellSnapshot/restore flows. Those are historical implementation details, not the current integrated platform request path or current MVP acceptance gates. Some corresponding code and release artifacts remain in the repository; this documentation change does not remove or alter them. See [archived standalone alpha documentation](../archive/standalone-alpha1/README.md) for version-specific context.
