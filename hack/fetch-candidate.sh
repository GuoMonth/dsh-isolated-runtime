#!/usr/bin/env bash
# Validate the workflow identity before trusting a downloaded release artifact.
set -Eeuo pipefail
run_id="${1:?candidate run id}"; output="${2:?output directory}"
[[ "$run_id" =~ ^[0-9]+$ ]]
gh api "repos/GuoMonth/dsh-isolated-runtime/actions/runs/$run_id" | jq -e '
  .path == ".github/workflows/release.yml" and .head_branch == "main" and
  .head_repository.full_name == "GuoMonth/dsh-isolated-runtime" and
  .conclusion == "success"' >/dev/null
mkdir -p "$output"
gh run download "$run_id" --repo GuoMonth/dsh-isolated-runtime --name mvp-candidate --dir "$output"
test "$(jq -r .candidateRun "$output/release.json")" = "$run_id"
test "$(jq -r .sourceSHA "$output/release.json")" = "$(gh api "repos/GuoMonth/dsh-isolated-runtime/actions/runs/$run_id" --jq .head_sha)"
