# Documentation map

Current entry: [integrated platform startup](https://github.com/GuoMonth/dsh-multi-tenant/blob/main/docs/reference/quickstart.md), [shared regression](https://github.com/GuoMonth/dsh-multi-tenant/blob/4ba252765bccb41314c0bdc6b11bcf60cc0b33ef/docs/evidence/cell-regression-2026-09-20.md). Published standalone instructions describe only their own release.

- [R2 internal Cell Connector](cell-connector.md) — source/artifact boundary; current results in the shared regression report.

- [Platform access configuration (R1)](platform-access.md) — platform mode and verified integration reference.
- [Shared constitution](../CONSTITUTION.md)
- [Internal RuntimePort](design/runtime-port.zh-CN.md) / [Cell adapter](design/cell-adapter.zh-CN.md) — contract; see shared regression for implementation evidence.
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
