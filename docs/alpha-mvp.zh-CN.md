# OIDC + Cell 集成 MVP

2026-09-22：按用户确认的[项目宪法](../CONSTITUTION.md)收缩当前里程碑。已验收 Cell/Operator 镜像仍以 v0.3.0-alpha.1（Linux/amd64）公开。当前 `main` 还包含固定模板 `cell-mvp-v1`，并通过了 2026-09-22 内部候选回归（[报告](https://github.com/GuoMonth/dsh-multi-tenant/blob/main/docs/evidence/cell-mvp-2026-09-22.md)）。该源码候选尚未重新发行：公开 runtime npm `0.3.0-alpha.1` 及其镜像仍对应先前发行。部署新候选请使用[平台候选指南](https://github.com/GuoMonth/dsh-multi-tenant/blob/main/docs/reference/cell-mvp-v1-candidate.zh-CN.md)；下文已发行 npm 清单不适用于新模板。企业自托管是方向，不代表当前生产可用。

## 当前目标与分工

两个用户通过 multi-tenant 的 OIDC 登录、创建自己的 Cell、使用原生 DSH，跨用户访问被拒绝。上层负责用户协议、身份/成员、授权和会话；本仓库负责 Cell、资源生命周期与受限访问通道；DSH 负责应用协议、Session 和工具。

集群、namespace、CNI、存储、DNS/TLS 和服务身份权限由管理员配置。平台通过中立内部接口消费 Cell，不建立第二套 Pod/PVC 控制器。

## 最小部署与边界

- 固定版本回归基线记录了一个集群、上层单副本、OIDC 提供方和固定模板。验证该组合时使用一套明确的管理员配置。
- 当前集成链路使用选定的管理员配置 Kubernetes 部署。
- 固定当次源码/DSH/image 版本。API、配置和状态格式允许随时破坏性变更，不承诺历史兼容、升级、迁移或无感恢复。
- 配置、权限、模板、版本不符合时 fast fail；异步就绪有界等待；错误含阶段、目标、观测状态、写入结果、重试/检查建议及日志关联 ID，且不含秘密。
- 超时不代表取消，记录缺失不代表停止完成。用原标识查询未知结果，不换 key 自动重建，不建设永久终态、无限重试或自动修复系统。
- 已发行交付通过管理员已配置集群上的平台 npm CLI 使用；本仓库不增加集群安装器。`cell-mvp-v1` 候选在新包与镜像发行前仍是源码候选，部署说明见上方平台候选指南。
- 当前源码使用标准 Kubernetes Pod 作为边界：`securityClass` 仅接受 `standard`，controller 会拒绝不支持的既有值。已发布 npm `0.3.0-alpha.1` 包及其固定部署产物保持不变；本次源码修改不修订或重新发布它们。

## 当前闭环验收

记录两个仓库提交、镜像 digest、精确 DSH、CNI/存储/CPU 架构、命令与脱敏结果；只针对当前组合。

1. 两个 OIDC 身份与各自 Cell：创建、读取、原生 HTTP/WS/stream/Fetch 可用；未认证、跨用户及绕过入口被拒绝。
2. 父平台会话失效后派生环境会话同时失效，旧连接关闭、新连接拒绝；登出/上层重启不删除 Cell，不承诺停止 DSH 后台任务。
3. 重复创建在当前资源存在期间不重复分配；未知写入、版本不匹配、权限错误提供可由 AI 解读的诊断；旧 UID 删除不影响新实例。
4. 正常 Pod 重建后当前版本的文件/会话仍保留。此项不是历史数据升级、灾难恢复或节点分区 fencing 证明。
5. 一次真实模型调用、唯一文件写入/读回；凭据私下配置。确定性模型可回归但不代替该证据。
6. 管理员删除/测试环境重建说明写清 data、private-state 和外部 Secret 的实际处置范围。只操作明确授权的资源，不静默清空旧数据。

未证实清理完成时拒绝复用旧数据并交给管理员，不把控制对象消失等同物理 writer 停止。不额外扩展为通用恢复/迁移机制。

## 已发行制品边界

已发行制品只描述各自版本；本文档不会改动已发布包或现有环境。当前 npm 包及固定清单见[发行说明](distribution.md)；较早 standalone 行为保存在[历史归档](archive/standalone-alpha1/README.md)。

[English](alpha-mvp.md)

旧发行结果保留于 [2026-09-20 集成回归](https://github.com/GuoMonth/dsh-multi-tenant/blob/4ba252765bccb41314c0bdc6b11bcf60cc0b33ef/docs/evidence/cell-regression-2026-09-20.md)。当前源码候选结果与限制见 [2026-09-22 Cell MVP 报告](https://github.com/GuoMonth/dsh-multi-tenant/blob/main/docs/evidence/cell-mvp-2026-09-22.md)。已发行交付仍通过管理员预配置 K8s 上的平台 npm CLI 使用；历史 R1–R6 待测清单不代表现在仍未测试。

不支持的 securityClass 会拒绝就绪与集成准入，但不代表既有 workload 已停止或删除，也不代表历史 standalone 路由已撤销；管理员需检查并明确处置这些资源。不支持从历史 sandboxed 部署原地升级。
