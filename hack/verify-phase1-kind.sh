#!/usr/bin/env bash
# Prove the Single-Cell vertical slice against real Kubernetes controllers,
# storage, EndpointSlices, image pulls and NetworkPolicy enforcement.
set -euo pipefail

repo_root="$(cd "$(dirname "$0")/.." && pwd)"
cluster_name="dsh-phase1-${RANDOM}"
registry_name="dsh-phase1-registry-${RANDOM}"
registry_port="$((30000 + RANDOM % 10000))"
registry_manifest_accept="application/vnd.oci.image.index.v1+json, application/vnd.oci.image.manifest.v1+json, application/vnd.docker.distribution.manifest.list.v2+json, application/vnd.docker.distribution.manifest.v2+json"
kubeconfig="$(mktemp)"
kind_config="$(mktemp)"
test_root="$(mktemp -d)"
port_forward_pid=""

k() {
  kubectl --kubeconfig "$kubeconfig" "$@"
}

registry_digest() {
  local repository="$1" tag="$2" digest
  digest="$(curl -fsSI \
    -H "Accept: ${registry_manifest_accept}" \
    "http://127.0.0.1:${registry_port}/v2/${repository}/manifests/${tag}" |
    awk 'tolower($1) == "docker-content-digest:" { gsub(/\r/, "", $2); print $2; exit }')"
  [[ "$digest" =~ ^sha256:[a-f0-9]{64}$ ]]
  printf '%s\n' "$digest"
}

expect_failure() {
  if "$@"; then
    echo "command unexpectedly succeeded: $*" >&2
    return 1
  fi
}

dump_failure() {
  echo "Phase 1 kind verification failed; redacted cluster evidence follows" >&2
  k get cells -A -o wide >&2 || true
  k get pvc,statefulset,service,networkpolicy,pod -A >&2 || true
  k -n dsh-system logs deployment/cell-operator --tail=200 >&2 || true
}

cleanup() {
  if [[ -n "$port_forward_pid" ]]; then
    kill "$port_forward_pid" >/dev/null 2>&1 || true
  fi
  kind delete cluster --name "$cluster_name" >/dev/null 2>&1 || true
  docker rm -f "$registry_name" >/dev/null 2>&1 || true
  rm -f "$kubeconfig" "$kind_config"
  rm -rf "$test_root"
}
trap 'dump_failure' ERR
trap cleanup EXIT

cat >"$kind_config" <<EOF
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
networking:
  disableDefaultCNI: true
  podSubnet: 192.168.0.0/16
containerdConfigPatches:
  - |-
    [plugins."io.containerd.grpc.v1.cri".registry.mirrors."localhost:${registry_port}"]
      endpoint = ["http://${registry_name}:5000"]
nodes:
  - role: control-plane
EOF

docker run --detach --restart=always --name "$registry_name" \
  --publish "127.0.0.1:${registry_port}:5000" \
  registry:2@sha256:a3d8aaa63ed8681a604f1dea0aa03f100d5895b6a58ace528858a7b332415373 >/dev/null
kind create cluster --name "$cluster_name" --kubeconfig "$kubeconfig" \
  --image kindest/node:v1.34.0@sha256:7416a61b42b1662ca6ca89f02028ac133a309a2a30ba309614e8ec94d976dc5a \
  --config "$kind_config" --wait 60s
docker network connect kind "$registry_name"

for calico_image in \
  quay.io/calico/cni:v3.32.2 \
  quay.io/calico/node:v3.32.2 \
  quay.io/calico/kube-controllers:v3.32.2; do
  if docker image inspect "$calico_image" >/dev/null 2>&1; then
    kind load docker-image --name "$cluster_name" "$calico_image"
  fi
done
k apply -f https://raw.githubusercontent.com/projectcalico/calico/v3.32.2/manifests/calico.yaml
k -n kube-system rollout status daemonset/calico-node --timeout=180s
k -n kube-system rollout status deployment/coredns --timeout=180s
k wait node --all --for=condition=Ready --timeout=180s

revision="$(git -C "$repo_root" rev-parse HEAD)"
local_cell="${CELL_IMAGE:-dsh-phase1-cell:test}"
local_operator="${OPERATOR_IMAGE:-dsh-phase1-operator:test}"
if [[ "${SKIP_IMAGE_BUILD:-0}" != "1" ]]; then
  docker buildx build --platform linux/amd64 --load --build-arg "SOURCE_REVISION=$revision" \
    -f "$repo_root/images/operator/Dockerfile" -t "$local_operator" "$repo_root"
  docker buildx build --platform linux/amd64 --load --build-arg "SOURCE_REVISION=$revision" \
    -f "$repo_root/images/cell/Dockerfile" -t "$local_cell" "$repo_root"
fi

cell_repo="localhost:${registry_port}/dsh-cell"
operator_repo="localhost:${registry_port}/dsh-operator"
docker tag "$local_cell" "$cell_repo:e2e"
docker tag "$local_operator" "$operator_repo:e2e"
docker push "$cell_repo:e2e" >/dev/null
docker push "$operator_repo:e2e" >/dev/null
cell_digest="$(registry_digest dsh-cell e2e)"
operator_digest="$(registry_digest dsh-operator e2e)"
curl -fsSI -H "Accept: ${registry_manifest_accept}" \
  "http://127.0.0.1:${registry_port}/v2/dsh-cell/manifests/${cell_digest}" >/dev/null
curl -fsSI -H "Accept: ${registry_manifest_accept}" \
  "http://127.0.0.1:${registry_port}/v2/dsh-operator/manifests/${operator_digest}" >/dev/null
test "$(docker image inspect "$local_cell" --format '{{.Id}}')" = \
  "$(docker image inspect "$cell_repo:e2e" --format '{{.Id}}')"
test "$(docker image inspect "$local_operator" --format '{{.Id}}')" = \
  "$(docker image inspect "$operator_repo:e2e" --format '{{.Id}}')"

k apply -f "$repo_root/config/crd/bases/dsh.isolated.io_cells.yaml"
k wait --for=condition=Established crd/cells.dsh.isolated.io --timeout=60s
k create namespace collision
k apply -f - <<EOF
apiVersion: dsh.isolated.io/v1alpha1
kind: Cell
metadata:
  name: collision
  namespace: collision
spec:
  image: ${cell_repo}@${cell_digest}
  storage:
    size: 1Gi
    retentionPolicy: Delete
EOF
collision_uid="$(k -n collision get cell collision -o jsonpath='{.metadata.uid}')"
collision_base="cell-${collision_uid}"
k -n collision create service clusterip "$collision_base" --tcp=80:8080

k apply -k "$repo_root/config/default"
k -n dsh-system set image deployment/cell-operator "manager=$operator_repo:e2e"
k -n dsh-system rollout status deployment/cell-operator --timeout=180s

wait_cell() {
  local namespace="$1" name="$2" wanted="$3" reason="${4:-}" json
  for _ in $(seq 1 180); do
    json="$(k -n "$namespace" get cell "$name" -o json 2>/dev/null || true)"
    if [[ -n "$json" ]] && jq -e --arg wanted "$wanted" --arg reason "$reason" '
      .status.observedGeneration == .metadata.generation and
      any(.status.conditions[]?;
        .type == "Ready" and .status == $wanted and ($reason == "" or .reason == $reason))
    ' <<<"$json" >/dev/null; then
      return
    fi
    sleep 1
  done
  echo "Cell $namespace/$name did not reach Ready=$wanted reason=$reason" >&2
  return 1
}

wait_cell collision collision False
test "$(k -n collision get cell collision -o json | jq -r '.status.conditions[] | select(.type == "AccessReady") | .reason')" = "OwnershipConflict"
k -n collision delete service "$collision_base"
wait_cell collision collision True
k -n collision delete cell collision --wait=true
for _ in $(seq 1 60); do
  if ! k -n collision get pvc "${collision_base}-data" >/dev/null 2>&1 && \
     ! k -n collision get pvc "${collision_base}-private" >/dev/null 2>&1; then
    break
  fi
  sleep 1
done
if k -n collision get pvc "${collision_base}-data" >/dev/null 2>&1 || \
   k -n collision get pvc "${collision_base}-private" >/dev/null 2>&1; then
  echo "Delete retention left a collision PVC behind" >&2
  exit 1
fi

k patch storageclass standard --type=merge -p '{"allowVolumeExpansion":true}'
k create namespace tenant-a
k -n tenant-a create secret generic provider-one \
  --from-literal=PHASE1_PROVIDER_MARKER=phase1-provider-one-7f9c
k -n tenant-a create secret generic provider-two \
  --from-literal=PHASE1_PROVIDER_MARKER=phase1-provider-two-8a0d
k apply -f - <<EOF
apiVersion: dsh.isolated.io/v1alpha1
kind: Cell
metadata:
  name: main
  namespace: tenant-a
spec:
  image: ${cell_repo}@${cell_digest}
  credentialsRef:
    name: provider-one
  storage:
    size: 1Gi
    retentionPolicy: Retain
EOF
wait_cell tenant-a main True
main_uid="$(k -n tenant-a get cell main -o jsonpath='{.metadata.uid}')"
main_base="cell-${main_uid}"
main_authority="${main_base}.tenant-a.svc"

test "$(k -n tenant-a get statefulset "$main_base" -o jsonpath='{.spec.replicas}')" = "1"
test "$(k -n tenant-a get statefulset "$main_base" -o jsonpath='{.spec.serviceName}')" = "${main_base}-headless"
test "$(k -n tenant-a get service "${main_base}-headless" -o jsonpath='{.spec.clusterIP}')" = "None"
test "$(k -n tenant-a get service "$main_base" -o jsonpath='{.spec.ports[0].port}')" = "80"
test "$(k -n tenant-a get serviceaccount "$main_base" -o jsonpath='{.automountServiceAccountToken}')" = "false"
test "$(k -n tenant-a get networkpolicy "$main_base" -o jsonpath='{.spec.policyTypes[0]}')" = "Ingress"
test -z "$(k -n tenant-a get networkpolicy "$main_base" -o jsonpath='{.spec.egress}')"
k -n tenant-a get cell main -o json | jq -e '
  (.status | keys | sort) == (["conditions","dshVersion","imageDigest","observedGeneration"] | sort) and
  .status.dshVersion == "0.1.3-alpha.1" and
  .status.imageDigest == "'"$cell_digest"'"
' >/dev/null

start_port_forward() {
  if [[ -n "$port_forward_pid" ]]; then
    kill "$port_forward_pid" >/dev/null 2>&1 || true
  fi
  proxy_port="$((20000 + RANDOM % 10000))"
  k -n tenant-a port-forward "service/$main_base" "${proxy_port}:80" \
    >"$test_root/port-forward.log" 2>&1 &
  port_forward_pid="$!"
  for _ in $(seq 1 30); do
    if bash -c "</dev/tcp/127.0.0.1/$proxy_port" >/dev/null 2>&1; then
      return
    fi
    sleep 1
  done
  cat "$test_root/port-forward.log" >&2
  return 1
}

start_port_forward
go run "$repo_root/test/e2e/dshprobe" --connect "127.0.0.1:$proxy_port" \
  --authority "$main_authority" --state-file "$test_root/probe-state.json"
main_pod="$(k -n tenant-a get pod -l "dsh.isolated.io/cell-uid=$main_uid" -o jsonpath='{.items[0].metadata.name}')"
k -n tenant-a exec "$main_pod" -- node -e \
  "require('node:fs').writeFileSync('/var/lib/dsh/data/workspace/phase1-marker', 'durable')"
k -n tenant-a exec "$main_pod" -- test -f /var/lib/dsh-private/.credentials.yaml
old_pod_uid="$(k -n tenant-a get pod "$main_pod" -o jsonpath='{.metadata.uid}')"
k -n tenant-a delete pod "$main_pod" --wait=true
k -n tenant-a rollout status statefulset "$main_base" --timeout=180s
new_pod="$(k -n tenant-a get pod -l "dsh.isolated.io/cell-uid=$main_uid" -o jsonpath='{.items[0].metadata.name}')"
test "$(k -n tenant-a get pod "$new_pod" -o jsonpath='{.metadata.uid}')" != "$old_pod_uid"
start_port_forward
go run "$repo_root/test/e2e/dshprobe" --connect "127.0.0.1:$proxy_port" \
  --authority "$main_authority" --state-file "$test_root/probe-state.json" --resume
k -n tenant-a exec "$new_pod" -- test -f /var/lib/dsh/data/workspace/phase1-marker

k -n tenant-a patch cell main --type=merge -p '{"spec":{"storage":{"size":"2Gi"}}}'
for _ in $(seq 1 60); do
  test "$(k -n tenant-a get pvc "${main_base}-data" -o jsonpath='{.spec.resources.requests.storage}')" = "2Gi" && break
  sleep 1
done
test "$(k -n tenant-a get pvc "${main_base}-data" -o jsonpath='{.spec.resources.requests.storage}')" = "2Gi"
expect_failure k -n tenant-a patch cell main --type=merge -p '{"spec":{"storage":{"size":"1Gi"}}}'
expect_failure k -n tenant-a patch cell main --type=merge -p '{"spec":{"storage":{"storageClassName":"other"}}}'
expect_failure k -n tenant-a patch cell main --type=merge -p '{"spec":{"storage":{"retentionPolicy":"Delete"}}}'

old_revision="$(k -n tenant-a get statefulset "$main_base" -o jsonpath='{.status.currentRevision}')"
k -n tenant-a patch cell main --type=merge -p '{"spec":{"credentialsRef":{"name":"provider-two"}}}'
k -n tenant-a rollout status statefulset "$main_base" --timeout=180s
new_revision="$(k -n tenant-a get statefulset "$main_base" -o jsonpath='{.status.currentRevision}')"
test "$new_revision" != "$old_revision"
main_pod="$(k -n tenant-a get pod -l "dsh.isolated.io/cell-uid=$main_uid" -o jsonpath='{.items[0].metadata.name}')"
# shellcheck disable=SC2016 # Expansion belongs to the shell inside the Pod.
k -n tenant-a exec "$main_pod" -- sh -c 'test "$PHASE1_PROVIDER_MARKER" = phase1-provider-two-8a0d'
k -n tenant-a create secret generic provider-two --dry-run=client -o yaml \
  --from-literal=PHASE1_PROVIDER_MARKER=phase1-provider-three-91be | k apply -f -
# shellcheck disable=SC2016 # Expansion belongs to the shell inside the Pod.
k -n tenant-a exec "$main_pod" -- sh -c 'test "$PHASE1_PROVIDER_MARKER" = phase1-provider-two-8a0d'
k -n tenant-a delete pod "$main_pod" --wait=true
k -n tenant-a rollout status statefulset "$main_base" --timeout=180s
main_pod="$(k -n tenant-a get pod -l "dsh.isolated.io/cell-uid=$main_uid" -o jsonpath='{.items[0].metadata.name}')"
# shellcheck disable=SC2016 # Expansion belongs to the shell inside the Pod.
k -n tenant-a exec "$main_pod" -- sh -c 'test "$PHASE1_PROVIDER_MARKER" = phase1-provider-three-91be'

k -n tenant-a patch cell main --type=merge -p '{"spec":{"credentialsRef":{"name":"provider-missing"}}}'
wait_cell tenant-a main False
k -n tenant-a create secret generic provider-missing --from-literal=PHASE1_PROVIDER_MARKER=recovered
wait_cell tenant-a main True

k -n tenant-a patch cell main --type=merge -p '{"spec":{"image":"'"${operator_repo}@${operator_digest}"'"}}'
wait_cell tenant-a main False
k -n tenant-a patch cell main --type=merge -p '{"spec":{"image":"'"${cell_repo}@${cell_digest}"'"}}'
wait_cell tenant-a main True
main_pod="$(k -n tenant-a get pod -l "dsh.isolated.io/cell-uid=$main_uid" -o jsonpath='{.items[0].metadata.name}')"
k -n tenant-a exec "$main_pod" -- sh -c 'kill -TERM 1'
for _ in $(seq 1 90); do
  restart_count="$(k -n tenant-a get pod "$main_pod" -o jsonpath='{.status.containerStatuses[0].restartCount}' 2>/dev/null || echo 0)"
  [[ "${restart_count:-0}" -ge 1 ]] && break
  sleep 1
done
test "${restart_count:-0}" -ge 1
wait_cell tenant-a main True

k create namespace dsh-denied
k -n dsh-system run access-client --image="${cell_repo}@${cell_digest}" \
  --labels="dsh.isolated.io/access=true" --command -- node -e 'setInterval(()=>{}, 60000)'
k -n dsh-system run denied-client --image="${cell_repo}@${cell_digest}" \
  --command -- node -e 'setInterval(()=>{}, 60000)'
k -n dsh-denied run cross-client --image="${cell_repo}@${cell_digest}" \
  --command -- node -e 'setInterval(()=>{}, 60000)'
k -n dsh-system wait pod/access-client pod/denied-client --for=condition=Ready --timeout=120s
k -n dsh-denied wait pod/cross-client --for=condition=Ready --timeout=120s
http_probe="const h=require('http');const r=h.get({host:'${main_base}.tenant-a.svc',port:80,path:'/',headers:{Host:'${main_authority}'}},x=>process.exit(x.statusCode===303?0:1));r.on('error',()=>process.exit(2));setTimeout(()=>process.exit(3),3000)"
k -n dsh-system exec access-client -- node -e "$http_probe"
expect_failure k -n dsh-system exec denied-client -- node -e "$http_probe"
expect_failure k -n dsh-denied exec cross-client -- node -e "$http_probe"
management_probe="const n=require('net').connect(8081,'${main_base}-0.${main_base}-headless.tenant-a.svc',()=>process.exit(0));n.on('error',()=>process.exit(2));setTimeout(()=>process.exit(3),3000)"
expect_failure k -n dsh-system exec access-client -- node -e "$management_probe"
egress_probe="const n=require('net').connect(443,'kubernetes.default.svc',()=>process.exit(0));n.on('error',()=>process.exit(2));setTimeout(()=>process.exit(3),3000)"
k -n tenant-a exec "$main_pod" -- node -e "$egress_probe"

k create namespace sandbox-test
k apply -f - <<EOF
apiVersion: dsh.isolated.io/v1alpha1
kind: Cell
metadata:
  name: sandbox
  namespace: sandbox-test
spec:
  image: ${cell_repo}@${cell_digest}
  securityClass: sandboxed
  storage:
    size: 1Gi
    retentionPolicy: Delete
EOF
wait_cell sandbox-test sandbox False
test "$(k -n sandbox-test get cell sandbox -o json | jq -r '.status.conditions[] | select(.type == "WorkloadReady") | .reason')" = "SandboxRuntimeClassUnconfigured"
k -n sandbox-test patch cell sandbox --type=merge -p '{"spec":{"securityClass":"standard"}}'
wait_cell sandbox-test sandbox True
k -n sandbox-test delete cell sandbox --wait=true

provisioner="$(k get storageclass standard -o jsonpath='{.provisioner}')"
k create namespace late-storage
k apply -f - <<EOF
apiVersion: dsh.isolated.io/v1alpha1
kind: Cell
metadata:
  name: late
  namespace: late-storage
spec:
  image: ${cell_repo}@${cell_digest}
  storage:
    size: 1Gi
    storageClassName: late-storage
    retentionPolicy: Delete
EOF
wait_cell late-storage late False
k apply -f - <<EOF
apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  name: late-storage
provisioner: ${provisioner}
allowVolumeExpansion: true
volumeBindingMode: WaitForFirstConsumer
reclaimPolicy: Delete
EOF
wait_cell late-storage late True
k -n late-storage delete cell late --wait=true

k -n tenant-a logs "$main_pod" 2>&1 | grep -Eq '[?&]token=[A-Za-z0-9_-]{43}' && {
  echo "launch token leaked into Pod logs" >&2
  exit 1
}
if k -n tenant-a logs "$main_pod" 2>&1 | grep -F 'phase1-provider-three-91be'; then
  echo "provider value leaked into Cell logs" >&2
  exit 1
fi
if docker history --no-trunc "$local_cell" | grep -F 'phase1-provider'; then
  echo "provider value leaked into Cell image history" >&2
  exit 1
fi

retained_data="${main_base}-data"
k -n tenant-a delete cell main --wait=true
for _ in $(seq 1 90); do
  if ! k -n tenant-a get pvc "${main_base}-private" >/dev/null 2>&1; then
    break
  fi
  sleep 1
done
k -n tenant-a get pvc "$retained_data" >/dev/null
if k -n tenant-a get pvc "${main_base}-private" >/dev/null 2>&1; then
  echo "private PVC survived Cell deletion" >&2
  exit 1
fi
k apply -f - <<EOF
apiVersion: v1
kind: Pod
metadata:
  name: retained-inspector
  namespace: tenant-a
spec:
  restartPolicy: Never
  containers:
    - name: inspect
      image: ${cell_repo}@${cell_digest}
      command: ["sh", "-c"]
      args:
        - >-
          test -f /data/workspace/phase1-marker &&
          test ! -e /data/home/.credentials.yaml &&
          ! grep -R -F phase1-provider-three-91be /data
      volumeMounts:
        - name: data
          mountPath: /data
  volumes:
    - name: data
      persistentVolumeClaim:
        claimName: ${retained_data}
EOF
k -n tenant-a wait pod/retained-inspector --for=jsonpath='{.status.phase}'=Succeeded --timeout=120s

echo "Phase 1 kind vertical slice passed"
