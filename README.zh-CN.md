# dsh-isolated-runtime

面向 [DeepSeek Harness（DSH）](https://github.com/deepseek-ai/deepseek-harness)
的 Kubernetes 原生隔离层。本项目只定义一个持久边界——`Cell`；基础设施能力继续由
Kubernetes、Gateway API 与 CSI 管理。

**当前状态：Phase 4 Fleet 运维已完成。** 同一套窄 `Cell` / `CellSnapshot` API 现在可以在
多个 namespace 中服从 Kubernetes 原生 quota 与 admission policy 并自动收敛。Controller
采用有界 worker、对象 watch、deadline 唤醒与 Kubernetes 错误退避；可选指标只暴露聚合的
controller 和封闭枚举授权结果。本项目仍不维护 fleet inventory、scheduler、namespace
策略引擎或备份服务。

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

namespace 就是租户边界。镜像必须固定 digest；存储只能扩容，`storageClassName`、
`retentionPolicy` 与 `restoreFrom` 创建后不可变。
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
make verify-kind-phase2 # 用 Envoy、Dex、Chromium 实证 HTTPS/OIDC/RBAC
make verify-kind-phase3 # 实证 writer-stop、CSI restore、rollout 与 fresh rollback
make verify-kind-phase4 # 实证 10 namespace / 50 Cell 的 quota、压力与恢复
make verify-dsh         # 运行精确版本的上游 DSH 兼容套件
golangci-lint run
```

当前支持范围只有 `dsh-v0.1.2-rc.1`，commit
`a66e4702047846cdaa10c66c9d3df3951f5ea70d`，不是 semver 范围。
[兼容性记录](./compat/dsh/README.zh-CN.md)解释了为什么 access seam 最终选择由
Cell-local launcher 持有 DSH 子进程。

`kubectl apply -k config/default` 安装不依赖 Gateway API 的 Phase 1 surface。安装 Envoy
Gateway，并提供 wildcard DNS/TLS 与 OIDC provider 配置后，`config/phase2` 增加参考
Gateway、authorizer 和路由模式。管理员使用普通 RoleBinding 授权；Operator 有意不管理它。
Envoy Gateway 安装必须采用同目录 `envoy-gateway.yaml` 配置，使数据面运行在 Gateway
namespace 并启用 Backend extension。

集群安装 CSI snapshot controller 与兼容 driver 后，`kubectl apply -k config/phase3` 启用
`CellSnapshot`；本项目不安装生产 CSI 组件。可执行示例见
[snapshot/restore sample](./config/samples/dsh_v1alpha1_cellsnapshot.yaml)。

`config/phase4` 是可选运维参考 overlay：它开启有界 controller 并发与私有 metrics listener，
但不创建 metrics Service 或 scraper。Namespace label、ResourceQuota、LimitRange、
StorageClass、RuntimeClass、Gateway 路由资格与 CSI 能力仍由集群管理员持有；详见
[namespace 契约](./docs/specs/namespace-contract.zh-CN.md)与
[指标契约](./docs/specs/metrics.zh-CN.md)。

发布候选 Cell/Operator 镜像只构建一次；全部门禁按 immutable digest 消费同一产物，成功后才把
原 digest 晋级为 GHCR `main`/`sha-*` 标签。晋级不重建，SBOM 与 provenance 仍绑定同一 manifest。

## 设计

- [架构](./docs/specs/architecture.zh-CN.md)
- [威胁模型](./docs/specs/threat-model.zh-CN.md)
- [Roadmap](./ROADMAP.zh-CN.md)
- [贡献指南](./CONTRIBUTING.zh-CN.md)

采用 Apache-2.0 许可证。已经删除的 pre-Cell API 和部署不承担兼容承诺。
