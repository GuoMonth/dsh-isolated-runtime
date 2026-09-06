#!/usr/bin/env bash
# Native Apple Silicon host proof; Docker Desktop itself is a maintainer follow-up.
# shellcheck disable=SC1091,SC2154
set -Eeuo pipefail
umask 077
repo_root="$(cd "$(dirname "$0")/.." && pwd)"
candidate="$(cd "${1:?candidate directory}" && pwd)"
[[ "$(uname -s)/$(uname -m)" == Darwin/arm64 ]]
node "$repo_root/hack/check-release.mjs" "$candidate"
proof_root="$(mktemp -d)"
trap 'rm -rf "$proof_root"' EXIT
mkdir -p "$proof_root/bundle" "$candidate/evidence"
tar -xzf "$candidate"/*.tar.gz -C "$proof_root/bundle" --strip-components=1
bundle="$proof_root/bundle"
demo_root="$proof_root/state with spaces"
test_root="$demo_root/runtime"
mkdir -p "$test_root"
source "$bundle/demo-files/host.sh"
source "$bundle/demo-files/tools.sh"
source "$bundle/hack/lib/browser-stack.sh"
# Use stock macOS utilities and download native private tools, without Homebrew.
export PATH=/usr/bin:/bin:/usr/sbin:/sbin
initialize_demo_host
prepare_demo_tools
[[ "$(node -p 'process.platform+"/"+process.arch')" == darwin/arm64 ]]
kind version
kubectl version --client -o json >/dev/null
jq --version
printf 'checksum probe\n' > "$test_root/checksum"
(cd "$test_root" && sha256sum checksum > checksums && sha256sum -c checksums)
create_reference_certificates
openssl verify -CAfile "$test_root/ca.crt" "$test_root/gateway.crt" "$test_root/dex.crt"
openssl x509 -in "$test_root/gateway.crt" -noout -text | grep -F 'DNS:*.cells.test'
# Kernel-held locks must reject a concurrent owner and release across processes.
exec 9>"$demo_root/lock"
demo_lock acquire
contend() {
  bash -c 'source "$1"; initialize_demo_host; exec 9>"$2"; demo_lock acquire' bash "$bundle/demo-files/host.sh" "$demo_root/lock"
}
if contend; then echo 'Concurrent macOS demo lock was accepted' >&2; exit 1; fi
demo_lock release
contend
# Saved PID metadata must not allow killing a different command or reused PID.
node -e 'setInterval(()=>{},1000)' dsh-process-probe "$demo_root" 9>&- &
probe_pid=$!
record_demo_process "$test_root/probe.pid" "$probe_pid"
stop_demo_process "$test_root/probe.pid" unrelated-process "$demo_root"
kill -0 "$probe_pid"
record_demo_process "$test_root/probe.pid" "$probe_pid"
printf 'different process start\n' > "$test_root/probe.pid.started"
stop_demo_process "$test_root/probe.pid" dsh-process-probe "$demo_root"
kill -0 "$probe_pid"
record_demo_process "$test_root/probe.pid" "$probe_pid"
stop_demo_process "$test_root/probe.pid" dsh-process-probe "$demo_root"
wait "$probe_pid" 2>/dev/null || true
if kill -0 "$probe_pid" 2>/dev/null; then echo 'Owned process did not stop' >&2; exit 1; fi
mkdir -p "$demo_root/browser"
cp "$bundle/test/e2e/phase2/package"*.json "$demo_root/browser/"
npm --prefix "$demo_root/browser" ci --ignore-scripts --no-audit --no-fund
export PLAYWRIGHT_BROWSERS_PATH="$demo_root/browser/browsers"
node "$demo_root/browser/node_modules/playwright/cli.js" install chromium
node "$bundle/test/e2e/mvp/host.cjs" "$demo_root" "$candidate/evidence/macos-browser.png"
node "$repo_root/hack/check-release.mjs" "$candidate" | jq \
  '. + {kind:"macos-host",success:true,hostPlatform:"darwin/arm64",dockerDesktopEndToEnd:"not-run",checks:["native private tools","stock TLS and checksums","exclusive lock","owned process cleanup","native Chromium DNS/TLS/profile lifecycle"]}' \
  > "$candidate/evidence/macos-host.json"
echo 'Apple Silicon native tools, lifecycle helpers and real Chromium host proof passed'
