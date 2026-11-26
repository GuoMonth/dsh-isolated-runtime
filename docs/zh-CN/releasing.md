# 发布 DSH runtime

[English](../en/releasing.md) · [文档目录](index.md)

runtime 分发 Linux/amd64 运行镜像和源码版本，并为推荐的 Kubernetes 参考部署提供可选 Connector 归档。参考平台镜像单独安装选定的归档。[0.5.0-rc.3](https://github.com/GuoMonth/dsh-isolated-runtime/releases/tag/v0.5.0-rc.3) 已于 2026-11-18 发布，配套平台 0.12.0-rc.2、DSH 0.2.2-rc.3。精确源码、镜像摘要、已接受安全风险和已测/未测范围见[验证报告](https://github.com/GuoMonth/dsh-isolated-runtime/releases/download/v0.5.0-rc.3/validation.md)。源码 `VERSION` 选择下一候选 0.5.0-rc.4，DSH 固定为 0.2.2-rc.5，已发布标签和制品保持原样。

runtime 版本在 `VERSION` 维护。RC/beta 发行使用 `--prerelease --latest=false`。运行镜像和 Connector 分别记录源码来源；实现未变时，可在比对与联合验收后沿用已有精确 Connector。

## 准备

```sh
make verify
make verify-dsh
node hack/pack-environment-connector.ts /absolute/output-directory
```

从已提交的 Connector 输入打包。归档内的 `source.json` 记录源码提交。同目录的 `connector.json` 记录仓库、提交、归档文件名、SHA256 和 npm SHA512 integrity。参考镜像选用归档前核对两个校验值。联合验收分别记录运行镜像摘要和 Connector 来源。参考安装器使用经过校验的 runtime RBAC 副本。

在 main 上运行手动 `prepare-release.yml`，构建运行镜像并上传候选、源码及 digest 信息。拉取精确镜像，执行 runtime/平台验收。精确 baseline 与原生设置行为见 [DSH 兼容说明（英文）](../../compat/dsh/README.md)。

## 发布

验收通过并获得发布授权后，在受测源码提交上建立轻量 `vVERSION` 标签。GitHub Release 附带运行镜像 digest、源码 SHA、DSH 身份、Connector 来源和验证报告，链接配套平台发行。平台随后发布受测 npm 归档及对应 Release。

已发布版本和制品保持原样。双方发布流程见[平台仓库](https://github.com/GuoMonth/dsh-multi-tenant/blob/main/README.zh-CN.md)。旧 runtime npm 安装器已弃用，安装使用 dsh-multi-tenant。

## 安全审计报告

候选版本准备流程保留原始审计报告和评估结果。Node 审计要求零发现。0.5.0-rc.3 的验证报告保留了当时的 Picomatch 例外记录。GitHub 已于 2026-11-18 撤回该公告。[GHSA-fc5q-39xc-vjpf](https://github.com/advisories/GHSA-fc5q-39xc-vjpf)
