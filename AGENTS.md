# Repository instructions

Read the [runtime contract](docs/en/contract.md) and shared [product principles](CONSTITUTION.md). Requirements and acceptance belong to the linked GitHub Issue.

- Use [development](docs/en/development.md) for toolchain and checks. Commits use DCO (`git commit -s`).
- Preserve owner, Sandbox/PVC UID and Pod identity. Runtime checks resources; platform owns user authorization. Upstream core alone reconciles Pods/Services.
- Tests may depend on existing fixture names; preserve names required by code. Real cluster checks use [test/environment](test/environment/README.md).
- CI runs source standards and documentation checks. Choose behavioral checks for the changed surface; cluster and publication workflows are manual.
- Keep docs/en and docs/zh-CN paired, update both languages and run make docs-check. Remove superseded documentation; Git and Releases preserve history.

Implementation: [Connector](packages/environment-connector/src/runtime.ts), [template](packages/environment-connector/src/template.ts). Distribution: [releasing](docs/en/releasing.md). Documentation: [index](docs/en/index.md).
