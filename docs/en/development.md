# Develop DSH runtime

[简体中文](../zh-CN/development.md) · [Guides](index.md)

Use Go 1.27.1, Node 24+ and the committed npm lockfiles. Make and the launcher image builder pin the Go toolchain. `dev-run` is an optional local toolchain wrapper; standard Go/Node installations work too.

```sh
npm ci --prefix packages/environment-connector
make docs-check
make verify
make verify-dsh
```

`make verify` builds Go and Connector, runs Go race/Connector tests, runs vet, then checks source standards and documentation. `make verify-dsh` installs the pinned DSH package and checks its settings patch and launcher transport. CI runs source/documentation standards and workflow syntax.

`make docs-check` checks paths, anchors, paired languages and command/version agreement. Set `DSH_PLATFORM_DOCS` and `DSH_RUNTIME_DOCS` to the paired checkouts for cross-repository links. Technical harness READMEs are English; public guides have both languages.

The launcher reaps its child, closes proxy transports and owns hijacked connections. Every goroutine needs an owner and exit condition. Forced SIGKILL is not a successful environment stop. Go tests use the race detector and goleak; optional pprof captures stay private.

Real Kubernetes lifecycle/volume/network checks use [test/environment](../../test/environment/README.md). Browser and upstream package smoke use [test/rc2](../../test/rc2/README.md); the settings patch is described in [compat/dsh](../../compat/dsh/README.md). Use disposable fixtures and one cluster write owner. Model/tool authorization, OIDC and two-user acceptance use the [platform](https://github.com/GuoMonth/dsh-multi-tenant) harness.

Use `git commit -s`. In the PR, record actual commands, artifact identities and untested cases. Package and image preparation are in [releasing](releasing.md).
