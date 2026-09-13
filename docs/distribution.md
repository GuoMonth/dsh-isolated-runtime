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
4. Confirm npm name/ownership and configure publication credentials separately.
   Inspect the bound tarball, test it against the publicly downloadable release,
   and publish that tarball with npm publish PACKAGE.tgz --access public --tag alpha.
   Do not publish packages/cli directly or assign the prerelease to latest.
5. The maintainer downloads on Mac, configures a real model privately and records
   file write/read plus stop/resume results. Fix failures in a new alpha. Until
   then, evidence must continue to say Docker Desktop/live-model not-run.

The npm package is version-bound; changing a dist-tag does not upgrade an
existing state directory. Cross-release local migration is intentionally not
automated. Security dependency follow-ups from issue #70 are not fixed by this
distribution work and must not be marked complete as part of this PR.
