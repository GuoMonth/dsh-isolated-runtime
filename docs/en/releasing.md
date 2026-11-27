# Release DSH runtime

[简体中文](../zh-CN/releasing.md) · [Guides](index.md)

Runtime distributes a Linux/amd64 workload image and source release, with an optional Connector archive for the recommended Kubernetes deployment. The reference platform image installs the selected archive separately. Source `VERSION` selects runtime candidate 0.5.0 with official DSH 0.2.2-rc.5, paired with platform 0.12.0. The previous [0.5.0-rc.4 testing report](https://github.com/GuoMonth/dsh-isolated-runtime/releases/download/v0.5.0-rc.4/TESTING.md) records the exact artifacts and observed outcomes for the RC.

Runtime version is maintained in `VERSION`. RC/beta releases use `--prerelease --latest=false`. Record separate workload and Connector source identities; an unchanged pinned Connector can be retained after comparison and joint acceptance.

## Prepare

```sh
make verify
make verify-dsh
node hack/pack-environment-connector.ts /absolute/output-directory
```

Pack from committed Connector inputs. The archive includes `source.json` with its source commit. The adjacent `connector.json` records the repository, commit, archive filename, SHA256 and npm SHA512 integrity. Verify both digests before selecting the archive for the reference image. Record the workload image digest and Connector provenance independently in joint acceptance. The reference installer uses a checksummed copy of the runtime RBAC.

Run the manual `prepare-release.yml` on main. It builds the workload image and uploads candidate/source/digest metadata. Pull that exact image and run runtime/platform acceptance. [DSH compatibility](../../compat/dsh/README.md) describes the exact baseline and native settings behavior.

## Publish

After acceptance and publication authorization, create a lightweight `vVERSION` tag at the verified source commit. Attach the workload digest, source SHA, DSH identity, Connector provenance and validation report to its GitHub Release; link the paired platform release. Platform then publishes the verified npm archive and its matching Release.

Published versions and artifacts remain immutable. See the [platform repository](https://github.com/GuoMonth/dsh-multi-tenant) for the paired release procedure. The legacy runtime npm installer is deprecated; installation uses dsh-multi-tenant.

## Security reports

Candidate preparation records raw audit reports and their assessment. Node audits require zero findings. The 0.5.0-rc.3 validation report documents its historical Picomatch exception. GitHub withdrew the advisory on 2026-11-18. [GHSA-fc5q-39xc-vjpf](https://github.com/advisories/GHSA-fc5q-39xc-vjpf)
