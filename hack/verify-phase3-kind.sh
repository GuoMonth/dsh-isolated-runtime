#!/usr/bin/env bash
# Extend the exact Phase 2 browser proof with the CSI-backed Phase 3 lifecycle.
set -Eeuo pipefail

repo_root="$(cd "$(dirname "$0")/.." && pwd)"
export DSH_E2E_EXTENSION="$repo_root/test/e2e/phase3/lifecycle.sh"
exec "$repo_root/hack/verify-phase2-kind.sh"
