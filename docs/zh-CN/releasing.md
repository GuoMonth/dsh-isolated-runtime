# 发布 DSH runtime

[English](../en/releasing.md) · [文档目录](index.md)

runtime 分发 Linux/amd64 运行镜像和源码版本，平台打包内部 Connector 并负责安装。[0.5.0-rc.3](https://github.com/GuoMonth/dsh-isolated-runtime/releases/tag/v0.5.0-rc.3) 已于 2026-11-18 发布，配套平台 0.12.0-rc.2、DSH 0.2.2-rc.3。精确源码、镜像摘要、已接受安全风险和已测/未测范围见[验证报告](https://github.com/GuoMonth/dsh-isolated-runtime/releases/download/v0.5.0-rc.3/validation.md)。源码 `VERSION` 与该版本一致，已发布标签和制品保持原样。

runtime 版本在 `VERSION` 维护。RC/beta 发行使用 `--prerelease --latest=false`。运行镜像和 Connector 分别记录源码来源；实现未变时，可在比对与联合验收后沿用已有精确 Connector。

## 准备

```sh
make verify
make verify-dsh
node hack/pack-environment-connector.mjs /absolute/output-directory
```

从已提交的干净源码打包。Connector 包含 source.json、SHA256 和 npm integrity，新 tarball 由平台消费并完成联合验证。runtime RBAC 在本仓库维护，平台打包经过校验的副本。

在 main 上运行手动 `prepare-release.yml`，构建运行镜像并上传候选、源码及 digest 信息。拉取精确镜像，执行 runtime/平台验收。精确 baseline 与原生设置行为见 [DSH 兼容说明（英文）](../../compat/dsh/README.md)。

## 发布

验收通过并获得发布授权后，在受测源码提交上建立轻量 `vVERSION` 标签。GitHub Release 附带运行镜像 digest、源码 SHA、DSH 身份、Connector 来源和验证报告，链接配套平台发行。平台随后发布受测 npm 归档及对应 Release。

已发布版本和制品保持原样。双方发布流程见[平台仓库](https://github.com/GuoMonth/dsh-multi-tenant/blob/main/README.zh-CN.md)。旧 runtime npm 安装器已弃用，安装使用 dsh-multi-tenant。

## 已接受的已知安全问题

2026-11-18，发行负责人授权将 picomatch 4.0.7 的 GHSA-fc5q-39xc-vjpf 作为本次发行的已知问题，重复 extglob 导致的 ReDoS 风险没有修复。接受期限截至 2026-12-02；其他公告、依赖版本变化、不完整报告及扫描失败仍阻断。默认本地审计保持严格，CI 显式携带该项授权并强制检查期限；仅显式接受的准备任务（`accept_picomatch=true`）启用该项例外，并在候选制品中保留原始审计报告及非零风险评估。最终验证报告和发行说明必须披露。
