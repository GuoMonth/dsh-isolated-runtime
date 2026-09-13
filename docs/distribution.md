# Alpha Distribution

Target: v0.2.0-alpha.1. This PR prepares the delivery; it does not itself publish
a GitHub Release, npm package or third-party mirror. The existing v0.1.2 stays
available unchanged. Do not interpret a completed PR as Mac acceptance.

## One Runtime, Multiple Entrances

- GitHub Releases: native host archives, SHA256SUMS, source/image metadata and
  acceptance evidence. Direct downloads do not require host Node or npm.
- GHCR: public Cell and Operator multi-architecture indexes, pinned by digest.
- npm: thin launcher, explicit version, alpha dist-tag, no install/postinstall
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

1. Review and merge the alpha PR only after required CI passes. Keep historical
   protected check names; do not bypass gates to simplify the release.
2. Main builds candidate images once and verifies exact digests. Exact archives
   run through deterministic model/browser, random identity, stop/resume and
   snapshot acceptance. macOS host evidence remains distinct from Docker Desktop.
3. The explicit Publish accepted alpha workflow consumes that candidate and
   publishes original archives and image identities as a GitHub pre-release.
   It also creates a version-bound npm tarball artifact via
   node hack/package-npm.mjs dist npm-dist, without publishing it to npm.
4. Run Publish npm alpha with the successful Publish accepted alpha run ID.
   It checks that run's identity, anonymously downloads and verifies the public
   release, and reproduces the npm tarball from the publication's exact source.
   Only a byte-identical original artifact is published, with the alpha tag.
   The maintainer has explicitly selected 0.2.0-alpha.1 as latest too: after
   registry integrity verification, the workflow adds latest to that same
   version and verifies anonymous resolution. Other alpha versions do not
   automatically move latest. Do not publish packages/cli directly.
5. The maintainer downloads on Mac, configures a real model privately and records
   file write/read plus stop/resume results. Fix failures in a new alpha. Until
   then, evidence must continue to say Docker Desktop/live-model not-run.

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
For this first alpha, both `alpha` and `latest` point to `0.2.0-alpha.1`; the
version remains a prerelease and the GitHub pre-release status is unchanged.
After successful publication, the default entry is `npx dsh-isolated-runtime start`.
If publication succeeds but the final registry check fails,
inspect `npm view dsh-isolated-runtime@0.2.0-alpha.1 dist.integrity` and the run
logs before retrying; npm versions are immutable. Expired Actions artifacts
require a reviewed recovery, not republishing an unverified source checkout.
