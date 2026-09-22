# Repository instructions

[CONSTITUTION.md](CONSTITUTION.md) links the shared product principles. This repository owns Kubernetes Cell resources and the runtime boundary. Current scope is [Cell MVP](docs/alpha-mvp.md); sequence and acceptance live in the multi-tenant platform's [Issue #82](https://github.com/GuoMonth/dsh-multi-tenant/issues/82) and [Issue #99](https://github.com/GuoMonth/dsh-multi-tenant/issues/99).

## Task routing

- Integration boundary: [RuntimePort](docs/design/runtime-port.zh-CN.md), [Cell adapter](docs/design/cell-adapter.zh-CN.md), and [architecture](docs/specs/architecture.md).
- Development checks: [CONTRIBUTING.md](CONTRIBUTING.md); Go details in [Go development](docs/go-development.md).
- Current release instructions: [distribution](docs/distribution.md) and [documentation index](docs/README.md). Historical standalone instructions are under `docs/archive/`.

## Repository-specific constraints

- Kubernetes/Gateway/CSI own resource mechanics. The platform owns OIDC and user authorization; DSH owns application protocols. Validate exact Cell and Pod identity at the runtime boundary.
- Preserve ownership checks and any legacy fixture/state names still used by current code or tests; their presence does not make historical flows current product requirements.
- Treat published packages, images and manifests as immutable release evidence. A source checkout or candidate PR is not an installable release; artifact acceptance binds actual images, package contents and source SHA.
- Automatic CI is only `Source standards`; do not introduce automatic builds, cluster runs or publication. Local checks can proceed within the task; releases, deployment and data deletion follow existing user authorization.
- Contributions use DCO (`git commit -s`). Report untested environments without inferring them from another platform's fixtures.
