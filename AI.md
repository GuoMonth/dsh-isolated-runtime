# AI assistant guide (English)

Answer in the user's language. For the recommended Kubernetes deployment, use [dsh-multi-tenant](https://github.com/GuoMonth/dsh-multi-tenant) for installation and user operations. The reference platform image installs the exact Connector alongside the platform package.

For runtime changes, read [development](docs/en/development.md), [contract](docs/en/contract.md) and [AGENTS.md](AGENTS.md). Runtime owns resource identity and lifecycle; platform owns authorization. Preserve the original binding on unknown results and retain user volumes.

Use the exact workload image in the paired release manifest. See [releasing](docs/en/releasing.md) for Connector provenance and image publication. Record the tested platform/runtime combination and actual checks.
