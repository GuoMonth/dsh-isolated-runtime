# Distribution: Cell integration and historical standalone

Current integration: administrators deploy the platform-mode Operator and runtime-owned Cell images; the platform repository supplies the OIDC npm CLI/container. See [platform startup](https://github.com/GuoMonth/dsh-multi-tenant/blob/main/docs/reference/quickstart.md) and [release coordination](https://github.com/GuoMonth/dsh-multi-tenant/blob/main/docs/reference/release.md). Both repositories use npm **latest** for future authorized releases; alpha maturity remains explicit. This PR does not publish or move any registry tag.

The runtime npm launcher remains the standalone local-cluster experience. It is not required to start the integrated platform and must not install a competing standalone authorizer. Public Cell/Operator digests must match the fixed source and DSH recorded by the platform. The existing `config/platform` overlay is packaged with runtime releases; its source `main` image placeholder must be replaced with the accepted digest.

The archive/host workflow below describes the historical standalone distribution. It is not a new macOS, snapshot or multi-environment gate for the existing-K8s Cell MVP. No Helm or automatic kind setup is introduced. New standalone publication requires a new version and that path's actual artifact acceptance; never republish the existing 0.2.0-alpha.1 bytes with changed docs/code.

## One Runtime, Multiple Entrances

- GitHub Releases: native host archives, SHA256SUMS, source/image metadata and
  acceptance evidence. Direct downloads do not require host Node or npm.
- GHCR: public Cell and Operator multi-architecture indexes, pinned by digest.
- npm: thin launcher, explicit version, latest dist-tag, no install/postinstall
  hooks. Requires Node 22+. Embeds accepted release identities and archive
  checksums; rejects checksum mismatches, unsafe archives and inner mismatch.
- Existing Kubernetes: administrator-managed Kustomize installation, not the
  local kind lifecycle. No automatic cluster adoption.

GHCR package visibility must be public for anonymous pulls. A public source
repository does not automatically make new image packages public. Maintainers
must verify anonymous downloads with an empty Docker credential configuration.
https://docs.github.com/en/packages/working-with-a-github-packages-registry/working-with-the-container-registry

## Dependencies and Network

runtime-files/images.json lists explicit reference-stack images;
images.lock.json records maintainer-resolved digests and native architectures.
Refresh intentionally with node hack/lock-runtime-images.mjs, review the diff,
then run acceptance. Local pulls use the lock and load the upstream aliases
into kind so the reference manifests continue to match. Kubernetes components
and local-path storage bundled inside the kind node are covered by its digest.
Cell and Operator identities come from release.json, not this reference lock.

External download sources still required by this alpha:

| Source | Use |
| --- | --- |
| GitHub Releases / raw.githubusercontent.com | Installation archive, kind/jq, Calico and Envoy manifests |
| ghcr.io | Project images and Dex |
| Docker Hub | kind node and Envoy images |
| quay.io | Calico |
| registry.k8s.io | Optional CSI images; Kubernetes tool download may use redirects |
| dl.k8s.io / kind.sigs.k8s.io / nodejs.org | Private kubectl, kind and Node binaries with upstream checksums |
| registry.npmjs.org | Locked identity and browser dependencies; npm launcher |
| Playwright browser CDN | Chromium for the private browser profile |
| github.com Kubernetes CSI repositories | Exact source commits for optional snapshot manifests |

No China connectivity guarantee or offline-install claim. A one-line npm entry
does not eliminate these downstream downloads. Tools have pinned versions and
upstream checksums, but this first alpha is not a completely vendored asset set.
Third-party GHCR mirroring and alternate download sources remain a separate
release-preparation follow-up: preserve provenance/licenses, all native
architectures and exact manifests, then test anonymous cold-start installation.
Do not install unverified public mirror shortcuts into users' Docker settings.

## Release Sequence

1. Review and merge after lightweight Source standards CI and appropriate local
   behavioral verification. Record local evidence in the PR.
2. Candidate publication is manual, never triggered by a main push. The optional
   Tested OCI promotion workflow builds candidate images once and verifies exact digests. Exact archives
   run through deterministic model/browser, random identity, stop/resume and
   snapshot acceptance. macOS host evidence remains distinct from Docker Desktop.
3. The explicit Publish accepted alpha workflow consumes that candidate and
   publishes original archives and image identities as a GitHub pre-release.
   It also creates a version-bound npm tarball artifact via
   node hack/package-npm.mjs dist npm-dist, without publishing it to npm.
4. Run Publish npm latest with the successful Publish accepted alpha run ID.
   It checks that run's identity, anonymously downloads and verifies the public
   release, and reproduces the npm tarball from the publication's exact source.
   Only a byte-identical original artifact is published, with the latest tag.
   Verify anonymous version/integrity/latest resolution. The old alpha tag is
   historical, not the current release channel. Do not publish packages/cli directly.
5. The maintainer downloads on Mac, configures a real model privately and records
   file write/read plus stop/resume results. Fix failures in a new alpha. Until
   then, evidence must continue to say Docker Desktop/live-model not-run.

Routine PRs do not run remote integration tests, image builds or multi-platform
jobs. Existing publication still requires accepted candidate artifacts; the
manual release workflow is an explicit, potentially expensive operation, not a
merge gate. Local test success must not be forged into a GitHub candidate run.

The npm package is version-bound; changing a dist-tag does not upgrade an
existing state directory. Cross-release local migration is intentionally not
automated. Security dependency follow-ups from issue #70 are not fixed by this
distribution work and must not be marked complete as part of this PR.

## npm Publication Credentials

In GitHub Settings > Secrets and variables > Actions, create a repository
**secret** named `NPM_TOKEN`, not a plaintext Actions variable. The workflow
injects it as `NODE_AUTH_TOKEN` only for the publication step. Do not put the
token in source, a PR, an issue, or a chat message.

Use an npm granular access token with package read/write permission and bypass
2FA enabled for non-interactive publishing. The account/token must be allowed
to create the initially unpublished `dsh-isolated-runtime` package. After the
first publish, narrow permission to that package and rotate short-lived tokens.
See [npm CI credentials](https://docs.npmjs.com/using-private-packages-in-a-ci-cd-workflow/).
Token-based direct publishing is transitional: npm targets January 2027 for
removing bypass-2FA direct publish. Migrate to OIDC trusted publishing before
that change; see the [npm notice](https://github.blog/changelog/2026-07-31-restricting-npm-bypass-2fa-granular-access-tokens/).

Trigger from main, replacing RUN_ID with the successful publication run (not
the main candidate build):

```sh
gh workflow run npm-publish.yml --ref main -f publication_run=RUN_ID
```

Missing credentials stop publication. A failed or wrong workflow, unpublished
GitHub release, invalid evidence, or changed tarball also stops publication.
The action never overwrites an npm version or changes package ownership.
Historically both `alpha` and `latest` selected `0.2.0-alpha.1`; the
version remains a prerelease and the GitHub pre-release status is unchanged.
After successful publication, the default entry is `npx dsh-isolated-runtime start`.
If publication succeeds but the final registry check fails,
inspect `npm view dsh-isolated-runtime@0.2.0-alpha.1 dist.integrity` and the run
logs before retrying; npm versions are immutable. Expired Actions artifacts
require a reviewed recovery, not republishing an unverified source checkout.

## Fixed current dependency

DSH is exactly `0.1.5-rc.2` / `fb2c4b9e698e30edb738bca4cf0618587db7d203`. Public Cell/Operator digest pins and their matching source form the release boundary. The deployed platform image is also digest-pinned. A newer iteration may break APIs, configuration and state formats; declare and validate the new combination, with no historical compatibility/upgrade obligation. Do not overwrite published artifacts or use mutable image tags to hide the change.

## Existing-cluster Cell release (current platform alpha)

This path releases only the fixed Cell/Operator pair and `release.json`; it does
not create host archives, run the standalone installer or publish the runtime
npm launcher. The platform npm release consumes this manifest. A distinct,
unused alpha tag can be used without changing the historical launcher version.

After the exact source and immutable public images have been accepted:

```bash
node hack/cell-release-manifest.mjs vX.Y.Z-alpha.N EXACT_TESTED_SOURCE_SHA \
  ghcr.io/guomonth/dsh-isolated-runtime-cell@sha256:CELL_DIGEST \
  ghcr.io/guomonth/dsh-isolated-runtime-operator@sha256:OPERATOR_DIGEST \
  /private/release.json
```

The generator resolves the DSH baseline from that exact source, validates each
image's repository/digest syntax, and refuses to overwrite its output. It does
not prove provenance, anonymous availability or acceptance; those require the
actual build records and the fixed integration test evidence. Do not stamp
arbitrary public images as accepted. Current tested runtime source is `3bcf68855bf16bcc5043058fa8133d2d2368efac`;
local-registry digests in the shared report are not public release artifacts.

For an authorized publication, create a GitHub prerelease at the tested source
with this `release.json`, the fixed regression evidence and release notes;
verify the downloaded manifest and anonymous image pulls. No automatic workflow
is added for this step. Then pass its tag and both digests to the platform's
manual release workflow. This path deliberately does not call `mvp-publish.yml`
or `npm-publish.yml`, whose archives belong to the standalone product. A changed
source/image combination needs relevant acceptance and an updated platform pin.
