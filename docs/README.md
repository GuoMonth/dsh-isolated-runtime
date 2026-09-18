# Design documents

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
- [Roadmap](../ROADMAP.md) — milestone sequence and review gates.

Executable API details belong in Go types, generated CRDs, and tests rather
than duplicated prose.
