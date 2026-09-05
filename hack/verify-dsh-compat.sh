#!/usr/bin/env bash
# Verify the exact upstream DSH release that defines the initial Cell contract.
set -euo pipefail

repo_root="$(cd "$(dirname "$0")/.." && pwd)"
baseline="$repo_root/compat/dsh/baseline.json"
source_repo="$(jq -r '.source.repository' "$baseline")"
source_tag="$(jq -r '.source.tag' "$baseline")"
source_commit="$(jq -r '.source.commit' "$baseline")"
source_version="$(jq -r '.source.version' "$baseline")"
package_manager="$(jq -r '.toolchain.packageManager' "$baseline")"
lock_sha="$(jq -r '.toolchain.lockfileSHA256' "$baseline")"
compat_root="${DSH_COMPAT_CACHE:-/tmp/dsh-isolated-runtime-compat}"
checkout="$compat_root/deepseek-harness"

mkdir -p "$compat_root"
if [[ ! -d "$checkout/.git" ]]; then
  git clone --filter=blob:none --no-checkout "$source_repo" "$checkout"
fi
git -C "$checkout" remote set-url origin "$source_repo"
git -C "$checkout" fetch --force --depth=1 origin "refs/tags/$source_tag:refs/tags/$source_tag"
git -C "$checkout" checkout --force --detach "$source_commit"

test "$(git -C "$checkout" rev-parse HEAD)" = "$source_commit"
test "$(git -C "$checkout" describe --tags --exact-match HEAD)" = "$source_tag"
test "$(jq -r '.version' "$checkout/package.json")" = "$source_version"
test "$(jq -r '.packageManager' "$checkout/package.json")" = "$package_manager"
test "$(sha256sum "$checkout/pnpm-lock.yaml" | awk '{print $1}')" = "$lock_sha"

# This exact release maps SIGTERM to exit code zero and uses the same forced
# exit code after successful disposal, disposal rejection, or timeout. The
# source therefore cannot provide an application-flush acknowledgement.
grep -Fq "process.on('SIGTERM', () => { interrupt(0) })" "$checkout/apps/cli/src/profile-boot.ts"
grep -Fq "() => { forceExitOnce(code) }" "$checkout/apps/cli/src/process-shutdown.ts"
grep -Fq "timeout = setTimeout(() => { forceExitOnce(code) }, timeoutMs)" "$checkout/apps/cli/src/process-shutdown.ts"

cd "$checkout"
export CI=1
export DSH_E2E_MAX_WORKERS="${DSH_E2E_MAX_WORKERS:-8}"
export DSH_TELEMETRY_DISABLED=1
test "$(pnpm --version)" = "${package_manager#pnpm@}"
pnpm install --frozen-lockfile --ignore-scripts
pnpm rebuild esbuild fs-ext node-pty koffi
pnpm run build:official

vitest="$checkout/node_modules/.bin/vitest"
"$vitest" run --maxWorkers="$DSH_E2E_MAX_WORKERS" \
  packages/api/gateway/tests/gateway-stream.host.spec.ts \
  packages/bundle/web-app/tests/startup.spec.ts \
  packages/client/connection/tests/browser-auth.host.spec.ts \
  packages/client/connection/tests/fetch-routes.host.spec.ts \
  apps/cli/tests/process-shutdown.spec.ts
"$vitest" run --config vitest.e2e.config.ts \
  apps/cli/tests/web-auth.e2e.ts \
  apps/cli/tests/headless-shutdown.e2e.ts
"$vitest" run --config vitest.expected.config.ts \
  apps/cli/tests/profiles/headless/tests/session-format-guard.expected.e2e.ts

cd "$repo_root"
DSH_REAL_CLI="$checkout/apps/cli/lib/bin.js" go test -count=1 -run TestRealDSHBrowserExchange ./internal/dshcompat/launcher
