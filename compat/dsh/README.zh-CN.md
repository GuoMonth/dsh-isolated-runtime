# DSH RC2 接入

精确源码身份/npm integrity 见 [baseline.json](baseline.json)。新镜像
`images/environment/Dockerfile` 按提交的 npm lock 安装官方 `0.2.0-rc.2`，
不再下载源码或运行 build:official/source deploy。

真实 Chromium 在远程 hostname 复现 Models 页“settings are unavailable in this browser”。
RC2 没有配置项可改变该 hostname 的持久模式，因此 `apply-patches.mjs` 仅替换发行包
ui-settings 客户端模块的一行，前后完整 SHA256 校验；不改认证、服务端接口或模型协议。
旧 `cell-settings.patch` 与源码重构建镜像配方已删除。

单 PVC 包含 workspace/home/dsh。launcher 在 workspace 启动；HOME=home、DSH_HOME=dsh，
原生凭据在 dsh/.credentials.yaml。用户 npm 工具写 HOME/.local，缓存写 /tmp。
平台密钥和集群 token 不进入环境；正常保留不等于备份或迁移。

`dev-run go=1.27 -- bash hack/verify-dsh-compat.sh` 验证官方包、补丁和 launcher。
浏览器与真实上游 Sandbox smoke 见 `test/rc2/README.md`；实际证据见
`docs/evidence/mvp-a-rc2-2026-09-29.md`，未做项不计通过。
