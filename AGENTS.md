# Repository instructions

Product boundaries: [shared constitution](CONSTITUTION.md). Interface and phase design: [AgentEnvironment](https://github.com/GuoMonth/dsh-multi-tenant/blob/main/docs/design/agent-environment.zh-CN.md); its linked Issues own acceptance and progress. Read the paired platform checkout when a task changes both repositories.

## Development

- Check commands: [CONTRIBUTING.md](CONTRIBUTING.md); toolchain pins: [Go development](docs/go-development.md). Contributions use DCO (git commit -s).
- Preserve immutable owner linkage and exact workspace, storage and Pod identity. Runtime matches resources; platform owns user authorization. Kubernetes manages resource mechanics.
- Preserve fixture/state names still required by current code and tests; names do not make historical product backends current requirements.
- Source currently implements Cell. The [local Sandbox trial](docs/evidence/agent-sandbox-local-2026-09-23.md) is test-only evidence, not production integration or stop/start acceptance.
- Automatic CI is Source standards only. Behavioral tests run locally according to changed surfaces; cluster diagnostics and publication workflows are manual.

## References

Current implementation: [RuntimePort](docs/design/runtime-port.zh-CN.md), [Cell adapter](docs/design/cell-adapter.zh-CN.md), [architecture](docs/specs/architecture.md). Distribution: [release instructions](docs/distribution.md). Other documents: [index](docs/README.md). Historical standalone guides live under docs/archive and do not supersede the current design.
