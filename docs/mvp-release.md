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
- The current browser proof does not assert completed model answers/tools.
- Main protection names the previous DSH version explicitly; change it together
  with the workflow check name while preserving required checks.

## Accepted defaults

Local demo uses an isolated kubeconfig and browser profile, loopback endpoints,
test OIDC identity, and existing kind/Calico/Envoy/Dex fixtures. Snapshots opt in;
metrics off. Users configure their own model in DSH. Closing the browser retains
data; explicit demo teardown deletes only demo-owned resources and data.
