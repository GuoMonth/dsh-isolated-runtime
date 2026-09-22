# v0.2.0-alpha.1 - Local Installation Alpha

This is the first productized local-installation alpha, not a production
certification. The DSH baseline remains 0.1.5-rc.2 at
fb2c4b9e698e30edb738bca4cf0618587db7d203.

- A formal dsh-runtime entry with status/doctor JSON, data-preserving stop and
  explicit uninstall confirmation.
- Independent random local login and OIDC client credentials.
- An npm thin launcher bound to the exact release archive checksum and image
  identities; direct release downloads remain supported.
- Versioned human and AI installation documentation.
- Maintainer-resolved third-party image identities; no automatic dependency PRs.

The legacy state path is recognized but cross-release in-place migration is
not supported. Do not uninstall an old installation before exporting needed
data. The legacy demo executable is a compatibility alias only; its down
command now requires --yes. Historical resource identifiers remain unchanged.

GitHub publication and npm publication are separate explicit maintainer actions.
The npm tarball is prepared from accepted release artifacts, never from an
unbound source directory, and uses the alpha dist-tag rather than latest.
No package is published merely by opening or merging this PR.

Automatic candidate evidence covers the real DSH flow with a deterministic
model, including local credentials and stop/resume persistence. Full Mac Docker
Desktop and real-model acceptance remain not-run until the maintainer supplies
that evidence after alpha publication. Neither is implied by Linux CI success.

首个产品化本地安装 alpha，保留精确 DSH 基线和现有 Cell/CellSnapshot API。
正式命令、随机本地凭据、npm 薄启动器与 AI 入口统一在同一份发行物上。
Mac 实机和真实模型测试由维护者在发布后验证；不宣称生产能力已全部验收。
