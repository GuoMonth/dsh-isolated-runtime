# 参与开发

阅读 [CONSTITUTION.md](CONSTITUTION.md) 与 [环境契约](docs/design/environment-contract.zh-CN.md)，以用户最新授权和 [MVP Issue #104](https://github.com/GuoMonth/dsh-multi-tenant/issues/104) 为验收依据。允许 Alpha 破坏性变更，不保留旧后端、兼容层或迁移路径。

使用 Go 1.27.1、Node 24 与已提交锁文件。先运行 `npm ci --prefix packages/environment-connector`，再运行 `dev-run go=1.27 -- make verify`；覆盖构建、Go race、Connector 测试、vet 与源码标准。`make verify-dsh` 校验精确上游源码接缝；真实集群检查见 [test/environment](test/environment/README.md)。fixture 不能替代真实运行证据。

CI 仅运行源码标准。PR 记录实际检查、制品身份和未覆盖项；本轮禁止发 npm、公网产品镜像或 Release。runtime 管资源身份与生命周期，平台管授权，上游 core 独占 Pod/Service 控制；不二次调谐、不从缺失推断成功停止、不自动补空卷。

提交使用 `git commit -s`。详见 [English](CONTRIBUTING.md) 与 [Go 工具链](docs/go-development.md)。
