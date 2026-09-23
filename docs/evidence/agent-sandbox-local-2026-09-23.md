# agent-sandbox 本地接入验证：GO 进入 W1

2026-09-23，主记录 [#100](https://github.com/GuoMonth/dsh-isolated-runtime/issues/100)，产品主线 [平台 #104](https://github.com/GuoMonth/dsh-multi-tenant/issues/104)。[机器可读结果](agent-sandbox-local-2026-09-23.json)，[可复查的测试脚本与步骤](../../test/agent-sandbox/README.md)。

**结论：选择上游 agent-sandbox core，进入 W1 正式替换。** 已在真实本地 Kubernetes 上证明普通 Pod、外部两块 PVC、现有 DSH launcher/HTTP/WS transport 能通过薄适配配合工作；没有新增同义 CRD、第二生命周期控制器或上游 fork。本次交付是接入试验及选型证据，production 仍使用 Cell；不是平台新版本已完成或可以发布的声明。

## 固定组合

- upstream `v1.0.3` / commit `527d9346fe1d237dea5c003f3c720531c7bab1df`；release core manifest 原始 SHA256 `725fafdabe6aac202a89dc57f1cfe0e2e92f3164c8c2bd343fffca52f7039d96`，仅固定镜像并显式关闭 extensions。
- 公开 controller 多架构 index `sha256:8c8f5814c16bd68631af0496a5fa4eb9bedce4d032de88b956130a041d9f438e`；实跑 Linux/amd64 manifest `registry.k8s.io/agent-sandbox/agent-sandbox-controller@sha256:b2160ee08dd4f2285b382d4b5073948891adfacbc9808a4ba9cf247766243c8c`。
- DSH 镜像复用已验收本地候选 `localhost:30482/dsh-cell@sha256:c9462f224229cb54295d5dfee8bb7f44a26285e1efda341800b7831592f6941e`，DSH `0.1.5-rc.2` / `fb2c4b9e698e30edb738bca4cf0618587db7d203`。这是本机 registry，不是新的公开发行。
- runtime/原有 transport 基线 `031eb7540ae913671a84a3a65e9e67048986f9f0`；Go probe 用项目 Go 1.27.1 构建，Node 24；未修改 production Connector/launcher。
- 单节点 kind `dsh-issue82`，Kubernetes `v1.37.0`、Calico `v3.32.2`、containerd `2.3.4`，`rancher.io/local-path`，RWO、WaitForFirstConsumer、PV reclaimPolicy Delete。不推广为其他 CSI 或跨节点性能/恢复证据。

Core 控制器来自公开发布镜像。kind 默认导入多架构镜像时遇到缓存缺少 arm64 层，改为导入已验证的 amd64 manifest 并固定该 digest；未重编译上游。集群原有 Cell/PVC/平台未改动；新资源限于 controller 安装及 `sandbox-spike` 测试命名空间。

## 真实链路结果

| 验证 | 实际结果 |
| --- | --- |
| 初始分配 | Sandbox 从 Suspended 创建，无工作负载；外部 data/private PVC UID 绑定完成，固定安全模板/SA/NetworkPolicy 就绪后 Running |
| DSH 原生访问 | 现有 Node transport → Go launcher → DSH；原生 cookie、settings/describe、session/create/list、WebSocket follow、session export 通过 |
| 正常停止 | 先撤销访问并关闭已建立 WS，再 Suspended；捕获同 Pod UID 的全部容器 terminated / exit 0 与 Succeeded，确认当前 generation、无 Pod/ready endpoint，原卷 UID 保持 |
| 再启动 | 两次热启动更换 Pod UID，Sandbox/PVC UID 不变；旧 cookie/session 继续使用，data/private 两卷 marker 可读 |
| 无调用方写入时重建 | 正常删除 Pod 后由上游重建；新 Pod、原 Sandbox/两卷、DSH Session 恢复 |
| 缺卷 | Pod Pending，未自动新建 PVC；未 Ready，访问拒绝 |
| 外来 Pod | 同名无 owner 且无认领标签的 Pod 未被认领/修改归属，Sandbox 未 Ready |
| UID 错误 | 实际同名 PVC 删除重建后，核验器拒绝新 UID；旧 UID 的 DeleteOptions 请求被 Kubernetes 拒绝，新 PVC 保留 |
| 并发与未知结果 | 同 resourceVersion 两次修改一成一拒；停止后的旧 revision 启动被拒；已删环境 patch Running 失败；成功 create 响应被丢弃后查原 key，重复 create 为 AlreadyExists |
| 删除与中断 | 确认停止后按 UID 删除 Sandbox，真实 GC 清理 Service；故意中断后两外部卷仍在，再依据原绑定删除 private；data 保持原 UID，独立只读 Pod 读回文件 |
| 网络 | 实际 Calico 阻断 DSH Pod 访问 K8s API，阻断无授权标签的 probe 入站；授权 probe 的原生访问正常 |

两卷 marker 使用 `kubectl exec` 写入/读取，验证的是实际存储，不冒充 DSH 模型工具调用。原生 DSH HTTP/WS 与 Session 持久性另有真实协议证据。没有本轮真实模型调用、MCP/CLI OAuth 或两用户 OIDC 全链路回归。

测试 probe 的 SA 只读本命名空间必要资源，用户 DSH Pod 不挂集群 token。每次应用请求由 test-only 核验器读取 Sandbox、真实 Pod/模板、Service、EndpointSlice、两 PVC 后交给原有 transport。基于真实快照的 15 个拒绝测试通过，覆盖过期 generation、错误 UID/owner、额外容器/envFrom、hostNetwork、镜像策略漂移、未绑定 PVC、错误/额外 endpoint 等。审查发现缺少 PVC Bound 判定，已加上 `status.phase=Bound` 和非空 volumeName，并用实际快照及负例复核。

## 停止证据与故障边界

首次监听脚本误用了 `kubectl get` 不支持的 `--resource-version` flag，未收集事件；脚本失败，环境保持 Suspended，未先恢复。修为原生 watch API，从此前记录的 resourceVersion 查询到同一 Pod 的 Succeeded/全部容器 exit 0 后才重新启动。后续两次实时 watch 均捕获正面终止证据。

额外测试以无权限身份监听，真实 API 返回 Forbidden：结果记为 StopUnverified，环境保持 Suspended，不自动恢复或删除卷；随后取得原 Pod UID 终止证据才清理。`require_terminal` 对实际终止事件通过，对空事件、错误 UID、缺容器状态均拒绝。**这是测试适配中的行为；production 的停止证据持久化、重启后准入及跨 revision 的相反意图竞争仍须 W2 实现。** Watch 410/历史压缩未专门故障注入；取不到证据必须失败，不能用无限重试或 Pod 404 替代。

同 revision CAS/删除实例拒绝证明 Kubernetes 原语可用，不证明生产平台所有并发已安全。未知 create 是丢弃成功响应的有限重查实验，不是传输层断网/超时注入。无调用方写入的重建证明正常健康节点路径，不是节点失联或强删下无双 writer。

PVC 名称挂载仍不能防御受信管理员在核验后更换同名 PVC/PV 内容；上游也不会在平台离线时读取我们的 UID 绑定。MVP 不开放租户 Kubernetes 写权限，不承诺该特权带外场景，不为它加 admission/fencing 系统。保留 data PVC 是保留当前字节，不是备份或恢复承诺。

## 性能样本与维护成本

- 首个 DSH 环境 Ready：10.827 秒；另一个独立故障测试环境为 9.523 秒。镜像已在节点缓存，不是冷镜像下载时间。
- 原环境两次热启动：6.669 / 6.638 秒；两次完整正常停止：1.886 / 1.466 秒。
- 无调用方时普通 Pod 重建至 Ready：7.871 秒。
- Probe 的每个 HTTP/WS 建立请求单轮 6 次 K8s 读。未计入未来平台绑定/授权额外查询，不作为生产 API 预算或优化收益承诺。

以上是单节点少量样本，不报告 P95、SLO、并发容量或跨 CSI 性能。启动、GC、调度仍交上游/Kubernetes；我们只保留一次操作的卷分配/删除、身份核验、访问撤销和必要停止证据。没有常驻新 reconciler。

## 审查与 W1/W2/W3 边界

Luna 只读源码/证据复核支持进入 W1，指出并修正 PVC Bound 校验；Claude Code 做了工具禁用的结果摘要反方审查，实际模型 `deepseek-flash[1m]`，不是 Claude 模型，也不是独立源码验证。采纳其生产启用前必须验证“证据缺失拒绝恢复”、业务并发和删除屏障的要求；不采纳其“首轮空证据假通过”的表述，因为首轮脚本实际失败且没有先行恢复。其“保留卷可回滚”措辞也未被采用，本轮没有验证回滚。

- **W1 [#97](https://github.com/GuoMonth/dsh-isolated-runtime/issues/97) / [平台 #105](https://github.com/GuoMonth/dsh-multi-tenant/issues/105)**：正式将产品 AgentEnvironment 映射上游 Sandbox；删除自有 Cell CRD/Operator/StatefulSet 控制路径及已裁决旧后端，落地模板、外部卷归属与 Node Connector；固定上游 release/digest。
- **W2 [#98](https://github.com/GuoMonth/dsh-isolated-runtime/issues/98)**：产品正常启停、StopUnverified 与重启后拒绝准入、未知结果/删除屏障、相反并发意图与连接撤销；健康节点范围，不能把这次脚本当生产实现。
- **W3 [平台 #106](https://github.com/GuoMonth/dsh-multi-tenant/issues/106)**：真实授权/HOME、两用户原生应用、性能基线与联合发行。上游试验通过不关闭这些实现任务。

本次不发布包或镜像，不迁移旧数据，不引入热池、自动 idle、快照、其他后端或历史兼容层。
