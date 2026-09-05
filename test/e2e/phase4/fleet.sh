# shellcheck shell=bash
# shellcheck disable=SC2034,SC2154
# This file is sourced by the Phase 3 lifecycle proof. It deliberately keeps
# the exact Gateway, browser, CSI and candidate-image environment already
# proven there, then adds a bounded multi-namespace load.

fleet_namespaces=10
cells_per_namespace=5
fleet_total_cells=$((fleet_namespaces * cells_per_namespace))
fleet_fixture_local="dsh-phase4-fixture:test"
fleet_fixture_repo="localhost:${registry_port}/dsh-fleet-fixture"
fleet_secret_value="fleet-secret-8c44d1"
runner_cpus="$(nproc)"
runner_memory_bytes="$(awk '/^MemTotal:/ { printf "%.0f", $2 * 1024; exit }' /proc/meminfo)"
runner_disk_bytes="$(df -B1 --output=avail "$repo_root" | awk 'NR == 2 { print $1 }')"

phase4_dump_failure() {
  local status=$?
  trap - ERR
  set +e
  echo "Phase 4 bounded fleet proof failed; aggregate evidence follows" >&2
  k get cells -A -o json | jq '[.items[] | select(.metadata.namespace | startswith("fleet-")) |
    .status.conditions[]? | {type, status, reason}] |
    group_by([.type, .status, .reason]) |
    map({condition: .[0], count: length})' >&2
  k get cellsnapshots -A -o json | jq '[.items[] | select(.metadata.namespace | startswith("fleet-")) |
    .status.conditions[]? | {type, status, reason}] |
    group_by([.type, .status, .reason]) |
    map({condition: .[0], count: length})' >&2
  k get events -A -o json | jq '[.items[] | {type, reason}] |
    group_by([.type, .reason]) | map({event: .[0], count: length})' >&2
  exit "$status"
}
trap phase4_dump_failure ERR

wait_fleet_ready_count() {
  local wanted="$1" timeout_seconds="${2:-720}" start="$SECONDS" ready
  while (( SECONDS - start < timeout_seconds )); do
    ready="$(k get cells -A -o json | jq '[.items[] | select(
      (.metadata.namespace | startswith("fleet-")) and
      any(.status.conditions[]?; .type == "Ready" and .status == "True") and
      .status.observedGeneration == .metadata.generation)] | length')"
    if (( ready >= wanted )); then
      printf '%s\n' "$ready"
      return
    fi
    sleep 2
  done
  echo "fleet reached only ${ready:-0}/${wanted} Ready Cells" >&2
  return 1
}

wait_fleet_snapshot_count() {
  local wanted="$1" timeout_seconds="${2:-720}" start="$SECONDS" ready
  while (( SECONDS - start < timeout_seconds )); do
    ready="$(k get cellsnapshots -A -o json | jq '[.items[] | select(any(.status.conditions[]?; .type == "Ready" and .status == "True"))] | length')"
    if (( ready >= wanted )); then
      printf '%s\n' "$ready"
      return
    fi
    sleep 2
  done
  echo "fleet reached only ${ready:-0}/${wanted} Ready CellSnapshots" >&2
  return 1
}

wait_fleet_policy_failure() {
  local namespace="$1" timeout_seconds="${2:-240}" start="$SECONDS"
  while (( SECONDS - start < timeout_seconds )); do
    if k -n "$namespace" get cells -o json | jq -e '
      any(.items[]; any(.status.conditions[]?;
        .status == "False" and .reason == "ReconcileFailed"))
    ' >/dev/null 2>&1; then
      return
    fi
    sleep 2
  done
  echo "namespace $namespace did not expose a native policy rejection" >&2
  return 1
}

wait_all_routes() {
  local timeout_seconds="${1:-720}" start="$SECONDS" accepted
  while (( SECONDS - start < timeout_seconds )); do
    accepted="$(k get httproutes -A -o json | jq '[.items[] | select(
      (.metadata.namespace | startswith("fleet-")) and
      any(.status.parents[]?.conditions[]?; .type == "Accepted" and .status == "True") and
      any(.status.parents[]?.conditions[]?; .type == "ResolvedRefs" and .status == "True"))] | length')"
    if (( accepted == fleet_total_cells )); then
      return
    fi
    sleep 2
  done
  echo "only ${accepted:-0}/${fleet_total_cells} fleet routes were accepted" >&2
  return 1
}

controller_reconcile_sum() {
  awk '/^controller_runtime_reconcile_total\{/ { total += $NF } END { printf "%.0f", total + 0 }' "$1"
}

sample_operator_rss() {
  local pod node bytes
  while :; do
    pod="$(k -n dsh-system get pods -l app.kubernetes.io/name=cell-operator -o json 2>/dev/null | jq -r '
      [.items[] | select(.metadata.deletionTimestamp == null and .status.phase == "Running")][0].metadata.name // empty
    ' || true)"
    if [[ -n "$pod" ]]; then
      node="$(k -n dsh-system get pod "$pod" -o jsonpath='{.spec.nodeName}' 2>/dev/null || true)"
      if [[ -n "$node" ]]; then
        bytes="$(k get --raw "/api/v1/nodes/${node}/proxy/stats/summary" 2>/dev/null | jq -r --arg pod "$pod" '
          [.pods[]? | select(.podRef.namespace == "dsh-system" and .podRef.name == $pod) | .memory.workingSetBytes][0] // empty
        ' || true)"
        if [[ "$bytes" =~ ^[0-9]+$ ]]; then
          printf '%s\n' "$bytes" >>"$operator_rss_samples"
        fi
      fi
    fi
    sleep 2
  done
}

# Metrics listeners must actually be absent in the inherited default mode, not
# merely omitted from Services and container port declarations.
disabled_operator_pid="$(start_forward dsh-system deployment/cell-operator 18980:8080 "$test_root/operator-metrics-disabled.log" 18980)"
disabled_authorizer_pid="$(start_forward dsh-system deployment/cell-authorizer 18990:9090 "$test_root/authorizer-metrics-disabled.log" 18990)"
if curl -fsS --max-time 2 http://127.0.0.1:18980/metrics >/dev/null 2>&1 ||
   curl -fsS --max-time 2 http://127.0.0.1:18990/metrics >/dev/null 2>&1; then
  echo "metrics endpoint was reachable while disabled" >&2
  exit 1
fi
kill "$disabled_operator_pid" "$disabled_authorizer_pid" >/dev/null 2>&1 || true
wait "$disabled_operator_pid" "$disabled_authorizer_pid" 2>/dev/null || true

# Metrics and worker limits are opt-in. Repeating a flag is intentional: Go's
# flag package applies the final occurrence, so this extension can preserve the
# exact Phase 3 runtime configuration while overriding only Phase 4 knobs.
k -n dsh-system patch deployment cell-operator --type=json -p "$(jq -cn '[
  {op:"add",path:"/spec/template/spec/containers/0/args/-",value:"--metrics-bind-address=:8080"},
  {op:"add",path:"/spec/template/spec/containers/0/args/-",value:"--cell-concurrency=4"},
  {op:"add",path:"/spec/template/spec/containers/0/args/-",value:"--snapshot-concurrency=2"},
  {op:"add",path:"/spec/template/spec/containers/0/args/-",value:"--snapshot-timeout=10m"},
  {op:"add",path:"/spec/template/spec/containers/0/ports/-",value:{name:"metrics",containerPort:8080,protocol:"TCP"}}
]')"
k -n dsh-system patch deployment cell-authorizer --type=json -p "$(jq -cn '[
  {op:"add",path:"/spec/template/spec/containers/0/args/-",value:"--metrics-bind-address=:9090"},
  {op:"add",path:"/spec/template/spec/containers/0/ports/-",value:{name:"metrics",containerPort:9090,protocol:"TCP"}}
]')"
k -n dsh-system rollout status deployment/cell-operator --timeout=180s
k -n dsh-system rollout status deployment/cell-authorizer --timeout=180s

# The operator is cluster-wide only because its namespaced Cells are. Namespace
# policy stays authoritative and the controller has no permission to mirror or
# reinterpret it into a project-owned fleet database.
operator_subject="system:serviceaccount:dsh-system:cell-operator"
for resource in namespaces resourcequotas limitranges flowschemas; do
  test "$(k auth can-i list "$resource" --as="$operator_subject" 2>/dev/null || true)" = "no"
done
test -z "$(k -n dsh-system get services -o json | jq -r '.items[].spec.ports[]? | select(.port == 8080 or .port == 9090) | .name' | grep -E '^metrics$' || true)"

docker buildx build --builder default --platform linux/amd64 --load \
  -f "$repo_root/test/e2e/phase4/Dockerfile.fixture" \
  -t "$fleet_fixture_local" "$repo_root"
docker tag "$fleet_fixture_local" "$fleet_fixture_repo:e2e"
docker push "$fleet_fixture_repo:e2e" >/dev/null
fleet_fixture_digest="$(registry_digest dsh-fleet-fixture e2e)"
operator_rss_samples="$test_root/phase4-operator-rss"
: >"$operator_rss_samples"
sample_operator_rss &
rss_sampler_pid=$!
extension_pids+=("$rss_sampler_pid")

for index in $(seq 0 $((fleet_namespaces - 1))); do
  namespace="$(printf 'fleet-%02d' "$index")"
  k create namespace "$namespace"
  if (( index != 0 )); then
    k label namespace "$namespace" dsh.isolated.io/routes=enabled
  fi
  k -n "$namespace" create secret generic fixture-ok \
    --from-literal="FLEET_SECRET_MARKER=$fleet_secret_value"
done
k -n fleet-07 create secret generic fixture-readiness \
  --from-literal="FLEET_SECRET_MARKER=$fleet_secret_value" \
  --from-literal="FLEET_FIXTURE_READY_URL=http://readiness-gate.fleet-07.svc:8080/"

k apply -f - <<'EOF'
apiVersion: v1
kind: LimitRange
metadata:
  name: container-defaults
  namespace: fleet-01
spec:
  limits:
    - type: Container
      default:
        memory: 64Mi
      defaultRequest:
        cpu: 2m
        memory: 8Mi
---
apiVersion: v1
kind: ResourceQuota
metadata:
  name: pvc-budget
  namespace: fleet-08
spec:
  hard:
    count/persistentvolumeclaims: "8"
---
apiVersion: v1
kind: LimitRange
metadata:
  name: storage-maximum
  namespace: fleet-09
spec:
  limits:
    - type: PersistentVolumeClaim
      max:
        storage: 512Mi
EOF

fleet_apply="$test_root/phase4-cells.yaml"
: >"$fleet_apply"
fleet_start="$SECONDS"
first_fleet_cell=true
for ns_index in $(seq 0 $((fleet_namespaces - 1))); do
  namespace="$(printf 'fleet-%02d' "$ns_index")"
  for cell_index in $(seq 0 $((cells_per_namespace - 1))); do
    image="${fleet_fixture_repo}@${fleet_fixture_digest}"
    credentials=fixture-ok
    is_canary=false
    if (( (ns_index == 0 || ns_index == 1) && cell_index == 0 )); then
      image="${cell_repo}@${cell_digest}"
      is_canary=true
    fi
    if (( ns_index == 7 && cell_index == 4 )); then
      credentials=fixture-readiness
    fi
    {
      if [[ "$first_fleet_cell" = false ]]; then
        echo '---'
      fi
      first_fleet_cell=false
      echo 'apiVersion: dsh.isolated.io/v1alpha1'
      echo 'kind: Cell'
      echo 'metadata:'
      echo "  name: cell-${cell_index}"
      echo "  namespace: ${namespace}"
      echo 'spec:'
      echo "  image: ${image}"
      echo '  credentialsRef:'
      echo "    name: ${credentials}"
      if ! (( ns_index == 1 && cell_index == 1 )); then
        echo '  resources:'
        echo '    requests:'
        if [[ "$is_canary" = true ]]; then
          echo '      cpu: 10m'
          echo '      memory: 64Mi'
        else
          echo '      cpu: 2m'
          echo '      memory: 8Mi'
        fi
        echo '    limits:'
        if [[ "$is_canary" = true ]]; then
          echo '      memory: 512Mi'
        else
          echo '      memory: 64Mi'
        fi
      fi
      echo '  storage:'
      echo '    size: 1Gi'
      echo '    storageClassName: csi-hostpath-sc'
      echo '    retentionPolicy: Delete'
    } >>"$fleet_apply"
  done
done
k apply -f "$fleet_apply"
test "$(k get cells -A -o json | jq '[.items[] | select(.metadata.namespace | startswith("fleet-"))] | length')" = "$fleet_total_cells"

# Core Cell service does not require a public-route capability. A missing
# namespace selector produces a native Gateway rejection without changing the
# four Cell Conditions.
wait_cell fleet-00 cell-0 True
fleet00_uid="$(k -n fleet-00 get cell cell-0 -o jsonpath='{.metadata.uid}')"
wait_route_rejected fleet-00 "cell-${fleet00_uid}"
wait_fleet_policy_failure fleet-08
wait_fleet_policy_failure fleet-09
wait_cell fleet-07 cell-4 False

# LimitRange defaults are admission-owned Pod state. The operator leaves the
# Cell and StatefulSet resource intent untouched and remains stable afterward.
fleet01_uid="$(k -n fleet-01 get cell cell-1 -o jsonpath='{.metadata.uid}')"
fleet01_base="cell-${fleet01_uid}"
test -z "$(k -n fleet-01 get statefulset "$fleet01_base" -o jsonpath='{.spec.template.spec.containers[0].resources.requests.cpu}')"
test "$(k -n fleet-01 get pod -l "dsh.isolated.io/cell-uid=${fleet01_uid}" -o jsonpath='{.items[0].spec.containers[0].resources.requests.cpu}')" = 2m

# Fix only the administrator-owned prerequisites. Kubernetes events, API error
# backoff and object watches must converge without editing or restarting the
# affected Cells.
k label namespace fleet-00 dsh.isolated.io/routes=enabled
k -n fleet-08 patch resourcequota pvc-budget --type=merge \
  -p '{"spec":{"hard":{"count/persistentvolumeclaims":"12"}}}'
k -n fleet-09 patch limitrange storage-maximum --type=merge \
  -p '{"spec":{"limits":[{"type":"PersistentVolumeClaim","max":{"storage":"2Gi"}}]}}'
k -n fleet-07 run readiness-gate --restart=Never \
  --image="${fleet_fixture_repo}@${fleet_fixture_digest}" --labels=app=readiness-gate
k -n fleet-07 expose pod readiness-gate --name=readiness-gate --port=8080 --target-port=8080
k -n fleet-07 wait pod/readiness-gate --for=condition=Ready --timeout=120s
wait_fleet_ready_count "$fleet_total_cells" 720 >/dev/null
wait_all_routes 720

# A burst of mutable Cell updates exercises ordinary StatefulSet rollout under
# the same worker bounds. No project scheduler or per-tenant queue is involved.
for ns_index in $(seq 0 $((fleet_namespaces - 1))); do
  namespace="$(printf 'fleet-%02d' "$ns_index")"
  k -n "$namespace" patch cell cell-2 --type=merge \
    -p '{"spec":{"resources":{"limits":{"memory":"80Mi"}}}}' >/dev/null
done
wait_fleet_ready_count "$fleet_total_cells" 720 >/dev/null
fleet_convergence_seconds=$((SECONDS - fleet_start))
(( fleet_convergence_seconds <= 720 ))

# Create eight overlapping operations while the external CSI controller is
# paused. The project controller may execute at most two reconciles at once;
# per-Cell CAS still queues a ninth operation that targets an active source.
k -n kube-system scale deployment snapshot-controller --replicas=0
k -n kube-system wait --for=delete pod -l app=snapshot-controller --timeout=120s
snapshot_start="$SECONDS"
primary_snapshots=()
for ns_index in 2 3 4 5; do
  namespace="$(printf 'fleet-%02d' "$ns_index")"
  for cell_index in 0 1; do
    snapshot_name="fleet-backup-${cell_index}"
    primary_snapshots+=("${namespace}/${snapshot_name}")
    k apply -f - <<EOF
apiVersion: dsh.isolated.io/v1alpha1
kind: CellSnapshot
metadata:
  name: ${snapshot_name}
  namespace: ${namespace}
spec:
  cellRef:
    name: cell-${cell_index}
  volumeSnapshotClassName: csi-hostpath-snapclass
EOF
  done
done
wait_snapshot_condition fleet-02 fleet-backup-0 Accepted True
k apply -f - <<'EOF'
apiVersion: dsh.isolated.io/v1alpha1
kind: CellSnapshot
metadata:
  name: fleet-backup-queued
  namespace: fleet-02
spec:
  cellRef:
    name: cell-0
  volumeSnapshotClassName: csi-hostpath-snapclass
EOF
wait_snapshot_condition fleet-02 fleet-backup-queued Accepted False OperationQueued

for item in "${primary_snapshots[@]}"; do
  namespace="${item%%/*}"
  snapshot_name="${item##*/}"
  wait_snapshot_condition "$namespace" "$snapshot_name" WriterStopped True
done
test "$(k get volumesnapshots -A -o json | jq '[.items[] | select(.metadata.namespace | startswith("fleet-"))] | length')" = 8

# A controller restart loses no operation state. CSI then completes the eight
# active snapshots; the queued ninth operation acquires the released Cell lock
# and completes afterward.
k -n dsh-system scale deployment cell-operator --replicas=0
k -n dsh-system wait --for=delete pod -l app.kubernetes.io/name=cell-operator --timeout=120s
k -n dsh-system scale deployment cell-operator --replicas=1
k -n dsh-system rollout status deployment/cell-operator --timeout=180s
k -n kube-system scale deployment snapshot-controller --replicas=2
k -n kube-system rollout status deployment/snapshot-controller --timeout=180s
wait_fleet_snapshot_count 9 720 >/dev/null
for ns_index in 2 3 4 5; do
  namespace="$(printf 'fleet-%02d' "$ns_index")"
  wait_cell "$namespace" cell-0 True
  wait_cell "$namespace" cell-1 True
done
snapshot_convergence_seconds=$((SECONDS - snapshot_start))
(( snapshot_convergence_seconds <= 720 ))

# Cluster-scoped storage and snapshot classes have no safe one-object watch
# mapping. Their documented slow fallback must recover a pending request after
# the administrator creates the missing native prerequisite.
k apply -f - <<EOF
apiVersion: dsh.isolated.io/v1alpha1
kind: Cell
metadata:
  name: late-storage
  namespace: fleet-07
spec:
  image: ${fleet_fixture_repo}@${fleet_fixture_digest}
  credentialsRef:
    name: fixture-ok
  resources:
    requests:
      cpu: 2m
      memory: 8Mi
    limits:
      memory: 64Mi
  storage:
    size: 1Gi
    storageClassName: phase4-late-sc
    retentionPolicy: Delete
EOF
wait_cell_condition fleet-07 late-storage StorageReady False PVCsPending
k apply -f - <<'EOF'
apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  name: phase4-late-sc
provisioner: hostpath.csi.k8s.io
reclaimPolicy: Delete
volumeBindingMode: Immediate
allowVolumeExpansion: true
EOF
wait_cell fleet-07 late-storage True
k -n fleet-07 delete cell late-storage --wait=true
k delete storageclass phase4-late-sc

k apply -f - <<'EOF'
apiVersion: dsh.isolated.io/v1alpha1
kind: CellSnapshot
metadata:
  name: late-snapshot-class
  namespace: fleet-02
spec:
  cellRef:
    name: cell-2
  volumeSnapshotClassName: phase4-late-snapclass
EOF
wait_snapshot_condition fleet-02 late-snapshot-class Accepted False SnapshotClassUnavailable
k apply -f - <<'EOF'
apiVersion: snapshot.storage.k8s.io/v1
kind: VolumeSnapshotClass
metadata:
  name: phase4-late-snapclass
driver: hostpath.csi.k8s.io
deletionPolicy: Delete
EOF
wait_snapshot_condition fleet-02 late-snapshot-class Ready True
k -n fleet-02 delete cellsnapshot late-snapshot-class --wait=true
k delete volumesnapshotclass phase4-late-snapclass

# Namespace deletion remains a native garbage-collection boundary. Recreating
# the same namespace and Cell name yields new API identities and converges from
# observed Kubernetes state, not from an operator inventory.
old_namespace_uid="$(k get namespace fleet-06 -o jsonpath='{.metadata.uid}')"
old_cell_uid="$(k -n fleet-06 get cell cell-0 -o jsonpath='{.metadata.uid}')"
k delete namespace fleet-06 --wait=true --timeout=300s
k create namespace fleet-06
k label namespace fleet-06 dsh.isolated.io/routes=enabled
k -n fleet-06 create secret generic fixture-ok \
  --from-literal="FLEET_SECRET_MARKER=$fleet_secret_value"
k apply -f - <<EOF
apiVersion: dsh.isolated.io/v1alpha1
kind: Cell
metadata:
  name: cell-0
  namespace: fleet-06
spec:
  image: ${fleet_fixture_repo}@${fleet_fixture_digest}
  credentialsRef:
    name: fixture-ok
  resources:
    requests:
      cpu: 2m
      memory: 8Mi
    limits:
      memory: 64Mi
  storage:
    size: 1Gi
    storageClassName: csi-hostpath-sc
    retentionPolicy: Delete
EOF
wait_cell fleet-06 cell-0 True
test "$(k get namespace fleet-06 -o jsonpath='{.metadata.uid}')" != "$old_namespace_uid"
new_cell_uid="$(k -n fleet-06 get cell cell-0 -o jsonpath='{.metadata.uid}')"
test "$new_cell_uid" != "$old_cell_uid"
wait_route fleet-06 "cell-${new_cell_uid}"

# Generate one allowed and one denied request after the authorizer registry was
# created. OIDC cookies may be reused, but the uncached SAR remains per request.
cell_a_host="$rollback_host"
cell_a_authority="$rollback_authority"
denied_uid="$(k -n fleet-02 get cell cell-2 -o jsonpath='{.metadata.uid}')"
cell_b_host="cell-${denied_uid}.cells.test"
cell_b_authority="${cell_b_host}:18443"
browser_eventually resume alice@example.com
browser_eventually deny alice@example.com

operator_metrics_log="$test_root/phase4-operator-metrics.log"
authorizer_metrics_log="$test_root/phase4-authorizer-metrics.log"
operator_forward_pid="$(start_forward dsh-system deployment/cell-operator 19080:8080 "$test_root/operator-metrics-forward.log" 19080)"
authorizer_forward_pid="$(start_forward dsh-system deployment/cell-authorizer 19090:9090 "$test_root/authorizer-metrics-forward.log" 19090)"
extension_pids+=("$operator_forward_pid" "$authorizer_forward_pid")
curl -fsS http://127.0.0.1:19080/metrics >"$operator_metrics_log"
curl -fsS http://127.0.0.1:19090/metrics >"$authorizer_metrics_log"
grep -Eq '^controller_runtime_max_concurrent_reconciles\{controller="cell"\} 4$' "$operator_metrics_log"
grep -Eq '^controller_runtime_max_concurrent_reconciles\{controller="cellsnapshot"\} 2$' "$operator_metrics_log"
grep -Eq '^dsh_authorizer_decisions_total\{decision="allow"\} [1-9][0-9]*(\.[0-9]+)?$' "$authorizer_metrics_log"
grep -Eq '^dsh_authorizer_decisions_total\{decision="denied"\} [1-9][0-9]*(\.[0-9]+)?$' "$authorizer_metrics_log"
while read -r decision; do
  case "$decision" in
    allow|unauthenticated|denied|route_mismatch|dependency_error) ;;
    *) echo "unbounded authorizer decision label: $decision" >&2; exit 1 ;;
  esac
done < <(sed -n 's/^dsh_authorizer_decisions_total{decision="\([^"]*\)"}.*/\1/p' "$authorizer_metrics_log")

# Custom counters are process-local by design. Restarting the authorizer resets
# them without changing authorization, then new requests repopulate the same
# closed label set.
kill "$authorizer_forward_pid" >/dev/null 2>&1 || true
wait "$authorizer_forward_pid" 2>/dev/null || true
k -n dsh-system rollout restart deployment/cell-authorizer
k -n dsh-system rollout status deployment/cell-authorizer --timeout=180s
authorizer_forward_pid="$(start_forward dsh-system deployment/cell-authorizer 19190:9090 "$test_root/authorizer-metrics-reset-forward.log" 19190)"
extension_pids+=("$authorizer_forward_pid")
curl -fsS http://127.0.0.1:19190/metrics >"$authorizer_metrics_log"
test "$(awk '/^dsh_authorizer_decisions_total/ { total += $NF } END { print total + 0 }' "$authorizer_metrics_log")" = 0
browser_eventually resume alice@example.com
browser_eventually deny alice@example.com
curl -fsS http://127.0.0.1:19190/metrics >"$authorizer_metrics_log"
grep -Eq '^dsh_authorizer_decisions_total\{decision="allow"\} [1-9][0-9]*(\.[0-9]+)?$' "$authorizer_metrics_log"
grep -Eq '^dsh_authorizer_decisions_total\{decision="denied"\} [1-9][0-9]*(\.[0-9]+)?$' "$authorizer_metrics_log"

if grep -Fq "$fleet_secret_value" "$operator_metrics_log" "$authorizer_metrics_log" ||
   grep -Eq 'fleet-[0-9]{2}|cell-[0-9a-f-]{36}|cells\.test|tenant-' "$operator_metrics_log" "$authorizer_metrics_log"; then
  echo "identity, topology, or secret leaked into Phase 4 metrics" >&2
  exit 1
fi

# After all API-visible work is complete, a quiet cluster must not generate
# reconciles solely to discover that it is still quiet.
steady_before="$(controller_reconcile_sum "$operator_metrics_log")"
sleep 20
curl -fsS http://127.0.0.1:19080/metrics >"$operator_metrics_log"
steady_after="$(controller_reconcile_sum "$operator_metrics_log")"
test "$steady_after" = "$steady_before"

# No Cell ever has more than one writer Pod, including during snapshot restart
# and namespace churn. This is checked from the authoritative API objects, not
# from a project inventory metric.
k get pods -A -o json | jq -e '
  [.items[] | select(.metadata.labels["dsh.isolated.io/cell-uid"] != null)] |
  group_by(.metadata.labels["dsh.isolated.io/cell-uid"]) |
  all(.[]; length <= 1)
' >/dev/null

kill "$rss_sampler_pid" >/dev/null 2>&1 || true
wait "$rss_sampler_pid" 2>/dev/null || true
operator_peak_rss="$(awk '$1 > maximum { maximum = $1 } END { print maximum }' "$operator_rss_samples")"
[[ "$operator_peak_rss" =~ ^[0-9]+$ ]]
operator_retries="$(awk '/^workqueue_retries_total\{/ { total += $NF } END { print total + 0 }' "$operator_metrics_log")"
awk -v retries="$operator_retries" 'BEGIN { exit !(retries > 0) }'

# Status stays a fixed, topology-free API surface at scale.
k get cells -A -o json | jq -e '
  all(.items[] | select(.metadata.namespace | startswith("fleet-"));
    (.status | keys | sort) == (["conditions","dshVersion","imageDigest","observedGeneration"] | sort))
' >/dev/null

printf 'Phase 4 fleet proof passed: runner=%s CPUs/%s memory bytes/%s available disk bytes, %d namespaces, %d initial Cells, %d overlapping snapshots, cell convergence=%ss, snapshot convergence=%ss, operator peak working set=%s bytes, workqueue retries=%s\n' \
  "$runner_cpus" "$runner_memory_bytes" "$runner_disk_bytes" "$fleet_namespaces" "$fleet_total_cells" 8 \
  "$fleet_convergence_seconds" "$snapshot_convergence_seconds" "$operator_peak_rss" "$operator_retries"
