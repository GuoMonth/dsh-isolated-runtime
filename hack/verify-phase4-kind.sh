#!/usr/bin/env bash
# Extend the exact Phase 3 lifecycle proof with the bounded fleet proof.
set -Eeuo pipefail

repo_root="$(cd "$(dirname "$0")/.." && pwd)"
export DSH_PHASE3_EXTENSION="$repo_root/test/e2e/phase4/fleet.sh"
exec "$repo_root/hack/verify-phase3-kind.sh"
