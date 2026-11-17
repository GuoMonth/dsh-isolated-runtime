# Repository instructions

Read the [runtime contract](docs/en/contract.md) and shared [product principles](CONSTITUTION.md). Requirements and acceptance belong to the linked GitHub Issue.

- Use [development](docs/en/development.md) for toolchain and checks. Commits use DCO (`git commit -s`).
- Preserve owner, Sandbox/PVC UID and Pod identity. Runtime checks resources; platform owns user authorization. Upstream core alone reconciles Pods/Services.
- Tests may depend on existing fixture names; preserve names required by code. Real cluster checks use [test/environment](test/environment/README.md).
- Use the root pnpm workspace for development. TypeScript uses ESM, `.ts` relative imports and the pinned Oxlint style; Go uses gofmt. Existing lint allowances may only shrink.
- PR CI runs `make verify` on Node 24 and Connector checks on Node 24.9 and 26. Cluster and publication workflows are manual.
- Keep docs/en and docs/zh-CN paired, update both languages and run make docs-check. Remove superseded documentation; Git and Releases preserve history.

Implementation: [Connector](packages/environment-connector/src/runtime.ts), [template](packages/environment-connector/src/template.ts). Distribution: [releasing](docs/en/releasing.md). Documentation: [index](docs/en/index.md).
