# MVP delivery ledger

Current target: Linux x86_64 and Apple Silicon macOS local kind experience, one
current DSH baseline, existing Cell/CellSnapshot v1alpha1 APIs. No fleet platform
or historical compatibility. v0.1.0 and milestone 7 are complete; v0.1.1 adds the
Apple Silicon host package and native Linux arm64 images.

The v0.1.2 candidate advances the exact DSH source baseline to 0.1.5-rc.2,
commit fb2c4b9e698e30edb738bca4cf0618587db7d203. The existing settings patch
applies unchanged. Compatibility acceptance adds upstream V2-to-V3 migration
and log-publication tests. V3 sessions cannot be read by the old DSH version;
cross-version CellSnapshot restore and in-place demo release changes remain
unsupported. Release-specific behavior and upgrade limits are recorded in
[release notes](release-notes.md).

Milestone: https://github.com/GuoMonth/dsh-isolated-runtime/milestone/7

- #50: exact DSH baseline and stable required checks
- #51: real DSH UI/model/tool/file/restart/restore acceptance
- #52: local demo and capability-based installation
- #53: immutable release bundle and bilingual Quickstart
- #54: final exact-artifact review, GO and publication; live-model testing follows publication

## Release gate

Build Cell and Operator once. Run all existing gates and the MVP journey using
those digests. Generate and test the installation/demo archive once. Record source
SHA, both image digests, archive checksum and deterministic CI evidence without
credentials. Publish the version in `VERSION` as a GitHub pre-release with the original archive and
the same GHCR images, verify anonymous downloads, then close the milestone.

The maintainer's 2026-09-06 decision replaces the original pre-release live-model
gate: publish the MVP first, run real-model end-to-end testing after publication,
and release fixes as needed. The release must explicitly report that live testing
has not run. The manual live-model workflow remains available to collect evidence
later; absent credentials do not block this MVP publication. Automatic candidate
failures still block publication. This project has no npm distribution.

## Current findings

- Upstream dsh-v0.1.3-alpha.1 is the latest release at implementation start.
  Its commit is d347e703908d0406b7a7ef80e3a0e594d86b2215.
- The official npm registry does not yet publish 0.1.3-alpha.1 (verified directly,
  including the tarball URL). Build the pinned official source and its runtime
  closure instead; do not silently substitute 0.1.2 or claim npm integrity.
- The new MVP proof adds completed model/tool/attachment checks to the retained browser proof.
- Main protection now requires `Exact DSH compatibility`; the other required checks remain.
- Upstream native settings are disabled at non-loopback hostnames. A hashed,
  one-site Cell integration patch enables its existing settings mirror behind
  the existing Gateway/Cell authorization. No new UI, API, or proxy rewrite.
- The native UI's request burst exceeded client-go's default 5 QPS / 10 burst
  budget in the synchronous authorizer. Its bounded client budget is now
  100 QPS / 200 burst; authorization still reads current state and performs a
  fresh SAR on every request, with the same fail-closed Gateway deadline.
- The 50 Cell regression exposed a snapshot acceptance race after the snapshot
  acquired its own writer fence. Acceptance now revalidates the persisted
  source identity and storage binding without requiring the deliberately
  fenced Cell to remain Ready. A controller interleaving test reproduces it.
- Exact-archive deterministic acceptance passed locally and in candidate CI
  for f767ce4. Subsequent fixes require a new complete candidate result; these
  earlier successes are supporting evidence, not a final release GO.
- Main candidate acceptance found a pinned Envoy cookie-contract error: its
  FNV-1a suffix uses unpadded hexadecimal, so it can have 1–8 digits. The launcher
  and browser proof now recognize all emitted widths. Regression tests first
  reproduced short credential cookies crossing the proxy, then verify request
  filtering and prevention of child response-cookie overwrites. The real Gateway
  gate selects a Kubernetes-assigned policy UID with a short default suffix
  before login, so every run covers this case without overriding cookie names.

## Accepted defaults

Local demo uses an isolated kubeconfig and browser profile, loopback endpoints,
test OIDC identity, and existing kind/Calico/Envoy/Dex fixtures. Snapshots opt in;
metrics off. Users configure their own model in DSH. Closing the browser retains
data; explicit demo teardown deletes only demo-owned resources and data.

## Apple Silicon follow-up

Native Linux amd64 and arm64 image builds are assembled into shared OCI indexes.
Each host archive has a platform manifest and an exact-archive deterministic
proof. A separate macOS arm64 job verifies native tools, stock TLS/checksums,
exclusive locks, owned-process cleanup and actual Chromium profile lifecycle.
Full Docker Desktop and real-model end-to-end testing remain post-release manual
follow-ups and are explicitly recorded as not run. Manual publication dispatch
is the maintainer's release action; the closed v0.1.0 review is not reused as an
approval record for later patch releases.
