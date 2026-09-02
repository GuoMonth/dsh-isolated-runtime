#!/usr/bin/env bash
# Regenerate committed API artifacts and fail if generation is not reproducible.
set -euo pipefail

repo_root="$(cd "$(dirname "$0")/.." && pwd)"
cd "$repo_root"

generated=(
  api/v1alpha1/zz_generated.deepcopy.go
  config/crd/bases/dsh.isolated.io_cells.yaml
)

before="$(sha256sum "${generated[@]}")"
go generate ./...
after="$(sha256sum "${generated[@]}")"

if [[ "$before" != "$after" ]]; then
  echo "generated Cell API artifacts are stale; run 'make generate'" >&2
  diff <(printf '%s\n' "$before") <(printf '%s\n' "$after") >&2 || true
  exit 1
fi
