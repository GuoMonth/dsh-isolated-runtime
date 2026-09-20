# DSH Isolated Runtime

A thin launcher for the exact GitHub Release and GHCR images, not a DSH plugin.
Requires Node.js 22+ and a running Docker engine. Supported hosts: Linux x86_64
and Apple Silicon macOS using a native arm64 terminal.

After the alpha package is published:

```sh
npx dsh-isolated-runtime@0.2.0-alpha.1 start
```

Use `start --no-open` on a headless host. The first invocation downloads the
verified release archive; the runtime downloads pinned tools and container
images. No Docker login or local source build is required. Model credentials
must be configured privately in DSH; this package does not provide a model.

`status --json`, `doctor --json`, `stop`, `up`, and `uninstall --yes` use the same
runtime entry as a direct release download. `stop` retains data. Uninstall
deletes all local cluster data. Never uninstall merely to retry a failed start.

The npm package embeds release checksums; it never resolves a moving `latest`
runtime. Future authorized releases use the `latest` npm dist-tag; alpha is maturity, not the channel. The source package deliberately
cannot be published until `hack/package-npm.mjs` binds accepted artifacts.

[AI installation guide](https://github.com/GuoMonth/dsh-isolated-runtime/blob/main/docs/ai/local-run.md)

For the OIDC + Cell integration, administrators deploy `config/platform` and use the [multi-tenant platform CLI](https://github.com/GuoMonth/dsh-multi-tenant#readme). Do not run this standalone installer alongside it. The existing 0.2.0-alpha.1 release is historical; changed source is not a republished artifact.
