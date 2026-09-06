# v0.1.1 — local MVP with Apple Silicon

This MVP pre-release adds an Apple Silicon macOS demo package while retaining the
Linux x86_64 package. Cell and Operator images contain native Linux amd64 and
arm64 variants under the same digest-pinned OCI indexes. They retain their SBOM
and provenance; publication copies the accepted archives and attaches version
tags to the accepted indexes without rebuilding.

On Apple Silicon, install and start Docker Desktop, download
`dsh-isolated-runtime-v0.1.1-darwin-arm64.tar.gz` and follow the included
[Quickstart](https://github.com/GuoMonth/dsh-isolated-runtime/blob/main/docs/quickstart.md).
The demo downloads native tools privately, uses stock macOS checksums/locks/TLS,
and opens native Chromium with isolated DNS, test-certificate handling and a
private profile. Closing Chromium retains data; `demo down` destroys only the
owned demo cluster and its data. No Rosetta, Homebrew or global hosts/trust edits
are required. The commands remain `demo up`, `demo open`, `demo down`.

## Distribution and evidence

GitHub Tag + Release provides both archives, combined SHA256SUMS, per-platform
manifests/evidence and a public release manifest. GHCR provides the two native
container architectures. This project does not publish an npm package.

Automatic acceptance covers native image smoke and the complete real DSH
login/model-fixture/tool/attachment/restart/CSI-restore journey on Linux amd64 and
arm64. The macOS arm64 proof covers native private tools, stock TLS, lock and
process ownership, and actual Chromium DNS/TLS/profile lifecycle. The existing
Linux isolation, lifecycle and 50 Cell regressions remain required.

**Full Docker Desktop end-to-end testing and a real DeepSeek model smoke have not
been run.** GitHub's hosted Apple Silicon runners cannot run nested virtualization;
the complete container journey is tested on a native Linux arm64 runner. Full Mac
Docker Desktop and real-model testing remain maintainer follow-ups after this
pre-release. `release-acceptance.json` states these limits explicitly. The manual
live-model workflow remains available; model credentials are supplied by the user.

本次增加 Apple Silicon 安装包和原生 arm64 容器镜像，继续通过 GitHub Release + GHCR 发布。
Mac 端工具及实际 Chromium、Linux arm64 核心链路均进入自动验收。完整 Docker Desktop 和
真实模型端到端测试尚未执行，留给维护者在发布后验证，发现问题后发布修复版本。

## Retained scope

DSH remains pinned to 0.1.3-alpha.1, built from its recorded official source because
the matching npm package was unavailable at baseline selection. The hashed native
settings patch, transparent proxy, data/private separation and Cell/CellSnapshot
v1alpha1 API remain unchanged. Snapshots are writer-stopped crash-consistent,
exclude private credentials and require fresh authorization/provider setup after
restore. No old-version migration, HA, multi-cluster or production capacity promise
is added. The upstream session-loading performance regression remains a known
limitation. Existing v0.1.0 release artifacts are preserved.

Runner limitation: https://docs.github.com/en/actions/reference/runners/github-hosted-runners#limitations-for-arm64-macos-runners
