# Repository instructions

[CONSTITUTION.md](CONSTITUTION.md) links the shared product principles. This repository owns Kubernetes Cell resources and the runtime boundary. Current scope is [Cell MVP](docs/alpha-mvp.md).

## Task routing

- Integration: [RuntimePort](docs/design/runtime-port.zh-CN.md) and [Cell adapter](docs/design/cell-adapter.zh-CN.md).
- Development checks: [CONTRIBUTING.md](CONTRIBUTING.md); Go-specific details in [Go development](docs/go-development.md).
- Installing an existing release: [installation runbook](docs/ai/local-run.md), using the selected release's bundled docs.
- Other docs: [index](docs/README.md). Historical milestones are not current gates.

## Repository-specific constraints

- Native Kubernetes/Gateway/CSI own resource mechanics. Keep application protocols in DSH and user authorization in the platform; enforce exact resource ownership at the runtime boundary.
- `dsh-runtime` is the public local command. Legacy fixture/state names may still participate in ownership checks; do not erase them or state to make tests pass.
- Use isolated `DSH_RUNTIME_HOME` and only clean task-owned resources. Source checkout is not an installable release: artifact acceptance binds actual images, archives and source SHA.
- Automatic CI is only `Source standards`; do not introduce automatic builds, cluster runs or publication. Local checks can proceed within the task; releases, deployment and data deletion follow existing user authorization.
- Contributions use DCO (`git commit -s`). Report untested environments without inferring them from another platform's fixtures.
