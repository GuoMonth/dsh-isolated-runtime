#!/usr/bin/env bash
# Test the exact downloadable archive. Only the external model is replaced.
# shellcheck disable=SC1091,SC2154
set -Eeuo pipefail
repo_root="$(cd "$(dirname "$0")/.." && pwd)"
candidate="$(cd "${1:?candidate directory required}" && pwd)"
node "$repo_root/hack/check-release.mjs" "$candidate"
proof_root="$(mktemp -d)"
export DSH_DEMO_HOME="$proof_root/state"
export MVP_MODE="${MVP_MODE:-deterministic}"
export MVP_MODEL="${MVP_MODEL:-$([[ "$MVP_MODE" == deterministic ]] && echo deepseek-v4-flash-vision-exp || echo deepseek-v4-flash)}"
export DOCKER_CONFIG="$proof_root/docker"
mkdir -p "$DOCKER_CONFIG"
mkdir -p "$proof_root/bundle"
tar -xzf "$candidate"/*.tar.gz -C "$proof_root/bundle" --strip-components=1
bundle="$proof_root/bundle"
cleanup() {
  local status=$?
  if [[ -n "${port_pid:-}" ]]; then kill "$port_pid" 2>/dev/null || true; fi
  if [[ -n "${saved_owner:-}" ]]; then printf '%s\n' "$saved_owner" > "$DSH_DEMO_HOME/owner"; fi
  if ((status != 0)); then
    echo "MVP proof failed; private diagnostics retained: $proof_root" >&2
    mkdir -p "$candidate/evidence"
    kubectl --kubeconfig "$DSH_DEMO_HOME/kubeconfig" get pods,cells,httproutes -A > "$candidate/evidence/objects.txt" 2>&1 || true
    node "$repo_root/hack/check-release.mjs" "$candidate" | jq --arg kind "$MVP_MODE" '. + {kind:$kind,success:false}' > "$candidate/evidence/failure.json"
  elif [[ "${MVP_KEEP_DEMO:-0}" != 1 ]]; then "$bundle/demo" down; rm -rf "$proof_root"; fi
}
trap cleanup EXIT
# A real occupied listener must fail before creating a cluster.
node -e 'const fs=require("node:fs");require("node:net").createServer().listen(18443,"127.0.0.1",()=>fs.writeFileSync(process.argv[1],"ready"))' "$proof_root/port-ready" > "$proof_root/port.log" 2>&1 &
port_pid=$!
for _ in $(seq 1 50); do [[ -f "$proof_root/port-ready" ]] && break; sleep 0.1; done
if [[ ! -f "$proof_root/port-ready" ]]; then echo 'Free demo ports 18443 and 15556 before acceptance' >&2; exit 1; fi
if "$bundle/demo" up --snapshots > "$proof_root/port-conflict.log" 2>&1; then
  kill "$port_pid"; echo 'Demo accepted an occupied port' >&2; exit 1
fi
kill "$port_pid"; wait "$port_pid" 2>/dev/null || true
unset port_pid
grep -Fq 'Local port 18443 is occupied' "$proof_root/port-conflict.log"
test ! -f "$DSH_DEMO_HOME/owner"
"$bundle/demo" up --snapshots
export PATH="$DSH_DEMO_HOME/tools/bin:$PATH"
k() { kubectl --kubeconfig "$DSH_DEMO_HOME/kubeconfig" "$@"; }
wait_cell() {
  for _ in $(seq 1 300); do
    if k -n tenant-demo get cell "$1" -o json | jq -e '.status.observedGeneration==.metadata.generation and any(.status.conditions[]?; .type=="Ready" and .status=="True")' >/dev/null; then return; fi
    sleep 1
  done
  echo "Cell $1 did not converge" >&2; return 1
}
cell_image="$(jq -r .images.cell "$candidate/release.json")"
if [[ "$MVP_MODE" == deterministic ]]; then
  k -n dsh-system create configmap mvp-provider --from-file=provider.cjs="$bundle/test/e2e/mvp/provider.cjs"
  k -n dsh-system apply -f - <<EOF
apiVersion: v1
kind: Pod
metadata:
  name: mvp-provider
  labels: {app: mvp-provider}
spec:
  automountServiceAccountToken: false
  containers:
    - name: provider
      image: $cell_image
      command: [node, /fixture/provider.cjs]
      ports: [{containerPort: 18080}]
      volumeMounts: [{name: fixture, mountPath: /fixture, readOnly: true}]
  volumes:
    - name: fixture
      configMap: {name: mvp-provider}
---
apiVersion: v1
kind: Service
metadata: {name: mvp-provider}
spec:
  selector: {app: mvp-provider}
  ports: [{port: 18080, targetPort: 18080}]
EOF
  k -n dsh-system wait pod mvp-provider --for=condition=Ready --timeout=180s
  k -n tenant-demo create secret generic mvp-provider-endpoint --from-literal=DEEPSEEK_BASE_URL=http://mvp-provider.dsh-system.svc:18080
  k -n tenant-demo patch cell assistant --type=merge -p '{"spec":{"credentialsRef":{"name":"mvp-provider-endpoint"}}}'
fi
uid="$(k -n tenant-demo get cell assistant -o jsonpath='{.metadata.uid}')"
wait_cell assistant
k -n tenant-demo rollout status "statefulset/cell-$uid" --timeout=300s
mkdir -p "$DSH_DEMO_HOME/browser"
cp "$bundle/test/e2e/phase2/package"*.json "$DSH_DEMO_HOME/browser/"
npm --prefix "$DSH_DEMO_HOME/browser" ci --ignore-scripts --no-audit --no-fund
source "$repo_root/hack/lib/reference-versions.sh"
browser_run() {
  if [[ -n "${CHROME_EXECUTABLE:-}" ]]; then
    node "$bundle/test/e2e/mvp/journey.cjs"
  else
    local mounts=()
    if [[ "$MVP_MODE" == live-model ]]; then
      [[ -f "${DEEPSEEK_KEY_FILE:-}" ]] || { echo 'DEEPSEEK_KEY_FILE is required' >&2; return 1; }
      mounts+=(--volume "$DEEPSEEK_KEY_FILE:/run/deepseek-key:ro" --env DEEPSEEK_KEY_FILE=/run/deepseek-key)
    fi
    docker run --rm --network host --shm-size=1g --user "$(id -u):$(id -g)" \
      --volume "$proof_root:$proof_root" --env "DSH_DEMO_HOME=$DSH_DEMO_HOME" \
      --env "MVP_MODEL=$MVP_MODEL" --env "MVP_MODE=$MVP_MODE" --env "MVP_RESUME=${MVP_RESUME:-0}" \
      --env "MVP_RESTORED=${MVP_RESTORED:-0}" \
      --env PLAYWRIGHT_BROWSERS_PATH=/ms-playwright "${mounts[@]}" \
      "$playwright_image" node "$bundle/test/e2e/mvp/journey.cjs"
  fi
}
browser_run
marker="$(jq -r .marker "$DSH_DEMO_HOME/runtime/journey.json")"
storage_check() {
  k -n tenant-demo exec -i "cell-$1-0" -- node - "$marker" "$MVP_MODE" "$2" < "$bundle/test/e2e/mvp/storage.cjs"
}
storage_check "$uid" initial
k -n tenant-demo delete pod "cell-$uid-0" --wait=true
k -n tenant-demo rollout status "statefulset/cell-$uid" --timeout=300s
MVP_RESUME=1 browser_run
storage_check "$uid" restarted
if [[ "$MVP_MODE" == deterministic ]]; then
  k -n dsh-system exec mvp-provider -- node -e 'fetch("http://127.0.0.1:18080/evidence").then(r=>r.json()).then(e=>{if(e.readResults<2||e.writeResults<2||e.attachmentReads<2||e.images<1)process.exit(1)})'
fi
# Fresh restore from actual completed model/tool state, using the same image.
k -n tenant-demo apply -f - <<EOF
apiVersion: dsh.isolated.io/v1alpha1
kind: CellSnapshot
metadata: {name: mvp-backup}
spec:
  cellRef: {name: assistant}
  volumeSnapshotClassName: csi-hostpath-snapclass
EOF
k -n tenant-demo wait cellsnapshot mvp-backup --for=condition=Ready --timeout=720s
k -n tenant-demo apply -f - <<EOF
apiVersion: dsh.isolated.io/v1alpha1
kind: Cell
metadata: {name: restored}
spec:
  image: $cell_image
  storage:
    size: 1Gi
    storageClassName: csi-hostpath-sc
    restoreFrom: {name: mvp-backup}
EOF
k -n tenant-demo wait cell restored --for=condition=Ready --timeout=300s
restored_uid="$(k -n tenant-demo get cell restored -o jsonpath='{.metadata.uid}')"
test "$restored_uid" != "$uid"
test "$(k -n tenant-demo exec "cell-$restored_uid-0" -- cat "/var/lib/dsh/data/workspace/$marker.txt")" = "$marker"
storage_check "$restored_uid" fresh-restore
if [[ "$MVP_MODE" == deterministic ]]; then
  k -n tenant-demo patch cell restored --type=merge -p '{"spec":{"credentialsRef":{"name":"mvp-provider-endpoint"}}}'
  wait_cell restored
fi
k -n tenant-demo create rolebinding restored-access --role="cell-$restored_uid-access" \
  --user='https://dex.dsh-system.svc:15556/dex#CglhbGljZS1zdWISBWxvY2Fs'
k -n tenant-demo get httproute "cell-$restored_uid" -o jsonpath='{.spec.hostnames[0]}' > "$DSH_DEMO_HOME/runtime/hostname"
MVP_RESTORED=1 browser_run
storage_check "$restored_uid" restored-configured
# Repeating up preserves the source UID and data, and does not replace TLS state.
"$bundle/demo" up --snapshots
test "$(k -n tenant-demo get cell assistant -o jsonpath='{.metadata.uid}')" = "$uid"
# Refuse deletion when ownership evidence disagrees, preserving all Cells.
saved_owner="$(cat "$DSH_DEMO_HOME/owner")"
printf 'unowned\n' > "$DSH_DEMO_HOME/owner"
if "$bundle/demo" down > "$proof_root/ownership.log" 2>&1; then
  echo 'Demo deleted resources without matching ownership' >&2; exit 1
fi
printf '%s\n' "$saved_owner" > "$DSH_DEMO_HOME/owner"
unset saved_owner
test "$(k -n tenant-demo get cell assistant -o jsonpath='{.metadata.uid}')" = "$uid"
# Explicit teardown is destructive only to the owned demo, and idempotent.
"$bundle/demo" down
"$bundle/demo" down
test ! -f "$DSH_DEMO_HOME/kubeconfig"
# Also prove the default installation works without opting into CSI/metrics.
"$bundle/demo" up
k -n tenant-demo get cell assistant -o json | jq -e '.spec.storage.storageClassName=="standard"' >/dev/null
if k get crd volumesnapshots.snapshot.storage.k8s.io >/dev/null 2>&1; then echo 'Default demo installed snapshots unexpectedly' >&2; exit 1; fi
k -n dsh-system get deployment cell-operator -o json | jq -e 'all(.spec.template.spec.containers[0].args[]; (startswith("--metrics-bind-address=")|not) or .=="--metrics-bind-address=0")' >/dev/null
mkdir -p "$candidate/evidence"
node "$repo_root/hack/check-release.mjs" "$candidate" | \
  jq --arg kind "$MVP_MODE" --arg model "$MVP_MODEL" \
    '. + {kind:$kind,model:$model,success:true}' > "$candidate/evidence/$MVP_MODE.json"
echo "MVP $MVP_MODE exact-archive acceptance passed"
