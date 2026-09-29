# Contributing

Read [CONSTITUTION.md](CONSTITUTION.md) and the [environment contract](docs/design/environment-contract.zh-CN.md). The latest user authorization and [MVP Issue #104](https://github.com/GuoMonth/dsh-multi-tenant/issues/104) own scope and acceptance. Breaking Alpha changes are allowed; do not retain old backends, compatibility layers or migration paths.

Use Go 1.27.1 (`dev-run go=1.27 -- make verify`), Node 24 and the committed Connector lockfile. Install its development dependencies with `npm ci --prefix packages/environment-connector`. `make verify` builds, runs Go race tests and Connector tests, vets Go, and checks source standards. `make verify-dsh` validates the exact upstream DSH source seam. Real cluster acceptance is described in [test/environment](test/environment/README.md); pure fixtures do not replace that evidence.

CI runs Source standards only. Record actual local behavior checks, artifact identities and uncovered cases in the PR. Publish only an explicitly authorized and locally verified release; see [distribution](docs/distribution.md).

Runtime owns resource identity and lifecycle; platform owns authorization. Keep exact owner, Sandbox UID, PVC UID and Pod identity. Runtime provisions the dedicated namespace and fixed resources without a background reconciler. Upstream core alone manages Pods and Services. Never infer successful stop from absence, and never replace missing data automatically.

Use `git commit -s` for DCO. The [Go guide](docs/go-development.md) explains the toolchain. See [中文](CONTRIBUTING.zh-CN.md).
