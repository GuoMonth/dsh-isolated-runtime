# shellcheck shell=bash
# shellcheck disable=SC2016,SC2034,SC2154
# This file is sourced by hack/verify-phase2-kind.sh after its complete browser
# proof. It deliberately reuses that cluster, registry, Gateway, Dex and
# Chromium state while adding only the CSI data-lifecycle fixture.

snapshotter_commit="5aab051d1af135e2c852f6fb7fc27fa709d877bf"
hostpath_commit="cc78ee78ae23908c9e0607df2fe09c7ecfa52597"
snapshotter_root="$test_root/external-snapshotter"
hostpath_root="$test_root/csi-driver-host-path"
phase3_provider_value="phase3-provider-51bd0c"

wait_snapshot_condition() {
  local namespace="$1" name="$2" condition="$3" status="$4" reason="${5:-}" json
  for _ in $(seq 1 600); do
    json="$(k -n "$namespace" get cellsnapshot "$name" -o json 2>/dev/null || true)"
    if [[ -n "$json" ]] && jq -e --arg condition "$condition" --arg status "$status" --arg reason "$reason" '
      .status.observedGeneration == .metadata.generation and
      any(.status.conditions[]?;
        .type == $condition and .status == $status and ($reason == "" or .reason == $reason))
    ' <<<"$json" >/dev/null; then
      return
    fi
    sleep 1
  done
  echo "CellSnapshot $namespace/$name did not reach $condition=$status reason=$reason" >&2
  return 1
}

wait_cell_condition() {
  local namespace="$1" name="$2" condition="$3" status="$4" reason="${5:-}" json
  for _ in $(seq 1 240); do
    json="$(k -n "$namespace" get cell "$name" -o json 2>/dev/null || true)"
    if [[ -n "$json" ]] && jq -e --arg condition "$condition" --arg status "$status" --arg reason "$reason" '
      .status.observedGeneration == .metadata.generation and
      any(.status.conditions[]?;
        .type == $condition and .status == $status and ($reason == "" or .reason == $reason))
    ' <<<"$json" >/dev/null; then
      return
    fi
    sleep 1
  done
  echo "Cell $namespace/$name did not reach $condition=$status reason=$reason" >&2
  return 1
}

wait_absent() {
  local namespace="$1" resource="$2" name="$3"
  for _ in $(seq 1 180); do
    if ! k -n "$namespace" get "$resource" "$name" >/dev/null 2>&1; then
      return
    fi
    sleep 1
  done
  echo "$resource $namespace/$name was not deleted" >&2
  return 1
}

checkout_exact() {
  local url="$1" commit="$2" destination="$3"
  git init -q "$destination"
  git -C "$destination" remote add origin "$url"
  git -C "$destination" fetch -q --depth=1 origin "$commit"
  git -C "$destination" checkout -q --detach FETCH_HEAD
  test "$(git -C "$destination" rev-parse HEAD)" = "$commit"
}

checkout_exact https://github.com/kubernetes-csi/external-snapshotter.git \
  "$snapshotter_commit" "$snapshotter_root"
checkout_exact https://github.com/kubernetes-csi/csi-driver-host-path.git \
  "$hostpath_commit" "$hostpath_root"

# Pull once through the host daemon and import exact amd64 images into kind.
# This avoids serial kubelet pulls and makes registry transients recoverable.
csi_images=(
  registry.k8s.io/sig-storage/snapshot-controller:v8.5.0
  registry.k8s.io/sig-storage/hostpathplugin:v1.18.0
  registry.k8s.io/sig-storage/csi-external-health-monitor-controller:v0.18.0
  registry.k8s.io/sig-storage/csi-node-driver-registrar:v2.17.0
  registry.k8s.io/sig-storage/livenessprobe:v2.19.0
  registry.k8s.io/sig-storage/csi-attacher:v4.12.0
  registry.k8s.io/sig-storage/csi-provisioner:v6.3.0
  registry.k8s.io/sig-storage/csi-resizer:v2.2.1
  registry.k8s.io/sig-storage/csi-snapshotter:v8.5.0
)
printf '%s\0' "${csi_images[@]}" | xargs -0 -n1 -P4 docker pull --platform linux/amd64 >/dev/null
for image in "${csi_images[@]}"; do
  docker save "$image" | docker exec --privileged -i "${cluster_name}-control-plane" \
    ctr --namespace=k8s.io images import --snapshotter=overlayfs - >/dev/null
done

for crd in volumesnapshotclasses volumesnapshotcontents volumesnapshots; do
  k apply --server-side -f \
    "$snapshotter_root/client/config/crd/snapshot.storage.k8s.io_${crd}.yaml"
done
k wait --for=condition=Established \
  crd/volumesnapshotclasses.snapshot.storage.k8s.io \
  crd/volumesnapshotcontents.snapshot.storage.k8s.io \
  crd/volumesnapshots.snapshot.storage.k8s.io --timeout=120s
k apply -f "$snapshotter_root/deploy/kubernetes/snapshot-controller/rbac-snapshot-controller.yaml"
sed 's#registry.k8s.io/sig-storage/snapshot-controller:v8.4.0#registry.k8s.io/sig-storage/snapshot-controller:v8.5.0#' \
  "$snapshotter_root/deploy/kubernetes/snapshot-controller/setup-snapshot-controller.yaml" | k apply -f -
k -n kube-system rollout status deployment/snapshot-controller --timeout=300s

KUBECONFIG="$kubeconfig" \
CSI_SNAPSHOTTER_TAG=v8.5.0 \
HOSTPATHPLUGIN_TAG=v1.18.0 \
  "$hostpath_root/deploy/kubernetes-1.34/deploy.sh"
k apply -f "$hostpath_root/examples/csi-storageclass.yaml"
k get volumesnapshotclass csi-hostpath-snapclass -o json | jq -e '
  .driver == "hostpath.csi.k8s.io" and .deletionPolicy == "Delete"
' >/dev/null

# Restart only the operator after the stable snapshot API exists. The Phase 2
# deployment and Cell contracts remain otherwise unchanged.
k -n dsh-system patch deployment cell-operator --type=strategic -p "$(jq -cn '{spec:{template:{spec:{containers:[{name:"manager",args:["--gateway-name=dsh","--gateway-namespace=dsh-system","--gateway-section-name=https","--base-domain=cells.test","--external-https-port=18443","--enable-snapshots=true","--writer-stop-timeout=2m","--snapshot-timeout=1m"]}]}}}}')"
k -n dsh-system rollout status deployment/cell-operator --timeout=180s

# Build a second exact-RC image whose only intended difference is immutable OCI
# revision metadata, plus an E2E-only failure image that writes after rollout
# and exits before readiness.
local_cell_b="dsh-phase3-cell-b:test"
docker buildx build --platform linux/amd64 --load \
  --build-arg "SOURCE_REVISION=${revision}-equivalent-b" \
  -f "$repo_root/images/cell/Dockerfile" -t "$local_cell_b" "$repo_root"
docker tag "$local_cell_b" "$cell_repo:e2e-b"
docker push "$cell_repo:e2e-b" >/dev/null
cell_b_repo_digest="$(docker inspect "$cell_repo:e2e-b" --format '{{index .RepoDigests 0}}')"
cell_b_digest="${cell_b_repo_digest##*@}"
test "$cell_b_digest" != "$cell_digest"
test "$(docker run --rm --entrypoint node "$local_cell_b" -e "process.stdout.write(require('/opt/dsh/node_modules/@deepseek-ai/dsh/package.json').version)")" = "0.1.2-rc.1"
test "$(docker image inspect "$local_cell" --format '{{json .RootFS.Layers}}')" = \
  "$(docker image inspect "$local_cell_b" --format '{{json .RootFS.Layers}}')"
runtime_config_a="$(docker image inspect "$local_cell" | jq -cS '.[0].Config | {User,Entrypoint,Cmd,Env,WorkingDir,Labels:(.Labels | del(."org.opencontainers.image.revision"))}')"
runtime_config_b="$(docker image inspect "$local_cell_b" | jq -cS '.[0].Config | {User,Entrypoint,Cmd,Env,WorkingDir,Labels:(.Labels | del(."org.opencontainers.image.revision"))}')"
test "$runtime_config_a" = "$runtime_config_b"

fault_image="dsh-phase3-fault:test"
docker buildx build --builder default --platform linux/amd64 --load \
  --build-arg "CELL_BASE=$local_cell" \
  -f "$repo_root/test/e2e/phase3/Dockerfile.fault" -t "$fault_image" "$repo_root"
docker tag "$fault_image" "$cell_repo:fault"
docker push "$cell_repo:fault" >/dev/null
fault_repo_digest="$(docker inspect "$cell_repo:fault" --format '{{index .RepoDigests 0}}')"
fault_digest="${fault_repo_digest##*@}"

k create namespace tenant-snapshot
k label namespace tenant-snapshot dsh.isolated.io/routes=enabled
k -n tenant-snapshot create secret generic phase3-provider \
  --from-literal="PHASE3_PROVIDER_MARKER=$phase3_provider_value"
k apply -f - <<EOF
apiVersion: dsh.isolated.io/v1alpha1
kind: Cell
metadata:
  name: source
  namespace: tenant-snapshot
spec:
  image: ${cell_repo}@${cell_digest}
  credentialsRef:
    name: phase3-provider
  storage:
    size: 1Gi
    storageClassName: csi-hostpath-sc
    retentionPolicy: Retain
EOF
wait_cell tenant-snapshot source True
source_uid="$(k -n tenant-snapshot get cell source -o jsonpath='{.metadata.uid}')"
source_base="cell-${source_uid}"
source_host="${source_base}.cells.test"
source_authority="${source_host}:18443"
wait_route tenant-snapshot "$source_base"
k -n tenant-snapshot create rolebinding alice-source \
  --role="${source_base}-access" --user="${dex_issuer}#${dex_alice_sub}"

# Establish real DSH state and keep a real upgraded session/follow connection
# open in the same authenticated browser context while the snapshot starts.
cell_a_host="$source_host"
cell_a_authority="$source_authority"
hold_marker="$test_root/browser/hold-ready"
hold_http_marker="$test_root/browser/hold-http-ready"
hold_log="$test_root/browser/hold.log"
rm -f "$hold_marker" "$hold_http_marker"
browser initial-hold alice@example.com >"$hold_log" 2>&1 &
hold_pid=$!
extension_pids+=("$hold_pid")
for _ in $(seq 1 90); do
  [[ -f "$hold_marker" && -f "$hold_http_marker" ]] && break
  sleep 1
done
if [[ ! -f "$hold_marker" || ! -f "$hold_http_marker" ]]; then
  sed -n '1,160p' "$hold_log" >&2
  echo "Phase 3 browser did not establish held WebSocket and HTTP streams" >&2
  exit 1
fi

source_pod="$(k -n tenant-snapshot get pod -l "dsh.isolated.io/cell-uid=${source_uid}" -o jsonpath='{.items[0].metadata.name}')"
k -n tenant-snapshot exec "$source_pod" -- sh -euc '
  mkdir -p /var/lib/dsh/data/workspace
  printf durable-workspace > /var/lib/dsh/data/workspace/phase3-source
  printf durable-home > /var/lib/dsh/data/home/phase3-home
  find /var/lib/dsh/data/home/attachments/v1 -type f | grep -q .
  printf private-source-only > /var/lib/dsh-private/phase3-private
  test "$PHASE3_PROVIDER_MARKER" = phase3-provider-51bd0c
'
source_private_hash="$(k -n tenant-snapshot exec "$source_pod" -- sha256sum /var/lib/dsh-private/.credentials.yaml | awk '{print $1}')"
k apply -f - <<EOF
apiVersion: v1
kind: Pod
metadata:
  name: writer-fence-probe
  namespace: tenant-snapshot
  labels:
    test.dsh.isolated.io/purpose: writer-fence-label-drift
  annotations:
    dsh.isolated.io/cell-name: source
    dsh.isolated.io/cell-uid: ${source_uid}
spec:
  restartPolicy: Never
  containers:
    - name: fence
      image: ${cell_repo}@${cell_digest}
      command: ["/usr/local/bin/node", "-e", "setInterval(()=>{},60000)"]
EOF
k -n tenant-snapshot wait pod/writer-fence-probe --for=condition=Ready --timeout=120s

# Hold the external snapshot-controller so the first operation remains active
# long enough to prove that a concurrent request exposes OperationQueued. The
# hostpath driver can otherwise complete a snapshot between two 1-second API
# observations on a fast machine.
k -n kube-system scale deployment snapshot-controller --replicas=0
k -n kube-system wait --for=delete pod -l app=snapshot-controller --timeout=120s

k apply -f - <<EOF
apiVersion: dsh.isolated.io/v1alpha1
kind: CellSnapshot
metadata:
  name: source-backup
  namespace: tenant-snapshot
spec:
  cellRef:
    name: source
  volumeSnapshotClassName: csi-hostpath-snapclass
EOF
wait_snapshot_condition tenant-snapshot source-backup Accepted True
k apply -f - <<EOF
apiVersion: dsh.isolated.io/v1alpha1
kind: CellSnapshot
metadata:
  name: queued-backup
  namespace: tenant-snapshot
spec:
  cellRef:
    name: source
  volumeSnapshotClassName: csi-hostpath-snapclass
EOF
wait_snapshot_condition tenant-snapshot queued-backup Accepted False OperationQueued
k -n tenant-snapshot delete cellsnapshot queued-backup --wait=true

# An unowned and deliberately unlabeled Pod carrying the exact Cell identity
# blocks the CSI action even after the StatefulSet itself reports zero.
for _ in $(seq 1 120); do
  if [[ "$(k -n tenant-snapshot get statefulset "$source_base" -o jsonpath='{.spec.replicas}')" = "0" &&
        "$(k -n tenant-snapshot get statefulset "$source_base" -o jsonpath='{.status.replicas}')" = "0" ]]; then
    break
  fi
  sleep 1
done
volume_snapshot="cellsnapshot-$(k -n tenant-snapshot get cellsnapshot source-backup -o jsonpath='{.metadata.uid}')"
expect_failure k -n tenant-snapshot get volumesnapshot "$volume_snapshot"
k -n tenant-snapshot delete pod writer-fence-probe --wait=true
wait_snapshot_condition tenant-snapshot source-backup WriterStopped True

# Kill the controller after the durable writer-stop barrier. Both reconcilers
# must recover from API state alone and resume the source without an in-memory
# transaction record.
k -n dsh-system scale deployment cell-operator --replicas=0
k -n dsh-system wait --for=delete pod -l app.kubernetes.io/name=cell-operator --timeout=120s
k -n dsh-system scale deployment cell-operator --replicas=1
k -n dsh-system rollout status deployment/cell-operator --timeout=180s

# The VolumeSnapshot may only exist after the StatefulSet is fully at zero.
for _ in $(seq 1 180); do
  if k -n tenant-snapshot get volumesnapshot "$volume_snapshot" >/dev/null 2>&1; then
    test "$(k -n tenant-snapshot get statefulset "$source_base" -o jsonpath='{.spec.replicas}')" = "0"
    test "$(k -n tenant-snapshot get statefulset "$source_base" -o jsonpath='{.status.replicas}')" = "0"
    break
  fi
  sleep 1
done
k -n tenant-snapshot get volumesnapshot "$volume_snapshot" >/dev/null
k -n kube-system scale deployment snapshot-controller --replicas=2
k -n kube-system rollout status deployment/snapshot-controller --timeout=180s
if ! wait "$hold_pid"; then
  sed -n '1,160p' "$hold_log" >&2
  echo "Phase 3 active-connection fixture failed" >&2
  exit 1
fi
extension_pids=()
wait_snapshot_condition tenant-snapshot source-backup Ready True
wait_cell tenant-snapshot source True
test -z "$(k -n tenant-snapshot get cell source -o jsonpath='{.metadata.annotations.dsh\.isolated\.io/active-snapshot-uid}')"
k -n tenant-snapshot rollout status statefulset "$source_base" --timeout=180s
browser resume alice@example.com

# Restore into a fresh Cell. The data claim points at the CSI object while the
# private claim is blank, so old provider/signing material cannot cross over.
k apply -f - <<EOF
apiVersion: dsh.isolated.io/v1alpha1
kind: Cell
metadata:
  name: restored
  namespace: tenant-snapshot
spec:
  image: ${cell_repo}@${cell_digest}
  storage:
    size: 1Gi
    retentionPolicy: Delete
    restoreFrom:
      name: source-backup
EOF
wait_cell tenant-snapshot restored True
restored_uid="$(k -n tenant-snapshot get cell restored -o jsonpath='{.metadata.uid}')"
restored_base="cell-${restored_uid}"
restored_host="${restored_base}.cells.test"
restored_authority="${restored_host}:18443"
wait_route tenant-snapshot "$restored_base"
k -n tenant-snapshot create rolebinding alice-restored \
  --role="${restored_base}-access" --user="${dex_issuer}#${dex_alice_sub}"
test "$(k -n tenant-snapshot get pvc "${restored_base}-data" -o jsonpath='{.spec.dataSource.apiGroup}')" = snapshot.storage.k8s.io
test "$(k -n tenant-snapshot get pvc "${restored_base}-data" -o jsonpath='{.spec.dataSource.kind}')" = VolumeSnapshot
test "$(k -n tenant-snapshot get pvc "${restored_base}-data" -o jsonpath='{.spec.dataSource.name}')" = "$volume_snapshot"
test -z "$(k -n tenant-snapshot get pvc "${restored_base}-private" -o jsonpath='{.spec.dataSource}')"
restored_pod="$(k -n tenant-snapshot get pod -l "dsh.isolated.io/cell-uid=${restored_uid}" -o jsonpath='{.items[0].metadata.name}')"
k -n tenant-snapshot exec "$restored_pod" -- sh -euc '
  test "$(cat /var/lib/dsh/data/workspace/phase3-source)" = durable-workspace
  test "$(cat /var/lib/dsh/data/home/phase3-home)" = durable-home
  find /var/lib/dsh/data/home/attachments/v1 -type f | grep -q .
  test ! -e /var/lib/dsh-private/phase3-private
  test -z "${PHASE3_PROVIDER_MARKER:-}"
'
restored_private_hash="$(k -n tenant-snapshot exec "$restored_pod" -- sha256sum /var/lib/dsh-private/.credentials.yaml | awk '{print $1}')"
test "$restored_private_hash" != "$source_private_hash"
cell_b_host="$source_host"
cell_b_authority="$source_authority"
cell_a_host="$restored_host"
cell_a_authority="$restored_authority"
browser resume alice@example.com

# A same-RC digest change is an ordinary StatefulSet rollout and keeps both
# protocol and persisted data intact.
restored_revision_a="$(k -n tenant-snapshot get statefulset "$restored_base" -o jsonpath='{.status.currentRevision}')"
k -n tenant-snapshot patch cell restored --type=merge \
  -p "{\"spec\":{\"image\":\"${cell_repo}@${cell_b_digest}\"}}"
wait_cell tenant-snapshot restored True
k -n tenant-snapshot rollout status statefulset "$restored_base" --timeout=180s
restored_revision_b="$(k -n tenant-snapshot get statefulset "$restored_base" -o jsonpath='{.status.currentRevision}')"
test "$restored_revision_b" != "$restored_revision_a"
test "$(k -n tenant-snapshot get cell restored -o jsonpath='{.status.imageDigest}')" = "$cell_b_digest"
restored_pod="$(k -n tenant-snapshot get pod -l "dsh.isolated.io/cell-uid=${restored_uid}" -o jsonpath='{.items[0].metadata.name}')"
k -n tenant-snapshot exec "$restored_pod" -- test -f /var/lib/dsh/data/workspace/phase3-source
browser resume alice@example.com

# Restore is digest-exact and fails before a PVC or workload is created. The
# mutable image intent can then be corrected and reconciles without cleanup.
k apply -f - <<EOF
apiVersion: dsh.isolated.io/v1alpha1
kind: Cell
metadata:
  name: mismatch
  namespace: tenant-snapshot
spec:
  image: ${cell_repo}@${cell_b_digest}
  storage:
    size: 1Gi
    retentionPolicy: Delete
    restoreFrom:
      name: source-backup
EOF
wait_cell_condition tenant-snapshot mismatch StorageReady False RestoreImageMismatch
mismatch_uid="$(k -n tenant-snapshot get cell mismatch -o jsonpath='{.metadata.uid}')"
mismatch_base="cell-${mismatch_uid}"
expect_failure k -n tenant-snapshot get pvc "${mismatch_base}-data"
expect_failure k -n tenant-snapshot get statefulset "$mismatch_base"
k -n tenant-snapshot patch cell mismatch --type=merge \
  -p "{\"spec\":{\"image\":\"${cell_repo}@${cell_digest}\"}}"
wait_cell tenant-snapshot mismatch True

# Fault injection never creates a second writer. A rollback is a new Cell from
# the immutable old snapshot, not an in-place image or PVC downgrade.
k -n tenant-snapshot patch cell restored --type=merge \
  -p "{\"spec\":{\"image\":\"${cell_repo}@${fault_digest}\"}}"
wait_cell tenant-snapshot restored False
test "$(k -n tenant-snapshot get statefulset "$restored_base" -o jsonpath='{.spec.replicas}')" = "1"
test "$(k -n tenant-snapshot get pod -l "dsh.isolated.io/cell-uid=${restored_uid}" --no-headers | wc -l)" = "1"
fault_marker_seen=false
for _ in $(seq 1 90); do
  fault_pod="$(k -n tenant-snapshot get pod -l "dsh.isolated.io/cell-uid=${restored_uid}" -o jsonpath='{.items[0].metadata.name}' 2>/dev/null || true)"
  if [[ -n "$fault_pod" ]] && k -n tenant-snapshot exec "$fault_pod" -- test -f /var/lib/dsh/data/workspace/incompatible-marker >/dev/null 2>&1; then
    fault_marker_seen=true
    break
  fi
  sleep 1
done
test "$fault_marker_seen" = true

k apply -f - <<EOF
apiVersion: dsh.isolated.io/v1alpha1
kind: Cell
metadata:
  name: rollback
  namespace: tenant-snapshot
spec:
  image: ${cell_repo}@${cell_digest}
  storage:
    size: 1Gi
    retentionPolicy: Delete
    restoreFrom:
      name: source-backup
EOF
wait_cell tenant-snapshot rollback True
rollback_uid="$(k -n tenant-snapshot get cell rollback -o jsonpath='{.metadata.uid}')"
rollback_base="cell-${rollback_uid}"
rollback_host="${rollback_base}.cells.test"
rollback_authority="${rollback_host}:18443"
wait_route tenant-snapshot "$rollback_base"
k -n tenant-snapshot create rolebinding alice-rollback \
  --role="${rollback_base}-access" --user="${dex_issuer}#${dex_alice_sub}"
rollback_pod="$(k -n tenant-snapshot get pod -l "dsh.isolated.io/cell-uid=${rollback_uid}" -o jsonpath='{.items[0].metadata.name}')"
k -n tenant-snapshot exec "$rollback_pod" -- sh -euc '
  test -f /var/lib/dsh/data/workspace/phase3-source
  test ! -e /var/lib/dsh/data/workspace/incompatible-marker
  test ! -e /var/lib/dsh-private/phase3-private
'
cell_b_host="$source_host"
cell_b_authority="$source_authority"
cell_a_host="$rollback_host"
cell_a_authority="$rollback_authority"
browser resume alice@example.com

# Exercise failure interleavings against the real API server while the CSI
# controller is paused so each transition remains observable.
k -n kube-system scale deployment snapshot-controller --replicas=0
k -n kube-system wait --for=delete pod -l app=snapshot-controller --timeout=120s
k -n dsh-system patch deployment cell-operator --type=strategic -p "$(jq -cn '{spec:{template:{spec:{containers:[{name:"manager",args:["--gateway-name=dsh","--gateway-namespace=dsh-system","--gateway-section-name=https","--base-domain=cells.test","--external-https-port=18443","--enable-snapshots=true","--writer-stop-timeout=30s","--snapshot-timeout=1m"]}]}}}}')"
k -n dsh-system rollout status deployment/cell-operator --timeout=180s
k apply -f - <<EOF
apiVersion: v1
kind: Pod
metadata:
  name: writer-stop-timeout-probe
  namespace: tenant-snapshot
  labels:
    test.dsh.isolated.io/purpose: writer-fence-label-drift
  annotations:
    dsh.isolated.io/cell-name: rollback
    dsh.isolated.io/cell-uid: ${rollback_uid}
spec:
  restartPolicy: Never
  containers:
    - name: fence
      image: ${cell_repo}@${cell_digest}
      command: ["/usr/local/bin/node", "-e", "setInterval(()=>{},60000)"]
EOF
k -n tenant-snapshot wait pod/writer-stop-timeout-probe --for=condition=Ready --timeout=120s
k apply -f - <<EOF
apiVersion: dsh.isolated.io/v1alpha1
kind: CellSnapshot
metadata:
  name: writer-stop-timeout
  namespace: tenant-snapshot
spec:
  cellRef:
    name: rollback
  volumeSnapshotClassName: csi-hostpath-snapclass
EOF
wait_snapshot_condition tenant-snapshot writer-stop-timeout Accepted True
wait_snapshot_condition tenant-snapshot writer-stop-timeout Failed False CleanupBlocked
writer_timeout_uid="$(k -n tenant-snapshot get cellsnapshot writer-stop-timeout -o jsonpath='{.metadata.uid}')"
expect_failure k -n tenant-snapshot get volumesnapshot "cellsnapshot-${writer_timeout_uid}"
test "$(k -n tenant-snapshot get statefulset "$rollback_base" -o jsonpath='{.spec.replicas}')" = "0"
k -n tenant-snapshot delete pod writer-stop-timeout-probe --wait=true
wait_snapshot_condition tenant-snapshot writer-stop-timeout Failed True WriterStopTimedOut
wait_cell tenant-snapshot rollback True
k -n tenant-snapshot delete cellsnapshot writer-stop-timeout --wait=true

k apply -f - <<EOF
apiVersion: dsh.isolated.io/v1alpha1
kind: CellSnapshot
metadata:
  name: source-changed
  namespace: tenant-snapshot
spec:
  cellRef:
    name: rollback
  volumeSnapshotClassName: csi-hostpath-snapclass
EOF
wait_snapshot_condition tenant-snapshot source-changed Accepted True
k -n tenant-snapshot patch cell rollback --type=merge -p '{"spec":{"resources":{"requests":{"cpu":"10m"}}}}'
wait_snapshot_condition tenant-snapshot source-changed Failed True SourceChanged
wait_cell tenant-snapshot rollback True
k -n tenant-snapshot delete cellsnapshot source-changed --wait=true

k apply -f - <<EOF
apiVersion: dsh.isolated.io/v1alpha1
kind: CellSnapshot
metadata:
  name: timed-out
  namespace: tenant-snapshot
spec:
  cellRef:
    name: rollback
  volumeSnapshotClassName: csi-hostpath-snapclass
EOF
wait_snapshot_condition tenant-snapshot timed-out WriterStopped True
timed_out_uid="$(k -n tenant-snapshot get cellsnapshot timed-out -o jsonpath='{.metadata.uid}')"
timed_out_volume="cellsnapshot-${timed_out_uid}"
for _ in $(seq 1 60); do
  k -n tenant-snapshot get volumesnapshot "$timed_out_volume" >/dev/null 2>&1 && break
  sleep 1
done
k -n tenant-snapshot get volumesnapshot "$timed_out_volume" >/dev/null
wait_snapshot_condition tenant-snapshot timed-out Failed True SnapshotTimedOut
wait_cell tenant-snapshot rollback True
k -n tenant-snapshot delete cellsnapshot timed-out --wait=true

k apply -f - <<EOF
apiVersion: dsh.isolated.io/v1alpha1
kind: CellSnapshot
metadata:
  name: cleanup-blocked
  namespace: tenant-snapshot
spec:
  cellRef:
    name: rollback
  volumeSnapshotClassName: csi-hostpath-snapclass
EOF
wait_snapshot_condition tenant-snapshot cleanup-blocked WriterStopped True
cleanup_uid="$(k -n tenant-snapshot get cellsnapshot cleanup-blocked -o jsonpath='{.metadata.uid}')"
cleanup_volume="cellsnapshot-${cleanup_uid}"
for _ in $(seq 1 60); do
  k -n tenant-snapshot get volumesnapshot "$cleanup_volume" >/dev/null 2>&1 && break
  sleep 1
done
k -n tenant-snapshot patch volumesnapshot "$cleanup_volume" --type=merge \
  -p '{"metadata":{"finalizers":["phase31.dsh.isolated.io/hold-cleanup"]}}'
k -n tenant-snapshot patch volumesnapshot "$cleanup_volume" --subresource=status --type=merge \
  -p '{"status":{"error":{"message":"phase31 injected CSI error"}}}'
wait_snapshot_condition tenant-snapshot cleanup-blocked Failed False CleanupBlocked
wait_cell_condition tenant-snapshot rollback Ready False SnapshotInProgress
test "$(k -n tenant-snapshot get statefulset "$rollback_base" -o jsonpath='{.spec.replicas}')" = "0"
k -n tenant-snapshot patch volumesnapshot "$cleanup_volume" --type=merge -p '{"metadata":{"finalizers":[]}}'
wait_snapshot_condition tenant-snapshot cleanup-blocked Failed True
wait_cell tenant-snapshot rollback True
k -n tenant-snapshot delete cellsnapshot cleanup-blocked --wait=true
k -n kube-system scale deployment snapshot-controller --replicas=2
k -n kube-system rollout status deployment/snapshot-controller --timeout=180s

# Correctable prerequisite errors do not interrupt the source Cell.
k apply -f - <<EOF
apiVersion: snapshot.storage.k8s.io/v1
kind: VolumeSnapshotClass
metadata:
  name: wrong-driver
driver: invalid.example.test/csi
deletionPolicy: Delete
---
apiVersion: dsh.isolated.io/v1alpha1
kind: CellSnapshot
metadata:
  name: missing-class
  namespace: tenant-snapshot
spec:
  cellRef:
    name: rollback
  volumeSnapshotClassName: unavailable-class
---
apiVersion: dsh.isolated.io/v1alpha1
kind: CellSnapshot
metadata:
  name: wrong-driver
  namespace: tenant-snapshot
spec:
  cellRef:
    name: rollback
  volumeSnapshotClassName: wrong-driver
EOF
wait_snapshot_condition tenant-snapshot missing-class Accepted False SnapshotClassUnavailable
wait_snapshot_condition tenant-snapshot wrong-driver Accepted False SnapshotClassDriverMismatch
wait_cell tenant-snapshot rollback True
k -n tenant-snapshot delete cellsnapshot missing-class wrong-driver --wait=true

# An operation UID with no matching CellSnapshot is stale API state, not a
# permanent queue. The Cell reconciler removes only the exact stale marker.
k -n tenant-snapshot annotate cell rollback dsh.isolated.io/active-snapshot-uid=00000000-0000-0000-0000-000000000000 --overwrite
for _ in $(seq 1 60); do
  test -z "$(k -n tenant-snapshot get cell rollback -o jsonpath='{.metadata.annotations.dsh\.isolated\.io/active-snapshot-uid}')" && break
  sleep 1
done
test -z "$(k -n tenant-snapshot get cell rollback -o jsonpath='{.metadata.annotations.dsh\.isolated\.io/active-snapshot-uid}')"
wait_cell tenant-snapshot rollback True

# Cancellation is reconciled: an owned CSI object is removed while the writer
# remains fenced, then the operation lock is released and the source resumes.
k apply -f - <<EOF
apiVersion: dsh.isolated.io/v1alpha1
kind: CellSnapshot
metadata:
  name: cancelled
  namespace: tenant-snapshot
spec:
  cellRef:
    name: rollback
  volumeSnapshotClassName: csi-hostpath-snapclass
EOF
cancelled_uid="$(k -n tenant-snapshot get cellsnapshot cancelled -o jsonpath='{.metadata.uid}')"
wait_snapshot_condition tenant-snapshot cancelled Accepted True
k -n tenant-snapshot delete cellsnapshot cancelled --wait=false
wait_absent tenant-snapshot cellsnapshot cancelled
wait_absent tenant-snapshot volumesnapshot "cellsnapshot-${cancelled_uid}"
wait_cell tenant-snapshot rollback True
cell_a_host="$rollback_host"
cell_a_authority="$rollback_authority"
browser resume alice@example.com

# A same-name VolumeSnapshot without this CellSnapshot's controller owner is
# observable as a terminal ownership failure and is never adopted or deleted.
k -n dsh-system scale deployment cell-operator --replicas=0
k -n dsh-system wait --for=delete pod -l app.kubernetes.io/name=cell-operator --timeout=120s
k apply -f - <<EOF
apiVersion: dsh.isolated.io/v1alpha1
kind: CellSnapshot
metadata:
  name: foreign-collision
  namespace: tenant-snapshot
spec:
  cellRef:
    name: rollback
  volumeSnapshotClassName: csi-hostpath-snapclass
EOF
foreign_snapshot_uid="$(k -n tenant-snapshot get cellsnapshot foreign-collision -o jsonpath='{.metadata.uid}')"
foreign_volume_snapshot="cellsnapshot-${foreign_snapshot_uid}"
k apply -f - <<EOF
apiVersion: snapshot.storage.k8s.io/v1
kind: VolumeSnapshot
metadata:
  name: ${foreign_volume_snapshot}
  namespace: tenant-snapshot
spec:
  volumeSnapshotClassName: csi-hostpath-snapclass
  source:
    persistentVolumeClaimName: ${rollback_base}-data
EOF
k -n dsh-system scale deployment cell-operator --replicas=1
k -n dsh-system rollout status deployment/cell-operator --timeout=180s
wait_snapshot_condition tenant-snapshot foreign-collision Failed True
test -z "$(k -n tenant-snapshot get volumesnapshot "$foreign_volume_snapshot" -o jsonpath='{.metadata.ownerReferences}')"
wait_cell tenant-snapshot rollback True
k -n tenant-snapshot delete cellsnapshot foreign-collision --wait=true
k -n tenant-snapshot delete volumesnapshot "$foreign_volume_snapshot" --wait=true

# Successful snapshots outlive their source Cell.
k -n tenant-snapshot delete cell source --wait=true
k -n tenant-snapshot get cellsnapshot source-backup >/dev/null

# A delete racing a fresh restore cannot remove the immutable dataSource before
# the exact recorded image becomes the first Ready reader. A missing Secret
# keeps that barrier open without changing the public Cell API.
k apply -f - <<EOF
apiVersion: dsh.isolated.io/v1alpha1
kind: Cell
metadata:
  name: deletion-race
  namespace: tenant-snapshot
spec:
  image: ${cell_repo}@${cell_digest}
  credentialsRef:
    name: delayed-race-provider
  storage:
    size: 1Gi
    retentionPolicy: Delete
    restoreFrom:
      name: source-backup
EOF
deletion_race_uid="$(k -n tenant-snapshot get cell deletion-race -o jsonpath='{.metadata.uid}')"
deletion_race_base="cell-${deletion_race_uid}"
for _ in $(seq 1 120); do
  if k -n tenant-snapshot get pvc "${deletion_race_base}-data" >/dev/null 2>&1 &&
     k -n tenant-snapshot get cellsnapshot source-backup -o json | jq -e --arg uid "$deletion_race_uid" '
       any(.metadata.finalizers[]?; . == ("dsh.isolated.io/restore-" + $uid))
     ' >/dev/null; then
    break
  fi
  sleep 1
done
k -n tenant-snapshot get pvc "${deletion_race_base}-data" >/dev/null
k -n tenant-snapshot patch cell deletion-race --type=merge \
  -p "{\"spec\":{\"image\":\"${cell_repo}@${cell_b_digest}\"}}"
wait_cell_condition tenant-snapshot deletion-race StorageReady False RestoreImageMismatch
test "$(k -n tenant-snapshot get statefulset "$deletion_race_base" -o jsonpath='{.spec.template.spec.containers[0].image}')" = "${cell_repo}@${cell_digest}"
k -n tenant-snapshot patch cell deletion-race --type=merge \
  -p "{\"spec\":{\"image\":\"${cell_repo}@${cell_digest}\"}}"
k -n tenant-snapshot delete cellsnapshot source-backup --wait=false
sleep 5
k -n tenant-snapshot get cellsnapshot source-backup >/dev/null
k -n tenant-snapshot get volumesnapshot "$volume_snapshot" >/dev/null
k -n tenant-snapshot create secret generic delayed-race-provider --from-literal=PHASE31_RACE_MARKER=ready
wait_cell tenant-snapshot deletion-race True
wait_absent tenant-snapshot cellsnapshot source-backup
wait_absent tenant-snapshot volumesnapshot "$volume_snapshot"
k -n tenant-snapshot delete cell deletion-race --wait=true

if { k get cells,cellsnapshots -A -o json; k get events -A -o json; \
     k logs -n dsh-system deployment/cell-operator; } | \
  grep -Eq "$phase3_provider_value|[?&]token=[A-Za-z0-9_-]{43}|eyJ[A-Za-z0-9_-]+\.eyJ"; then
  echo "secret or token leaked into Phase 3 status, events or logs" >&2
  exit 1
fi

echo "Phase 3 writer-stopped crash-consistent lifecycle passed"
