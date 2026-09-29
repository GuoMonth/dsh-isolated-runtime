# Upstream core input

`agent-sandbox-v1.0.3.yaml` is the unmodified official release asset; `source.json` records its SHA-256 and the linux/amd64 controller digest actually loaded in the dedicated kind cluster. Installation packaging verifies the original checksum, then substitutes the recorded digest for the sole controller image tag. It must not deploy extensions controllers or a second product controller.

The pinned upstream asset contains its own generated CRD and RBAC. This repository does not regenerate or fork it. Runtime's separate platform role is `../runtime/cluster-role.yaml`. Full platform installation and installed-resource evidence belong to the platform chart and E acceptance.
