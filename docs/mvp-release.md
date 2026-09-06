# MVP v0.1.0 delivery ledger

Target: Linux x86_64 local kind experience, one current DSH baseline, existing
Cell/CellSnapshot v1alpha1 APIs. No fleet platform or historical compatibility.

Milestone: https://github.com/GuoMonth/dsh-isolated-runtime/milestone/7

- #50: exact DSH baseline and stable required checks
- #51: real DSH UI/model/tool/file/restart/restore acceptance
- #52: local demo and capability-based installation
- #53: immutable release bundle and bilingual Quickstart
- #54: final exact-artifact review, live-model smoke, GO and publication

## Release gate

Build Cell and Operator once. Run all existing gates and the MVP journey using
those digests. Generate and test the installation/demo archive once. Run the live
DeepSeek smoke manually on that same candidate. Record source SHA, both image
digests, archive checksum, model and CI evidence without credentials. Only then
publish v0.1.0 as a pre-release and close the milestone. Missing live credentials
or any failed gate keeps the retrospective open.

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
