#!/usr/bin/env bash
# verify.sh runs the local quality gates in the same order as CI.
set -euo pipefail

cd "$(dirname "$0")/.."

unformatted="$(gofmt -l .)"
if [[ -n "$unformatted" ]]; then
  echo "gofmt required on:" >&2
  echo "$unformatted" >&2
  exit 1
fi

go vet ./...
go test ./...
go build ./...
