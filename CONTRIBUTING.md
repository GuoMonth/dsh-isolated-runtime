# Contributing

Read the [project constitution](CONSTITUTION.md). Focus on the Cell MVP and a neutral internal boundary. Pin each validation version; breaking API/configuration/state changes are allowed, with no historical compatibility, upgrade or seamless recovery promise. Fail fast with structured diagnostics and validate the core flow before adding infrastructure.

This project optimizes for a small executable contract. Changes should remove
ambiguity rather than add compatibility layers.

## Before opening a pull request

GitHub automatically runs only `Source standards`: syntax, Go formatting,
whitespace, LF/final-newline and workflow syntax checks. It has one Linux job,
a five-minute timeout and cancels superseded PR runs. Main pushes do not repeat
CI or build images. Run the same source check with `node hack/check-standards.mjs`.

Run behavioral tests locally according to the changed surface and record the
commit, commands, results and relevant artifact digests in the PR. Heavy GitHub
workflows remain manual diagnostic/release tools; do not dispatch them for
routine PR acceptance. Local success is sufficient for behavioral acceptance.

| Changed surface | Relevant local checks |
| --- | --- |
| Documentation | Links, referenced commands, `git diff --check`, Source standards |
| Go behavior | Affected tests; `make verify` for a broad change; lint/vuln when relevant |
| API / CRD | `make verify-cell`, generated-artifact drift checks |
| DSH / image / access seam | `make verify-dsh` (downloads exact upstream source) |
| Cluster behavior | Relevant `make verify-kind` / `verify-kind-phase2` / `verify-kind-phase3` / `verify-kind-phase4` |
| CLI / installation | Cell CLI checks below and a clean consumer install; `hack/verify-mvp.sh` applies only to the legacy standalone installer |
| Release inputs | `node hack/verify-release-contract.mjs` after committing inputs; exact-archive acceptance |

Use [Go development](docs/go-development.md) for toolchain pins and lifecycle diagnostics. Select checks for the changed behavior; documentation alone does not require cluster or release acceptance. Once checks pass, broaden or repeat for new changes or unresolved concerns.

Current Cell npm CLI checks, only when that surface changes:

```sh
npm test --prefix packages/cell-cli
node packages/cell-cli/bin/cli.mjs --verify-release
```

Legacy standalone checks apply only when modifying the retained legacy implementation; they are not Cell CLI acceptance:

```sh
npm ci --ignore-scripts --prefix packages/cli
npm test --prefix packages/cli
npm ci --ignore-scripts --prefix runtime-files
node --test test/local-runtime.test.cjs
shellcheck -x dsh-runtime demo demo-files/host.sh demo-files/tools.sh demo-files/forward.sh
```

## Design rules

- Namespace is the tenant boundary; do not add a second tenant identifier.
- Keep topology, routing, scheduling, and session state out of Cell.
- Use native Kubernetes, Gateway API, and CSI resources instead of shadow APIs.
- Keep all images and DSH behavior pinned by content/version.
- Do not parse DSH protocols in the launcher.
- State security assumptions explicitly and fail closed at trust boundaries.
- Generated API and CRD artifacts are committed and must have zero drift.

Current acceptance is recorded in the active Issue; historical milestone GO gates do not apply to every change.
Apache-2.0 contributions require the usual Developer Certificate of Origin
sign-off (`git commit -s`).
