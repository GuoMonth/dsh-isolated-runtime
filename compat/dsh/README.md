# DSH RC2 compatibility

[简体中文](README.zh-CN.md)

The workload image installs official DSH 0.2.0-rc.2 using the committed npm lockfile. [baseline.json](baseline.json) records the source identity, package integrity, transport and data paths.

At remote hostnames, RC2 disables the native Models settings page. `apply-patches.mjs` changes one line of the shipped ui-settings client module, verifying the full file SHA256 before and after. Authentication, server routes and model protocols stay native.

```sh
bash hack/verify-dsh-compat.sh
```

This checks package installation, patch hashes and launcher transport. Browser/upstream Sandbox tests are in [test/rc2](../../test/rc2/README.md); current RC coverage is in [validation.md](https://github.com/GuoMonth/dsh-isolated-runtime/releases/download/v0.4.0-rc.1/validation.md). Data and lifecycle are defined by the [contract](../../docs/en/contract.md).
