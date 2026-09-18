#!/usr/bin/env bash
# Install the Linux amd64 tools used by manually dispatched reference gates.
set -euo pipefail

if [[ "$(uname -s)/$(uname -m)" != Linux/x86_64 ]]; then
  echo "this CI tool installer requires Linux amd64" >&2
  exit 1
fi
destination="${DSH_TEST_TOOLS_DIR:-${HOME}/.local/bin}"
mkdir -p "$destination"
GOBIN="$destination" go install sigs.k8s.io/kind@v0.33.0
temporary="$(mktemp -d)"
trap 'rm -rf "$temporary"' EXIT
curl -fsSL --retry 3 -o "$temporary/kubectl" \
  https://dl.k8s.io/release/v1.37.0/bin/linux/amd64/kubectl
printf '%s  %s\n' \
  6129359f4e1f3848a5572ccb0b26cf28b8ca08cef38c95a765b2f64a2c961a2f \
  "$temporary/kubectl" | sha256sum --check
install -m 0755 "$temporary/kubectl" "$destination/kubectl"
if [[ -n "${GITHUB_PATH:-}" ]]; then
  printf '%s\n' "$destination" >>"$GITHUB_PATH"
fi
