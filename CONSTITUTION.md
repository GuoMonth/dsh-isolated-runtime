# 共享产品原则

本仓库与 dsh-multi-tenant 共用 [DSH 平台与运行时产品原则](https://github.com/GuoMonth/dsh-multi-tenant/blob/main/CONSTITUTION.md)，不另维护副本。

开源 Alpha、单 PVC、破坏性变更和首版/后续边界见[企业持久 AI 工作环境定位](https://github.com/GuoMonth/dsh-multi-tenant/blob/main/docs/design/enterprise-positioning.zh-CN.md)。运行时复用成熟基础设施，数据与归属可靠优先于极限启动；这些是目标要求，不代表当前Alpha已具备企业交付能力。

接口与阶段方案见 [AgentEnvironment 设计](https://github.com/GuoMonth/dsh-multi-tenant/blob/main/docs/design/agent-environment.zh-CN.md)，实施状态见其关联 Issue。运行时技术约束和验证入口见 [AGENTS.md](AGENTS.md) 与 [CONTRIBUTING.md](CONTRIBUTING.md)。

跨仓库任务若同时修改原则，应读取配套 multi-tenant 工作区的版本并记录双方提交；main 链接不代表未合并变更。

本轮固定 DSH 0.2.0-rc.2，不维护旧接口、历史升级或迁移。先冻结契约，再并行实现，最后联合验收；详见 [MVP 执行计划](https://github.com/GuoMonth/dsh-multi-tenant/blob/main/docs/plans/mvp-rc2.zh-CN.md)。
