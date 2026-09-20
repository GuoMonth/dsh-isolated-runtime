# Documentation map

- [R2 internal Cell Connector](cell-connector.md) — source/artifact boundary; regression pending.

- [Platform access configuration (R1)](platform-access.md) — implementation slice; cluster proof pending.
- [Shared constitution](../CONSTITUTION.md)
- [Internal RuntimePort](design/runtime-port.zh-CN.md) / [Cell adapter](design/cell-adapter.zh-CN.md) — planned integration, not implementation evidence.
- [Current alpha MVP direction and acceptance](./alpha-mvp.md)
- [Go toolchain, lifecycle checks and leak diagnostics](./go-development.md)
- [Local installation](./quickstart.md)
- [AI installation runbook](./ai/local-run.md)
- [Alpha distribution](./distribution.md)

- [Architecture](./specs/architecture.md) — ownership, boundaries, flows, and non-goals.
- [Threat model](./specs/threat-model.md) — guarantees, assumptions, and abuse cases.
- [Namespace contract](./specs/namespace-contract.md) — native capability and policy boundaries.
- [Metrics contract](./specs/metrics.md) — bounded aggregate observability surface.
- [DSH compatibility](../compat/dsh/README.md) — exact upstream evidence and launcher decision.
- [Roadmap](../ROADMAP.md) — current integration sequence.

Executable API details belong in Go types, generated CRDs, and tests rather
than duplicated prose.

Read by task. `archive/`, release ledgers and old acceptance snapshots are historical evidence, not current requirements. For an installed release, use its bundled documentation.

- [R5 allocation implementation](design/r5-allocation.md): create/inspect, immutable intent and deferred checks.

- [R6 exact deletion request](design/r6-deletion.md): identity preconditions and actual data ownership.
