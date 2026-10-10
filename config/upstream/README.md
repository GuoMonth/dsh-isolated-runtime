# Upstream Agent Sandbox core (English)

`agent-sandbox-v1.0.3.yaml` is the official release asset. [source.json](source.json) records its checksum and Linux/amd64 controller digest. Platform installation packaging verifies the original file and pins the sole controller image.

The upstream asset owns its generated CRD/RBAC. The separate platform runtime role is [cluster-role.yaml](../runtime/cluster-role.yaml). Deploy through [dsh-multi-tenant](https://github.com/GuoMonth/dsh-multi-tenant).
