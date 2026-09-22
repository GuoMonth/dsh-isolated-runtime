#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."
export GOTOOLCHAIN=go1.27.1
template_tmp=$(mktemp)
trap 'rm -f "$template_tmp"' EXIT
go run ./internal/controller/cmd/generate-cell-template > "$template_tmp"
diff -u packages/cell-connector/src/templates/cell-mvp-v1.json "$template_tmp"
