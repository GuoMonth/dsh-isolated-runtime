# Repository instructions

Read the [runtime contract](docs/en/contract.md) and shared [product principles](CONSTITUTION.md). Requirements and acceptance belong to the linked GitHub Issue.

- Use [development](docs/en/development.md) for toolchain and checks. Commits use DCO (`git commit -s`).
- Preserve owner, Sandbox/PVC UID and Pod identity. Runtime checks resources; platform owns user authorization. Upstream core alone reconciles Pods/Services.
- Tests may depend on existing fixture names; preserve names required by code. Real cluster checks use [test/environment](test/environment/README.md).
- Use the root pnpm workspace, Node 24.22.3 or later in 24.x, or Node 26.13.1 or later. Use strict TypeScript ESM with `.ts` relative imports and erasable syntax in Node tooling. Run pinned Oxlint and extend no lint allowance.
- Use gofmt, go vet and the pinned golangci-lint with the Go version in go.mod. Pass context through blocking operations, wrap errors with the operation, keep wire types explicit, and join goroutines on shutdown.
- PR CI runs `make verify` on Node 24.22.3 and 26.13.1. Native DSH checks run on both when the integration changes. Cluster and publication workflows are manual.
- Keep docs/en and docs/zh-CN paired, update both languages and run make docs-check. Remove superseded documentation; Git and Releases preserve history.

Implementation: [Connector](packages/environment-connector/src/runtime.ts), [template](packages/environment-connector/src/template.ts). Distribution: [releasing](docs/en/releasing.md). Documentation: [index](docs/en/index.md).
