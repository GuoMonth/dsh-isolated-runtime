# v0.1.2 - DSH 0.1.5-rc.2

This pre-release upgrades the exact DSH source baseline from 0.1.3-alpha.1 to
0.1.5-rc.2 (fb2c4b9e698e30edb738bca4cf0618587db7d203). The source archive and
lockfile are checksum-pinned. The existing Cell settings integration patch still
applies without changes. Linux x86_64 and Apple Silicon host packages retain
native Linux amd64/arm64 Cell and Operator images.

DSH brings session-loading improvements, Web reconnection fixes, general file
uploads and the new file-preview sidebar. The Cell/CellSnapshot API and the
Gateway OIDC/RBAC access path retain their existing contracts.

## Existing data and upgrades

DSH now uses session format V3. Upstream migrates supported older logs into new
logs while retaining the originals; upgraded sessions cannot be read by the old
DSH version. Back up existing data before changing a Cell image. Switching back
to an old image is not a supported rollback of upgraded sessions. CellSnapshot
restore remains bound to the recorded DSH version and image digest, so old
snapshots cannot be restored directly into this new baseline.

The local demo refuses to reuse a state directory from another release. Use a
separate DSH_DEMO_HOME to try this release and stop the old demo's browser and
port forwards first because the local ports are shared. Export needed files
before any explicit demo down, which deletes the demo cluster and its data.
There is no automatic in-place demo upgrade. Existing installations do not
automatically replace their digest-pinned Cell images.

## Verification and distribution

Publication uses the accepted immutable archives and image indexes without
rebuilding. Required candidate checks cover DSH compatibility, image startup,
Gateway login, deterministic model streaming, real DSH file tools and attachment
reads, restart persistence, CSI restore, and the existing isolation regressions.
Compatibility tests additionally exercise upstream V2-to-V3 migration and log
publication. This does not certify arbitrary user logs or custom DSH plugins.

Full Mac Docker Desktop end-to-end testing and a real DeepSeek model smoke are
not part of this deterministic acceptance and remain manual follow-ups. The
release evidence records those limits. Model credentials remain user-supplied.

本次将 DSH 精确基线升级到 0.1.5-rc.2，继续提供 Linux x86_64 和 Apple Silicon 安装包。
日常入口保持 demo up / open / down；会话格式升级至 V3，旧日志由上游迁移并保留原文件，
升级后不支持旧版本读取。请先保留数据备份；本地 demo 不提供跨版本原地升级，
跨版本 CellSnapshot 恢复仍被拒绝。确定性核心链路验证不代表真实模型或完整 Mac Docker
Desktop 链路已经验收。
