# dsh-isolated-runtime

面向 [DeepSeek Harness（DSH）](https://github.com/deepseek-ai/deepseek-harness)
的 Kubernetes 原生隔离层。本项目只定义一个持久边界——`Cell`；基础设施能力继续由
Kubernetes、Gateway API 与 CSI 管理。

**当前状态：Phase 1 单 Cell 垂直切片。** Operator 将 Cell 翻译为 Kubernetes 原生的存储、
身份、工作负载、Service 与入口策略资源。Cell 镜像以 access launcher 为 PID 1，并在 Pod
替换后保持 DSH 数据。

[English](./README.md)

## Cell 契约

```yaml
apiVersion: dsh.isolated.io/v1alpha1
kind: Cell
metadata:
  name: assistant
  namespace: tenant-alice
spec:
  image: ghcr.io/example/dsh-cell@sha256:0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef
  storage:
    size: 20Gi
```

namespace 就是租户边界。镜像必须固定 digest；存储只能扩容，`storageClassName` 与
`retentionPolicy` 创建后不可变。
API 不暴露 session、Pod/Node 地址、`RuntimeClass`、revision、scheduler、checkpoint、
profile 或 hostname。完整内容见[示例](./config/samples/dsh_v1alpha1_cell.yaml)与
[生成的 CRD](./config/crd/bases/dsh.isolated.io_cells.yaml)。

## 验证入口

```bash
make verify             # 格式、生成漂移、vet、race 测试、构建
make test-envtest        # 使用真实 API server 验证 controller reconcile
make verify-cell        # 在临时 kind 集群验证 CRD 行为
make verify-images      # 生产镜像与真实 DSH 持久化 smoke test
make verify-kind        # 在 kind 完成 Phase 1 垂直实证
make verify-dsh         # 运行精确版本的上游 DSH 兼容套件
golangci-lint run
```

初始支持范围只有 `dsh-v0.1.2-alpha.4`，commit
`4e84901e6471b79ec0338099867ebb4606d12bb5`，不是 semver 范围。
[兼容性记录](./compat/dsh/README.zh-CN.md)解释了为什么 access seam 最终选择由
Cell-local launcher 持有 DSH 子进程。

使用 `kubectl apply -k config/default` 安装 CRD、cluster-wide Operator 与其
`dsh-system` namespace。公开路由与用户认证仍属于 Phase 2。

## 设计

- [架构](./docs/specs/architecture.zh-CN.md)
- [威胁模型](./docs/specs/threat-model.zh-CN.md)
- [Roadmap](./ROADMAP.zh-CN.md)
- [贡献指南](./CONTRIBUTING.zh-CN.md)

采用 Apache-2.0 许可证。已经删除的 pre-Cell API 和部署不承担兼容承诺。
