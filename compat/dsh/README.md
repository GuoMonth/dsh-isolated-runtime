# DSH 0.2.1-rc.3 compatibility

[简体中文](README.zh-CN.md)

The workload image installs official DSH 0.2.1-rc.3 using the committed npm lockfile. [baseline.json](baseline.json) records the source identity, package integrity, transport and data paths.

DSH 0.2.1 persists authenticated-browser settings on the Host natively. The workload image does not patch the settings client; authentication, server routes and model protocols stay native.

```sh
bash hack/verify-dsh-compat.sh
```

This checks package installation and launcher transport. Browser/upstream Sandbox tests are in [test/rc2](../../test/rc2/README.md); current RC coverage is in [validation.md](https://github.com/GuoMonth/dsh-isolated-runtime/releases/download/v0.5.0-rc.1/validation.md). Data and lifecycle are defined by the [contract](../../docs/en/contract.md).
