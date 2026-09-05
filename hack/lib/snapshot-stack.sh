# shellcheck shell=bash
# shellcheck disable=SC2016,SC2034,SC2154
# This file is sourced by hack/verify-phase2-kind.sh after its complete browser
# proof. It deliberately reuses that cluster, registry, Gateway, Dex and
# Chromium state while adding only the CSI data-lifecycle fixture.

snapshotter_commit="5aab051d1af135e2c852f6fb7fc27fa709d877bf"
hostpath_commit="cc78ee78ae23908c9e0607df2fe09c7ecfa52597"
snapshotter_root="$test_root/external-snapshotter"
hostpath_root="$test_root/csi-driver-host-path"

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
  local url="$1" commit="$2" destination="$3" attempt
  git init -q "$destination"
  if git -C "$destination" remote get-url origin >/dev/null 2>&1; then
    git -C "$destination" remote set-url origin "$url"
  else
    git -C "$destination" remote add origin "$url"
  fi
  for attempt in $(seq 1 5); do
    if git -C "$destination" -c http.version=HTTP/1.1 fetch -q --depth=1 origin "$commit"; then
      break
    fi
    if (( attempt == 5 )); then
      echo "failed to fetch exact commit $commit after $attempt attempts" >&2
      return 1
    fi
    sleep $((attempt * 2))
  done
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
for image in "${csi_images[@]}"; do
  pull_image "$image"
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
