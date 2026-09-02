# 贡献指南

本项目追求小而可执行的契约。变更应减少歧义，不应增加兼容层。

## 提交 PR 前

```bash
make verify
golangci-lint run
```

API/CRD 变更运行 `make verify-cell`；`compat/dsh`、`internal/dshcompat` 或 Cell
镜像/access seam 变更运行 `make verify-dsh`。完整 DSH 门会下载并测试精确固定的上游源码树。

## 设计规则

- namespace 是 tenant boundary，不增加第二个 tenant identifier；
- Cell 不包含 topology、route、scheduler 或 session 状态；
- 复用 Kubernetes、Gateway API 与 CSI 原生资源，不建立影子 API；
- 镜像与 DSH 行为必须按 content/version 固定；
- launcher 不解析 DSH 协议；
- 显式陈述安全假设，信任边界 fail closed；
- 生成的 API/CRD 产物必须提交且保持零漂移。

大里程碑以 issue 复盘，并记录 GO / CONDITIONAL GO。Apache-2.0 贡献需按 DCO 使用
`git commit -s` 签名。
