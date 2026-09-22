# Documentation map

Start with the [integrated platform setup](https://github.com/GuoMonth/dsh-multi-tenant/blob/main/docs/reference/quickstart.md), the [shared regression evidence](https://github.com/GuoMonth/dsh-multi-tenant/blob/4ba252765bccb41314c0bdc6b11bcf60cc0b33ef/docs/evidence/cell-regression-2026-09-20.md), and [Cell MVP scope](alpha-mvp.md). The current user identity and OIDC path belongs to the multi-tenant platform. Gateway provides TLS and routing; the Connector validates and proxies to the selected Cell instance.

- [Shared constitution](../CONSTITUTION.md)
- [RuntimePort](design/runtime-port.zh-CN.md) / [Cell adapter](design/cell-adapter.zh-CN.md) — internal interface and resource boundary
- [Platform access configuration](platform-access.md)
- [Cell Connector](cell-connector.md)
- [Cell MVP scope and acceptance](alpha-mvp.md)
- [Architecture](specs/architecture.md) and [threat model](specs/threat-model.md)
- [Go development](go-development.md)
- [Current distribution contract](distribution.md) and [Cell CLI source README](../packages/cell-cli/README.md)
- [Exact DSH baseline](../compat/dsh/README.md)
- [Current roadmap](../ROADMAP.md)

The published npm `0.3.0-alpha.1` package and its fixed `operator.yaml` / `release.json` describe that release. Source changes such as [PR #93](https://github.com/GuoMonth/dsh-isolated-runtime/pull/93) are candidates until separately released. `archive/` contains version-specific standalone installation, snapshot/restore, and earlier implementation/planning material; it is not the current entry point or a current acceptance checklist.
