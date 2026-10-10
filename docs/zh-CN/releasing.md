# 发布 DSH runtime

[English](../en/releasing.md) · [文档目录](index.md)

runtime 分发 Linux/amd64 运行镜像和源码版本，平台打包内部 Connector 并负责安装。RC 候选：[0.4.0-rc.1](https://github.com/GuoMonth/dsh-isolated-runtime/releases/tag/v0.4.0-rc.1)，配套平台 0.10.0-rc.1、DSH 0.2.0-rc.2。

runtime 版本在 `VERSION` 维护。RC/beta 发行使用 `--prerelease --latest=false`。运行镜像和 Connector 分别记录源码来源；实现未变时，可在比对与联合验收后沿用已有精确 Connector。

## 准备

```sh
make verify
make verify-dsh
node hack/pack-environment-connector.mjs /absolute/output-directory
```

从已提交的干净源码打包。Connector 包含 source.json、SHA256 和 npm integrity，新 tarball 由平台消费并完成联合验证。runtime RBAC 在本仓库维护，平台打包经过校验的副本。

在 main 上运行手动 `prepare-release.yml`，构建运行镜像并上传候选、源码及 digest 信息。拉取精确镜像，执行 runtime/平台验收。settings 补丁及校验见 [DSH 兼容说明（英文）](../../compat/dsh/README.md)。

## 发布

验收通过并获得发布授权后，在受测源码提交上建立轻量 `vVERSION` 标签。GitHub Release 附带运行镜像 digest、源码 SHA、DSH 身份、Connector 来源和验证报告，链接配套平台发行。平台随后发布受测 npm 归档及对应 Release。

已发布版本和制品保持原样。双方发布流程见[平台仓库](https://github.com/GuoMonth/dsh-multi-tenant/blob/main/README.zh-CN.md)。旧 runtime npm 安装器已弃用，安装使用 dsh-multi-tenant。
