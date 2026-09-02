# Contributing

This project optimizes for a small executable contract. Changes should remove
ambiguity rather than add compatibility layers.

## Before opening a pull request

```bash
make verify
golangci-lint run
```

Run `make verify-cell` for API/CRD changes and `make verify-dsh` for changes
under `compat/dsh`, `internal/dshcompat`, or the Cell image/access seam. The full
DSH check downloads and tests the exact pinned upstream tree.

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
