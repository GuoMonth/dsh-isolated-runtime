# DSH RC2 兼容说明

[English](README.md)

运行镜像使用已提交的 npm 锁文件安装官方 DSH 0.2.0-rc.2。[baseline.json](baseline.json) 记录源码身份、包完整性、传输和数据路径。

RC2 在远程 hostname 下禁用原生 Models 配置页。`apply-patches.mjs` 替换发行包 ui-settings 客户端模块的一行，前后校验完整文件 SHA256。认证、服务端路由和模型协议保持原生行为。

```sh
bash hack/verify-dsh-compat.sh
```

此命令检查包安装、补丁哈希和 launcher 传输。浏览器及上游 Sandbox 测试见 [test/rc2（英文）](../../test/rc2/README.md)，实际发行覆盖见 [validation.md（英文）](https://github.com/GuoMonth/dsh-isolated-runtime/releases/download/v0.4.0-alpha.1/validation.md)。数据和生命周期见[契约](../../docs/zh-CN/contract.md)。
