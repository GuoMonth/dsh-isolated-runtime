# v0.1.0 — first local MVP

This pre-release provides a Linux x86_64 local kind experience using DSH's own
browser UI: create a Cell, authenticate, configure a model, use tools and files,
retain state across Pod replacement and optionally restore a writer-stopped CSI
snapshot into a fresh Cell.

Download the archive and SHA256SUMS, verify the checksum and follow the included
English or Chinese Quickstart. Docker is required. Model usage requires your own
credentials. The archive's release.json records the exact DSH source, image
digests and candidate workflow. `deterministic.json` records successful automatic
acceptance on these exact artifacts. `release-acceptance.json` records the release
decision and explicitly marks real-model acceptance as not yet run.
Images retain their original SBOM/provenance and are not rebuilt for publication.

## Publication and acceptance

Distribution uses the GitHub `v0.1.0` tag and pre-release assets, with Cell and
Operator images in GHCR. This project does not publish an npm package.

All candidate automatic gates passed, including the real DSH browser/tool/file/
restart/restore journey with a deterministic model HTTP fixture, CSI lifecycle,
isolation and the retained 50 Cell regression. A real DeepSeek model smoke has
**not been run**. The maintainer explicitly chose to publish this MVP first,
perform live end-to-end testing after publication and address findings in patch
releases. This replaces the original pre-release live-model requirement; the
unchanged archive retains the earlier planning ledger as historical context.

本次通过 GitHub Tag + Release 发布安装包，Cell/Operator 镜像发布到 GHCR，不发布 npm 包。
自动验收已通过；真实 DeepSeek 模型端到端测试尚未执行，由维护者在发布后完成，发现问题后
发布修复版本。本次决定取代原定的发布前真实模型门禁，已验收安装包保持原文件。

DSH 0.1.3-alpha.1 is built from its pinned official source because the matching
npm artifact was not published at baseline selection. A recorded, hashed Cell
integration patch enables DSH's native settings at the authenticated Cell
hostname. The release manifest records this source and patch, without claiming
an unavailable upstream npm integrity value.

Only this DSH baseline and linux/amd64 are supported. Snapshots are crash-consistent,
exclude private credentials and require fresh authorization and provider setup
after restore. There is no old-version migration support, production capacity
promise, HA or multi-cluster scope. Local demo teardown deletes its cluster/data.

The upstream 0.1.3-alpha.1 release notes report a session-loading performance
regression. This MVP keeps that upstream limitation explicit; it does not add a
session engine or compatibility layer to work around it.
