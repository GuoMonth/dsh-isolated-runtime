# Distribution

Runtime distributes a Linux/amd64 workload image and source release. The internal Connector stays private and is bundled into the [platform npm package](https://github.com/GuoMonth/dsh-multi-tenant); administrators install with the [platform AI guide](https://github.com/GuoMonth/dsh-multi-tenant/blob/main/packages/multi-tenant/AI.md).

## Prepare and validate

Run `make verify` and `make verify-dsh` with the pinned tools. `node hack/pack-environment-connector.mjs /absolute/output-directory` packs a clean committed Connector with source.json, SHA256 and npm integrity. Changes to this artifact must be consumed and jointly tested by the platform before release.

The manual `prepare-release.yml` workflow builds the workload image from its exact main commit, publishes a candidate image and uploads its source/digest metadata. Pull that exact image locally and run the real runtime/platform acceptance before creating a public release. Candidate image publication is not acceptance.

The workload Dockerfile pins DSH0.2.0-rc.2 and its npm closure. The minimal remote-settings patch is checksummed in [compat/dsh](../compat/dsh/README.md). Runtime RBAC and upstream core pins are consumed by the platform installer.

## Publish

After local acceptance and user authorization, create a lightweight `vVERSION` tag at the verified source commit. Create the matching GitHub Release with the workload digest, source SHA, DSH identity, Connector provenance and a link to the paired platform release. Do not publish the internal Connector to npm. The platform publishes its npm package and matching GitHub Release together, using the same verified artifacts.

Keep published artifacts immutable. Retain the local joint evidence and list actual limitations; normal PVC retention does not prove backup, storage hard quotas or disaster recovery.

## Retired npm entry

The old `dsh-isolated-runtime` npm installer is retired. New installations use `dsh-multi-tenant`; runtime source and workload images continue through GitHub Releases. The manual `retire-npm.yml` workflow marks both legacy npm versions deprecated using the existing npm credential, then checks public registry metadata. It preserves versions, tarballs and dist-tags and does not publish a replacement package or change runtime release tags.
