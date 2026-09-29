# DSH Isolated Runtime

Kubernetes-only runtime for the [DSH platform](https://github.com/GuoMonth/dsh-multi-tenant). The platform owns login, authorization and allocation bindings; this repository supplies a narrow Connector, a fixed single-PVC workload and the DSH launcher. Upstream Agent Sandbox core is the only controller.

DSH is pinned to **0.2.0-rc.2**, source `639ed015397290b3745d163aafe02ffee4aa3f84`. Every environment has one external PVC at `/var/lib/dsh/data`, containing `workspace`, `home` and `dsh`. Resources use native CPU/memory requests and limits and one storage capacity.

- [Executable contract](docs/design/environment-contract.zh-CN.md)
- [Runtime implementation](packages/environment-connector/src/runtime.ts), exported as `createAgentEnvironmentRuntime(options)`
- [Fixed workload template](packages/environment-connector/src/template.ts), [platform RBAC](config/runtime/cluster-role.yaml)
- [Contributing and checks](CONTRIBUTING.md), [distribution](docs/distribution.md)
- [Current MVP scope](https://github.com/GuoMonth/dsh-multi-tenant/issues/104)

There is no standalone backend, custom Cell operator, installation CLI or historical state migration. Explicit stop requires observed successful termination of the exact writer and healthy-node evidence; unknown outcomes require inspection of the original allocation. Deletion retains the PVC, namespace and its isolation policy. Node partition recovery is not promised.

This is a local RC candidate, not a published release. Platform installation artifacts consume the pinned Connector, image and upstream core assets. See [中文](README.zh-CN.md).
