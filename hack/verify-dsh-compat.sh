#!/usr/bin/env bash
set -euo pipefail
repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$repo_root"
# Official npm closure, verified by package-lock integrity against
# compat/dsh/baseline.json. No upstream source build and no settings-client patch.
npm ci --prefix images/environment --omit=dev --no-audit --no-fund
DSH_REAL_CLI="$repo_root/images/environment/node_modules/@deepseek-ai/dsh/lib/bin.js"
export DSH_REAL_CLI
go test ./compat/dsh ./cmd/environment-launcher ./internal/dshcompat/launcher -count=1
