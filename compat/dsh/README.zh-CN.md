# DSH 0.2.1-rc.3 兼容说明

[English](README.md)

运行镜像使用已提交的 npm 锁文件安装官方 DSH 0.2.1-rc.3。[baseline.json](baseline.json) 记录源码身份、包完整性、传输和数据路径。

DSH 0.2.1 原生支持认证浏览器在 Host 上持久化设置。运行镜像不再给 settings 客户端打补丁；认证、服务端路由和模型协议保持原生行为。

```sh
bash hack/verify-dsh-compat.sh
```

此命令检查包安装和 launcher 传输。浏览器及上游 Sandbox 测试见 [test/rc2（英文）](../../test/rc2/README.md)，当前 RC 覆盖见 [validation.md（英文）](https://github.com/GuoMonth/dsh-isolated-runtime/releases/download/v0.5.0-rc.1/validation.md)。数据和生命周期见[契约](../../docs/zh-CN/contract.md)。
