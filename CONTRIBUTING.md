# Contributing

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

```bash
make verify
make lint
make vuln
```

Toolchain pins, lifecycle rules and leak diagnostics are documented in
[Go development](docs/go-development.md). Keep these checks local; this does not
expand automatic CI beyond Source standards.

Run `make verify-cell` for API/CRD changes and `make verify-dsh` for changes
under `compat/dsh`, `internal/dshcompat`, or the Cell image/access seam. The full
DSH check downloads and tests the exact pinned upstream tree.

For cluster changes, run the relevant `make verify-kind`, `make verify-kind-phase2`,
`make verify-kind-phase3` or `make verify-kind-phase4` locally. Installation and
release changes also need the focused checks in AGENTS.md and exact-archive
acceptance in `hack/verify-mvp.sh`. Do not relabel local evidence as CI evidence.

## Design rules

- Namespace is the tenant boundary; do not add a second tenant identifier.
- Keep topology, routing, scheduling, and session state out of Cell.
- Use native Kubernetes, Gateway API, and CSI resources instead of shadow APIs.
- Keep all images and DSH behavior pinned by content/version.
- Do not parse DSH protocols in the launcher.
- State security assumptions explicitly and fail closed at trust boundaries.
- Generated API and CRD artifacts are committed and must have zero drift.

Large milestones end with an issue-based review and a GO/CONDITIONAL GO record.
Apache-2.0 contributions require the usual Developer Certificate of Origin
sign-off (`git commit -s`).
