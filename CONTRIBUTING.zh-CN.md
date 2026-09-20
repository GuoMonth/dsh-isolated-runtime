# 贡献指南

产品边界见[宪法入口](CONSTITUTION.md)。[英文贡献指南](CONTRIBUTING.md) 维护检查命令与资源设计约束，避免维护两套不一致门禁。

按改动选择验证：文档检查链接与 Source standards；Go 修改验证相关行为；API/CRD 检查生成漂移；镜像/访问和集群行为按对应原生验收。安装与发行仅在修改对应流程时验证，不阻塞普通文档或 Cell MVP 开发。

自动 CI 仅 Source standards，其他检查本地按需运行。验收记录在当前 Issue，不沿用旧里程碑的通用 GO 门禁。贡献需 DCO（`git commit -s`）；PR 写明实际命令、结果与未覆盖项。
