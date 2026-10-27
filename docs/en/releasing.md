# Release DSH runtime

[简体中文](../zh-CN/releasing.md) · [Guides](index.md)

Runtime distributes a Linux/amd64 workload image and source release. The platform bundles the private Connector and owns installation. RC candidate: [0.5.0-rc.1](https://github.com/GuoMonth/dsh-isolated-runtime/releases/tag/v0.5.0-rc.1), prepared 2026-10-27 with platform 0.11.0-rc.1 and DSH 0.2.1-rc.3. Tested scope is in the [validation report](https://github.com/GuoMonth/dsh-isolated-runtime/releases/download/v0.5.0-rc.1/validation.md).

Runtime version is maintained in `VERSION`. RC/beta releases use `--prerelease --latest=false`. Record separate workload and Connector source identities; an unchanged pinned Connector can be retained after comparison and joint acceptance.

## Prepare

```sh
make verify
make verify-dsh
node hack/pack-environment-connector.mjs /absolute/output-directory
```

Pack from a clean committed source. The Connector includes source.json, SHA256 and npm integrity; its new tarball must be consumed and jointly tested by the platform. Runtime RBAC stays here; the platform packages the checksummed copy.

Run the manual `prepare-release.yml` on main. It builds the workload image and uploads candidate/source/digest metadata. Pull that exact image and run runtime/platform acceptance. [DSH compatibility](../../compat/dsh/README.md) describes the exact baseline and native settings behavior.

## Publish

After acceptance and publication authorization, create a lightweight `vVERSION` tag at the verified source commit. Attach the workload digest, source SHA, DSH identity, Connector provenance and validation report to its GitHub Release; link the paired platform release. Platform then publishes the verified npm archive and its matching Release.

Published versions and artifacts remain immutable. See the [platform repository](https://github.com/GuoMonth/dsh-multi-tenant) for the paired release procedure. The legacy runtime npm installer is deprecated; installation uses dsh-multi-tenant.
