# Repository instructions

Product boundaries: [shared constitution](CONSTITUTION.md). Interface and phase design: [AgentEnvironment](https://github.com/GuoMonth/dsh-multi-tenant/blob/main/docs/design/agent-environment.zh-CN.md); its linked Issues own acceptance and progress. Read the paired platform checkout when a task changes both repositories.

## Development

- Check commands: [CONTRIBUTING.md](CONTRIBUTING.md); toolchain pins: [Go development](docs/go-development.md). Contributions use DCO (git commit -s).
- Preserve immutable owner linkage and exact workspace, storage and Pod identity. Runtime matches resources; platform owns user authorization. Kubernetes manages resource mechanics.
- Preserve fixture/state names still required by current code and tests; names do not make historical product backends current requirements.
- Source implements AgentEnvironment on upstream core. The [environment contract](docs/design/environment-contract.zh-CN.md) and [real cluster harness](test/environment/README.md) define the current seam; historical dual-volume trials are not current acceptance.
- Automatic CI is Source standards only. Behavioral tests run locally according to changed surfaces; cluster diagnostics and publication workflows are manual.

## References

Current implementation: [environment contract](docs/design/environment-contract.zh-CN.md), [Connector](packages/environment-connector/src/runtime.ts), [fixed template](packages/environment-connector/src/template.ts). Distribution: [release instructions](docs/distribution.md). Other documents: [index](docs/README.md). Historical standalone guides live under docs/archive and do not supersede the current design.
