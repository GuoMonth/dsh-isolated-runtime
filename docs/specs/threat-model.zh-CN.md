# Cell 威胁模型

## 范围

集成请求路径使用 multi-tenant 平台处理 OIDC 与用户授权。Gateway 负责 TLS 终止和路由，不代替平台执行 OIDC 或 Cell 授权。Connector 接收平台已授权的内部请求，并在代理前校验目标 Cell/Pod identity。

namespace 与 Cell UID 定义租户实例边界。NetworkPolicy 将 Cell proxy port 的访问限制在平台路径内，但实际生效依赖 CNI。DSH 在 Pod 内仍仅监听 loopback。租户数据与私有运行时状态分离；Provider 凭据不得进入租户数据、status 或日志。

这是面向普通 Kubernetes Pod 的应用隔离边界，不抵御已失陷的 Node/kernel、集群管理员或受信任的 Kubernetes、CNI、CSI、Gateway、云控制面。DSH 及启用插件位于 Cell 信任边界内。

基础边界继续保留：非root、禁止提权、drop capabilities、RuntimeDefault seccomp、不自动挂载ServiceAccount token、不提供宿主目录或Docker socket。资源请求/限额由Cell模板与集群策略提供，不假设任意Pod默认有配额。当前自动策略主要限制Cell ingress；控制面/私网的非必要egress由管理员参考部署配置并实测，不能由Pod存在本身推断已隔离。

## 威胁与控制

| 威胁 | 控制 / 限制 |
| --- | --- |
| 跨租户目标混淆 | 从平台已授权的 Cell 引用解析目标，并在转发前校验实时 namespace、Cell UID 和其拥有的 Pod identity。 |
| Pod 过期或重建 | 对比当前 owner/UID 链；目标实例不匹配时 fail closed。 |
| 网络直连绕过 | DSH 保持 loopback 监听，只允许配置的平台路径进入 proxy port。策略生效要求 CNI 确实执行 NetworkPolicy。 |
| 凭据泄漏 | launch token 只留在进程内存，诊断脱敏；Provider Secret 在租户数据与 status 之外提供。 |
| 并发或过期数据访问 | 由 Kubernetes 卷归属和当前 Cell 生命周期定义访问。历史 snapshot/restore 机制不是当前 MVP 验收承诺。 |
| 工作负载失陷 | 将 DSH 及插件视为该 Cell 内受信任代码；普通 Pod 隔离不能遏制宿主机或 kernel 失陷。 |

## 历史实现与实证

较早 standalone 发行使用 Envoy OAuth、`cell-authorizer`、SubjectAccessReview 及 snapshot/restore。这些不是当前集成平台请求路径。部分内容仍作为历史源码或已发布制品行为保留；本文档调整并未删除它们。版本历史见[归档的 standalone 文档](../archive/standalone-alpha1/README.md)。

固定版本集成实证及其限制记录于[共享回归报告](https://github.com/GuoMonth/dsh-multi-tenant/blob/4ba252765bccb41314c0bdc6b11bcf60cc0b33ef/docs/evidence/cell-regression-2026-09-20.md)。当前源码使用标准 Pod 边界并拒绝不支持的 `securityClass` 值；这不改变已发布 npm `0.3.0-alpha.1` 包及其固定制品。
