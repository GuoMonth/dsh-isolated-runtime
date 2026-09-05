#!/usr/bin/env bash
# Test the exact downloadable archive. Only the external model is replaced.
set -Eeuo pipefail
repo_root="$(cd "$(dirname "$0")/.." && pwd)"
candidate="$(cd "${1:?candidate directory required}" && pwd)"
node "$repo_root/hack/check-release.mjs" "$candidate"
proof_root="$(mktemp -d)"
export DSH_DEMO_HOME="$proof_root/state"
export MVP_MODE="${MVP_MODE:-deterministic}"
mkdir -p "$proof_root/bundle"
tar -xzf "$candidate"/*.tar.gz -C "$proof_root/bundle" --strip-components=1
bundle="$proof_root/bundle"
cleanup() {
  local status=$?
  if ((status != 0)); then echo "MVP proof failed; private diagnostics retained: $proof_root" >&2
  elif [[ "${MVP_KEEP_DEMO:-0}" != 1 ]]; then "$bundle/demo" down; rm -rf "$proof_root"; fi
}
trap cleanup EXIT
"$bundle/demo" up --snapshots
export PATH="$DSH_DEMO_HOME/tools/bin:$PATH"
k() { kubectl --kubeconfig "$DSH_DEMO_HOME/kubeconfig" "$@"; }
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
      --env "MVP_MODE=$MVP_MODE" --env "MVP_RESUME=${MVP_RESUME:-0}" \
      --env PLAYWRIGHT_BROWSERS_PATH=/ms-playwright "${mounts[@]}" \
      "$playwright_image" node "$bundle/test/e2e/mvp/journey.cjs"
  fi
}
browser_run
marker="$(jq -r .marker "$DSH_DEMO_HOME/runtime/journey.json")"
test "$(k -n tenant-demo exec "cell-$uid-0" -- cat "/var/lib/dsh/data/workspace/$marker.txt")" = "$marker"
k -n tenant-demo delete pod "cell-$uid-0" --wait=true
k -n tenant-demo rollout status "statefulset/cell-$uid" --timeout=300s
MVP_RESUME=1 browser_run
if [[ "$MVP_MODE" == deterministic ]]; then
  k -n dsh-system exec mvp-provider -- node -e 'fetch("http://127.0.0.1:18080/evidence").then(r=>r.json()).then(e=>{if(e.readResults<2||e.writeResults<2)process.exit(1)})'
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
k -n tenant-demo exec "cell-$restored_uid-0" -- sh -c '! grep -q DEEPSEEK_API_KEY /var/lib/dsh-private/.credentials.yaml'
# Repeating up preserves the source UID and data, and does not replace TLS state.
"$bundle/demo" up --snapshots
test "$(k -n tenant-demo get cell assistant -o jsonpath='{.metadata.uid}')" = "$uid"
mkdir -p "$candidate/evidence"
node "$repo_root/hack/check-release.mjs" "$candidate" | \
  jq --arg kind "$MVP_MODE" --arg model "${MVP_MODEL:-deepseek-v4-flash}" \
    '. + {kind:$kind,model:$model,success:true}' > "$candidate/evidence/$MVP_MODE.json"
echo "MVP $MVP_MODE exact-archive acceptance passed"
