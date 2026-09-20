# Cell Alpha distribution

Runtime `v0.3.0-alpha.1` owns the public Cell/Operator image pair and `release.json`. The npm package `dsh-isolated-runtime@0.3.0-alpha.1` distributes that exact manifest and platform-mode deployment YAML. Platform `dsh-multi-tenant@0.9.0-alpha.1` owns OIDC, membership, sessions and the user-facing server. Both use npm `latest`; GitHub uses ordinary Release / Latest. Alpha maturity is explicit in version names.

## Install on existing Kubernetes

See the [package README](../packages/cell-cli/README.md) for `release` and `manifests` commands. The administrator supplies K8s, enforced CNI policies, storage, namespaces, Gateway API, DNS/TLS and private platform configuration. The CLI prints files only; it has no cluster credentials, network dependencies or install scripts. Apply reviewed YAML with kubectl, then start the platform. Cell image selection remains in the platform tenant configuration and must match the fixed manifest.

DSH is exactly `0.1.5-rc.2` / `fb2c4b9e698e30edb738bca4cf0618587db7d203`. Runtime image/config source is `3bcf68855bf16bcc5043058fa8133d2d2368efac`. `hack/package-cell-manifests.mjs` renders `config/platform` from that exact commit, replacing only the Operator image placeholder with its public digest. The package records the YAML SHA-256. No runtime image is rebuilt for npm publication.

## Publish

Current package source is `packages/cell-cli`; `packages/cli` and `hack/package-npm.mjs` retain historical standalone source only. Do not publish the old directory for a new Cell release.

1. Publish the accepted image pair via the manual `cell-publish.yml` workflow. The existing release's source tag and image identities remain immutable.
2. Bind the exact public release manifest in `packages/cell-cli/release.json`; update the package version and regenerate YAML with `node hack/package-cell-manifests.mjs`. Copy the repository LICENSE.
3. Run package tests, pack/install the actual tarball into a clean consumer and validate its emitted YAML against the target Kubernetes API with server dry-run. Check the source and workflow syntax.
4. After authorized merge, trigger `gh workflow run npm-publish.yml --ref main`. The workflow compares the manifest with the public GitHub release, checks and packs the package, preserves the original tarball, then publishes to npm `latest` using the existing `NPM_TOKEN` Actions secret. It verifies exact registry integrity and the dist-tag with bounded waiting. Existing npm versions may only be verified, never overwritten.
5. The GitHub release receives the original npm tarball and `npm-publication.json`, recording the **package source commit** separately from the already accepted **runtime image source commit**. Existing attachments are compared rather than replaced. Verify the actual public npm install after publication.

## Breaking change from standalone 0.2

The old local-cluster launcher is not the new npm entry. `start/up/stop/uninstall`, automatic kind setup and host archives are removed from the current package. This is not an in-place migration or a macOS compatibility claim. Old standalone users can select exactly `0.2.0-alpha.1` and its [versioned distribution instructions](https://github.com/GuoMonth/dsh-isolated-runtime/blob/v0.2.0-alpha.1/docs/distribution.md). Historical scripts are not Cell release gates.

New iterations may break APIs, configuration and state formats. No historical compatibility, upgrade, HA, recovery or automatic data migration is promised. Public image digests, npm tarballs and release source tags remain immutable.
