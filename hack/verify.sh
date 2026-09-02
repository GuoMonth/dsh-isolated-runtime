#!/usr/bin/env bash
# verify.sh runs the local quality gates in the same order as CI.
set -euo pipefail

cd "$(dirname "$0")/.."

make verify
