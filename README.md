# DSH Isolated Runtime

[简体中文](README.zh-CN.md)

The Kubernetes runtime for self-hosted, multi-user [DeepSeek Harness](https://github.com/deepseek-ai/deepseek-harness). It provides the workload image, launcher and in-process Connector used by [dsh-multi-tenant](https://github.com/GuoMonth/dsh-multi-tenant).

**Install through the platform repository.** Its npm package bundles the Connector, CLI and Helm chart. The RC combination is runtime **0.4.0-rc.1**, platform **0.10.0-rc.1**, DSH **0.2.0-rc.2**.

## What runtime does

Runtime creates each authorized user's fixed Kubernetes resources, verifies their owner and UIDs, handles explicit stop/start/delete and forwards native DSH traffic. Upstream Agent Sandbox core is the sole Pod/Service controller; the platform owns OIDC and user authorization.

Each environment has one PVC at `/var/lib/dsh/data`, containing `workspace/`, `home/` and `dsh/`. Normal stop/start and Pod recreation retain files, conversations, configuration and tool credentials. Deletion retains user storage. See the [contract](docs/en/contract.md) for states and failure handling.

The RC supports fresh Linux/amd64 installations on one node, with one platform replica and trusted organization members. Node-partition recovery, backups, HA and data migration require separate work.

## Documentation

- [English guides](docs/en/index.md) · [中文指南](docs/zh-CN/index.md)
- [Development](docs/en/development.md) · [Releasing](docs/en/releasing.md)
- [Source and image releases](https://github.com/GuoMonth/dsh-isolated-runtime/releases)
- [Platform installation](https://github.com/GuoMonth/dsh-multi-tenant) · [AI assistant guide (English)](AI.md)

MIT licensed.
