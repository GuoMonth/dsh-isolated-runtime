#!/usr/bin/env bash
# Prove trusted browser access through real Envoy Gateway, Dex, Kubernetes RBAC,
# Calico NetworkPolicy and the production Cell image.
set -Eeuo pipefail

repo_root="$(cd "$(dirname "$0")/.." && pwd)"
cluster_name="dsh-phase2-${RANDOM}"
registry_name="dsh-phase2-registry-${RANDOM}"
registry_port="$((30000 + RANDOM % 10000))"
kubeconfig="$(mktemp)"
kind_config="$(mktemp)"
test_root="$(mktemp -d)"
gateway_forward_pid=""
dex_forward_pid=""
envoy_gateway_version="v1.9.1"
envoy_gateway_sha256="72b3971364f172eb0b9636c7142cc84ff695467bc065897958bde85a3c06cfd5"
envoy_gateway_image="envoyproxy/gateway:${envoy_gateway_version}"
playwright_image="mcr.microsoft.com/playwright@sha256:dcc5531e97840b9b5e794f2814476b21571c5124a3fca2267d73041f56e7580e"
dex_image="ghcr.io/dexidp/dex@sha256:8499afd690c437f52301efd2b05b2455da5bd2dfc20332cd697dc9937f808462"
dex_cache_image="dsh-phase2-dex-cache:test"
envoy_data_plane_image="docker.io/envoyproxy/envoy:distroless-v1.39.1@sha256:eb2c01c13125d1629637cb4e4cce7207009fb7cc2c8027f9742758549d15b6f4"
envoy_data_plane_cache_image="dsh-phase2-envoy-cache:test"
envoy_shutdown_image="docker.io/envoyproxy/gateway-dev:latest"
dex_issuer="https://dex.dsh-system.svc:15556/dex"
# Dex encodes (userID, connectorID) as the standards-facing OIDC subject.
dex_alice_sub="CglhbGljZS1zdWISBWxvY2Fs"
extension_pids=()

k() {
  kubectl --kubeconfig "$kubeconfig" "$@"
}

expect_failure() {
  if "$@"; then
    echo "command unexpectedly succeeded: $*" >&2
    return 1
  fi
}

dump_failure() {
  echo "Phase 2 kind verification failed; redacted cluster evidence follows" >&2
  k get gateway,httproute,backend,backendtlspolicy,securitypolicy -A >&2 || true
  k get cellsnapshots,volumesnapshots -A >&2 || true
  k get cells -A -o wide >&2 || true
  k get deployment,statefulset,service,pod -A >&2 || true
  k get events -A --sort-by=.lastTimestamp | tail -100 >&2 || true
  k -n dsh-system logs deployment/cell-operator --tail=200 >&2 || true
  k -n dsh-system logs deployment/cell-authorizer --tail=200 >&2 || true
  k -n envoy-gateway-system logs deployment/envoy-gateway --tail=200 >&2 || true
}

cleanup() {
  trap - ERR
  set +e
  for pid in "$gateway_forward_pid" "$dex_forward_pid"; do
    if [[ -n "$pid" ]]; then
      kill "$pid" >/dev/null 2>&1 || true
    fi
  done
  for pid in "${extension_pids[@]}"; do
    kill "$pid" >/dev/null 2>&1 || true
  done
  kind delete cluster --name "$cluster_name" >/dev/null 2>&1 || true
  docker rm -f "$registry_name" >/dev/null 2>&1 || true
  rm -f "$kubeconfig" "$kind_config"
  if ! rm -rf "$test_root" >/dev/null 2>&1; then
    docker run --rm --volume "$test_root:/cleanup" --entrypoint /bin/sh "$playwright_image" \
      -c 'rm -rf /cleanup/* /cleanup/.[!.]* /cleanup/..?*' >/dev/null 2>&1 || true
    rmdir "$test_root" >/dev/null 2>&1 || true
  fi
}
trap 'dump_failure' ERR
trap cleanup EXIT

wait_cell() {
  local namespace="$1" name="$2" wanted="$3" reason="${4:-}" json
  for _ in $(seq 1 240); do
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

wait_route() {
  local namespace="$1" name="$2" json
  for _ in $(seq 1 240); do
    json="$(k -n "$namespace" get httproute "$name" -o json 2>/dev/null || true)"
    if [[ -n "$json" ]] && jq -e '
      any(.status.parents[]?.conditions[]?; .type == "Accepted" and .status == "True") and
      any(.status.parents[]?.conditions[]?; .type == "ResolvedRefs" and .status == "True")
    ' <<<"$json" >/dev/null; then
      return
    fi
    sleep 1
  done
  echo "HTTPRoute $namespace/$name did not become Accepted with resolved refs" >&2
  return 1
}

wait_route_rejected() {
  local namespace="$1" name="$2" json
  for _ in $(seq 1 120); do
    json="$(k -n "$namespace" get httproute "$name" -o json 2>/dev/null || true)"
    if [[ -n "$json" ]] && jq -e '
      any(.status.parents[]?.conditions[]?; .status == "False")
    ' <<<"$json" >/dev/null; then
      return
    fi
    sleep 1
  done
  echo "HTTPRoute $namespace/$name did not report a rejected condition" >&2
  return 1
}

wait_cell_event() {
  local namespace="$1" name="$2" reason="$3" json
  for _ in $(seq 1 60); do
    json="$(k -n "$namespace" get events --field-selector "involvedObject.kind=Cell,involvedObject.name=${name}" -o json 2>/dev/null || true)"
    if [[ -n "$json" ]] && jq -e --arg reason "$reason" '
      any(.items[]?; .reason == $reason)
    ' <<<"$json" >/dev/null; then
      return
    fi
    sleep 1
  done
  echo "Cell $namespace/$name did not emit event $reason" >&2
  return 1
}

wait_gateway() {
  for _ in $(seq 1 600); do
    if k -n dsh-system get gateway dsh -o json | jq -e '
      any(.status.conditions[]?; .type == "Programmed" and .status == "True")
    ' >/dev/null 2>&1; then
      return
    fi
    sleep 1
  done
  echo "Gateway dsh-system/dsh did not become Programmed" >&2
  return 1
}

start_forward() {
  local namespace="$1" resource="$2" mapping="$3" logfile="$4" port="$5"
  k -n "$namespace" port-forward "$resource" "$mapping" >"$logfile" 2>&1 &
  local pid=$!
  for _ in $(seq 1 60); do
    if bash -c "</dev/tcp/127.0.0.1/$port" >/dev/null 2>&1; then
      echo "$pid"
      return
    fi
    sleep 1
  done
  sed -n '1,120p' "$logfile" >&2
  return 1
}

browser() {
  local mode="$1" username="${2:-alice@example.com}" expect_url="${3:-}" expect_status="${4:-}"
  if [[ "${USE_HOST_CHROME:-0}" == "1" ]]; then
    if [[ ! -d "$test_root/browser/node_modules" ]]; then
      cp "$repo_root/test/e2e/phase2/package.json" \
        "$repo_root/test/e2e/phase2/package-lock.json" \
        "$repo_root/test/e2e/phase2/browser.cjs" \
        "$test_root/browser/"
      npm --prefix "$test_root/browser" ci --ignore-scripts --no-audit --no-fund
    fi
    MODE="$mode" USERNAME="$username" \
      CELL_A_URL="https://${cell_a_authority}" \
      CELL_B_URL="https://${cell_b_authority}" \
      EXPECT_URL="$expect_url" EXPECT_STATUS="$expect_status" \
      CHROME_EXECUTABLE="${CHROME_EXECUTABLE:-/usr/bin/google-chrome}" \
      node "$test_root/browser/browser.cjs"
    return
  fi
  docker run --rm --network host \
    --add-host auth.cells.test:127.0.0.1 \
    --add-host "${cell_a_host}:127.0.0.1" \
    --add-host "${cell_b_host}:127.0.0.1" \
    --add-host dex.dsh-system.svc:127.0.0.1 \
    --volume "$repo_root/test/e2e/phase2:/src:ro" \
    --volume "$test_root/browser:/work" \
    --env "MODE=$mode" \
    --env "USERNAME=$username" \
    --env "CELL_A_URL=https://${cell_a_authority}" \
    --env "CELL_B_URL=https://${cell_b_authority}" \
    --env "EXPECT_URL=$expect_url" \
    --env "EXPECT_STATUS=$expect_status" \
    --env "HOST_UID=$(id -u)" \
    --env "HOST_GID=$(id -g)" \
    --env PLAYWRIGHT_BROWSERS_PATH=/ms-playwright \
    "$playwright_image" bash -euc '
      trap '\''chown -R "$HOST_UID:$HOST_GID" /work'\'' EXIT
      if [[ ! -d /work/node_modules ]]; then
        cp /src/package.json /src/package-lock.json /src/browser.cjs /work/
        cd /work
        npm ci --ignore-scripts --no-audit --no-fund
      fi
      node /work/browser.cjs
    '
}

browser_eventually() {
  local mode="$1" username="${2:-alice@example.com}"
  for _ in $(seq 1 30); do
    if browser "$mode" "$username"; then
      return
    fi
    sleep 1
  done
  echo "browser mode $mode did not recover" >&2
  return 1
}

sed \
  -e "s/REGISTRY_PORT/${registry_port}/g" \
  -e "s/REGISTRY_NAME/${registry_name}/g" \
  "$repo_root/test/e2e/phase2/kind-template.yaml" >"$kind_config"

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

# Keep gateway installation deterministic when the public registry is slow.
# The host cache is reused across the Phase 1-3 gates just like Calico.
if ! docker image inspect "$envoy_gateway_image" >/dev/null 2>&1; then
  docker pull --platform linux/amd64 "$envoy_gateway_image" >/dev/null
fi
docker save "$envoy_gateway_image" | docker exec --privileged -i "${cluster_name}-control-plane" \
  ctr --namespace=k8s.io images import --snapshotter=overlayfs - >/dev/null
if ! docker image inspect "$dex_image" >/dev/null 2>&1; then
  docker pull --platform linux/amd64 "$dex_image" >/dev/null
fi
docker tag "$dex_image" "$dex_cache_image"
docker save "$dex_cache_image" | docker exec --privileged -i "${cluster_name}-control-plane" \
  ctr --namespace=k8s.io images import --snapshotter=overlayfs - >/dev/null
docker exec "${cluster_name}-control-plane" ctr --namespace=k8s.io images tag \
  "docker.io/library/${dex_cache_image}" "$dex_image" >/dev/null
if ! docker image inspect "$envoy_data_plane_image" >/dev/null 2>&1; then
  docker pull --platform linux/amd64 "$envoy_data_plane_image" >/dev/null
fi
docker tag "$envoy_data_plane_image" "$envoy_data_plane_cache_image"
docker save "$envoy_data_plane_cache_image" | docker exec --privileged -i "${cluster_name}-control-plane" \
  ctr --namespace=k8s.io images import --snapshotter=overlayfs - >/dev/null
docker exec "${cluster_name}-control-plane" ctr --namespace=k8s.io images tag \
  "docker.io/library/${envoy_data_plane_cache_image}" "$envoy_data_plane_image" >/dev/null
if ! docker image inspect "$envoy_shutdown_image" >/dev/null 2>&1; then
  docker pull --platform linux/amd64 "$envoy_shutdown_image" >/dev/null
fi
docker save "$envoy_shutdown_image" | docker exec --privileged -i "${cluster_name}-control-plane" \
  ctr --namespace=k8s.io images import --snapshotter=overlayfs - >/dev/null

revision="$(git -C "$repo_root" rev-parse HEAD)"
local_cell="${CELL_IMAGE:-dsh-phase2-cell:test}"
local_operator="${OPERATOR_IMAGE:-dsh-phase2-operator:test}"
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
cell_repo_digest="$(docker inspect "$cell_repo:e2e" --format '{{index .RepoDigests 0}}')"
operator_repo_digest="$(docker inspect "$operator_repo:e2e" --format '{{index .RepoDigests 0}}')"
cell_digest="${cell_repo_digest##*@}"
operator_digest="${operator_repo_digest##*@}"
test "$cell_digest" != "$cell_repo_digest"
test "$operator_digest" != "$operator_repo_digest"

curl -fsSL \
  "https://github.com/envoyproxy/gateway/releases/download/${envoy_gateway_version}/install.yaml" \
  -o "$test_root/envoy-gateway-install.yaml"
echo "${envoy_gateway_sha256}  ${test_root}/envoy-gateway-install.yaml" | sha256sum --check
grep -q 'gateway.networking.k8s.io/bundle-version: v1.6.1' "$test_root/envoy-gateway-install.yaml"
k apply --server-side -f "$test_root/envoy-gateway-install.yaml"
k -n envoy-gateway-system rollout status deployment/envoy-gateway --timeout=600s
k apply -f "$repo_root/test/e2e/phase2/envoy-gateway-rbac.yaml"
k -n envoy-gateway-system create configmap envoy-gateway-config \
  --from-file="envoy-gateway.yaml=$repo_root/config/phase2/envoy-gateway.yaml" \
  --dry-run=client -o yaml | k apply -f -
k -n envoy-gateway-system rollout restart deployment/envoy-gateway
k -n envoy-gateway-system rollout status deployment/envoy-gateway --timeout=600s

k apply -f "$repo_root/config/crd/bases/dsh.isolated.io_cells.yaml"
k wait --for=condition=Established crd/cells.dsh.isolated.io --timeout=60s
k apply -f "$repo_root/config/default/namespace.yaml"
k label namespace dsh-system dsh.isolated.io/routes=enabled --overwrite

openssl req -x509 -newkey rsa:2048 -nodes -days 2 \
  -keyout "$test_root/ca.key" -out "$test_root/ca.crt" \
  -subj /CN=dsh-phase2-test-ca >/dev/null 2>&1
openssl req -new -newkey rsa:2048 -nodes \
  -keyout "$test_root/gateway.key" -out "$test_root/gateway.csr" \
  -subj '/CN=*.cells.test' \
  -addext 'subjectAltName=DNS:*.cells.test,DNS:auth.cells.test' >/dev/null 2>&1
openssl x509 -req -days 2 -in "$test_root/gateway.csr" \
  -CA "$test_root/ca.crt" -CAkey "$test_root/ca.key" -CAcreateserial \
  -copy_extensions copy -out "$test_root/gateway.crt" >/dev/null 2>&1
openssl req -new -newkey rsa:2048 -nodes \
  -keyout "$test_root/dex.key" -out "$test_root/dex.csr" \
  -subj /CN=dex.dsh-system.svc \
  -addext 'subjectAltName=DNS:dex.dsh-system.svc' >/dev/null 2>&1
openssl x509 -req -days 2 -in "$test_root/dex.csr" \
  -CA "$test_root/ca.crt" -CAkey "$test_root/ca.key" -CAserial "$test_root/ca.srl" \
  -copy_extensions copy -out "$test_root/dex.crt" >/dev/null 2>&1
k -n dsh-system create secret tls dsh-gateway-tls \
  --cert="$test_root/gateway.crt" --key="$test_root/gateway.key"
k -n dsh-system create secret tls dex-tls \
  --cert="$test_root/dex.crt" --key="$test_root/dex.key"
k -n dsh-system create secret generic dsh-oidc-client \
  --from-literal=client-secret=dsh-phase2-client-secret
k -n dsh-system create configmap dex-ca --from-file="ca.crt=$test_root/ca.crt"
k apply -f "$repo_root/test/e2e/phase2/dex.yaml"
k -n dsh-system rollout status deployment/dex --timeout=600s

k create namespace collision
k label namespace collision dsh.isolated.io/routes=enabled
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
k -n collision create role "${collision_base}-access" --verb=get --resource=configmaps
k apply -f - <<EOF
apiVersion: gateway.networking.k8s.io/v1
kind: HTTPRoute
metadata:
  name: ${collision_base}
  namespace: collision
spec:
  parentRefs:
    - name: dsh
      namespace: dsh-system
      sectionName: wrong-listener
  hostnames: ["${collision_base}.cells.test"]
  rules:
    - backendRefs:
        - name: not-a-cell
          port: 80
EOF

kubectl kustomize "$repo_root/config/phase2" | sed \
  "s#ghcr.io/guomonth/dsh-isolated-runtime-operator:main#${operator_repo}@${operator_digest}#g" \
  >"$test_root/phase2.yaml"
k apply -f "$test_root/phase2.yaml"
k -n dsh-system patch deployment cell-operator --type=strategic -p "$(jq -cn '{spec:{template:{spec:{containers:[{name:"manager",args:["--gateway-name=dsh","--gateway-namespace=dsh-system","--gateway-section-name=https","--base-domain=cells.test","--external-https-port=18443"]}]}}}}')"
k -n dsh-system patch deployment cell-authorizer --type=strategic -p "$(jq -cn --arg issuer "$dex_issuer" '{spec:{template:{spec:{containers:[{name:"authorizer",args:["--oidc-issuer="+$issuer,"--oidc-client-id=dsh-browser","--gateway-name=dsh","--gateway-namespace=dsh-system","--gateway-section-name=https","--base-domain=cells.test","--external-https-port=18443"],env:[{name:"SSL_CERT_FILE",value:"/etc/dsh-ca/ca.crt"}],volumeMounts:[{name:"dex-ca",mountPath:"/etc/dsh-ca",readOnly:true}]}],volumes:[{name:"dex-ca",configMap:{name:"dex-ca"}}]}}}}')"

k apply -f - <<EOF
apiVersion: gateway.envoyproxy.io/v1alpha1
kind: Backend
metadata:
  name: dex
  namespace: dsh-system
spec:
  endpoints:
    - fqdn:
        hostname: dex.dsh-system.svc
        port: 15556
---
apiVersion: gateway.networking.k8s.io/v1
kind: BackendTLSPolicy
metadata:
  name: dex
  namespace: dsh-system
spec:
  targetRefs:
    - group: gateway.envoyproxy.io
      kind: Backend
      name: dex
  validation:
    caCertificateRefs:
      - group: ""
        kind: ConfigMap
        name: dex-ca
    hostname: dex.dsh-system.svc
EOF
k -n dsh-system patch envoyproxy dsh --type=merge -p '{"spec":{"provider":{"kubernetes":{"envoyService":{"type":"ClusterIP"},"envoyDeployment":{"patch":{"type":"StrategicMerge","value":{"spec":{"template":{"metadata":{"labels":{"dsh.isolated.io/access":"true"}}}}}}}}}}}'
k -n dsh-system patch gateway dsh --type=merge -p '{"spec":{"listeners":[{"name":"https","protocol":"HTTPS","port":443,"hostname":"*.cells.test","tls":{"mode":"Terminate","certificateRefs":[{"kind":"Secret","name":"dsh-gateway-tls"}]},"allowedRoutes":{"namespaces":{"from":"Selector","selector":{"matchLabels":{"dsh.isolated.io/routes":"enabled"}}},"kinds":[{"group":"gateway.networking.k8s.io","kind":"HTTPRoute"}]}}]}}'
k -n dsh-system patch httproute auth-callback --type=merge -p '{"spec":{"parentRefs":[{"name":"dsh","sectionName":"https"}],"hostnames":["auth.cells.test"],"rules":[{"matches":[{"path":{"type":"PathPrefix","value":"/"}}],"backendRefs":[{"name":"cell-authorizer","port":8080}]}]}}'
k apply -f - <<EOF
apiVersion: gateway.envoyproxy.io/v1alpha1
kind: SecurityPolicy
metadata:
  name: dsh-browser-access
  namespace: dsh-system
spec:
  targetRefs:
    - group: gateway.networking.k8s.io
      kind: Gateway
      name: dsh
      sectionName: https
  oidc:
    provider:
      issuer: ${dex_issuer}
      authorizationEndpoint: ${dex_issuer}/auth
      tokenEndpoint: ${dex_issuer}/token
      backendRefs:
        - group: gateway.envoyproxy.io
          kind: Backend
          name: dex
          port: 15556
    clientID: dsh-browser
    clientSecret:
      name: dsh-oidc-client
    cookieConfig:
      sameSite: Lax
    cookieDomain: cells.test
    redirectURL: https://auth.cells.test:18443/oauth2/callback
    logoutPath: /logout
    scopes: [openid, profile, email, groups]
    forwardAccessToken: false
    forwardIDToken:
      header: X-Dsh-Oidc-Token
    denyRedirect:
      headers:
        - name: :path
          type: Prefix
          value: /api/
        - name: Sec-Fetch-Dest
          type: Exact
          value: empty
  extAuth:
    grpc:
      backendRefs:
        - group: gateway.envoyproxy.io
          kind: Backend
          name: cell-authorizer
          port: 9001
    headersToExtAuth: [X-Dsh-Oidc-Token]
    timeout: 2s
    failOpen: false
    includeRouteMetadata: true
    statusOnError: 503
EOF
k -n dsh-system rollout status deployment/cell-operator --timeout=180s
k -n dsh-system rollout status deployment/cell-authorizer --timeout=180s

wait_cell collision collision True
test "$(k -n collision get role "${collision_base}-access" -o jsonpath='{.rules[0].resources[0]}')" = configmaps
test -z "$(k -n collision get role "${collision_base}-access" -o jsonpath='{.metadata.ownerReferences}')"
wait_route_rejected collision "$collision_base"
wait_cell_event collision collision OwnershipConflict
k -n collision delete role "${collision_base}-access"
for _ in $(seq 1 60); do
  test "$(k -n collision get role "${collision_base}-access" -o jsonpath='{.rules[0].verbs[0]}' 2>/dev/null || true)" = access && break
  sleep 1
done
test "$(k -n collision get role "${collision_base}-access" -o jsonpath='{.rules[0].verbs[0]}')" = access
test "$(k -n collision get httproute "$collision_base" -o jsonpath='{.spec.rules[0].backendRefs[0].name}')" = not-a-cell
k -n collision delete httproute "$collision_base"
wait_route collision "$collision_base"
k -n collision delete cell collision --wait=true

k create namespace tenant-a
k create namespace tenant-b
k label namespace tenant-a tenant-b dsh.isolated.io/routes=enabled
for namespace in tenant-a tenant-b; do
  k apply -f - <<EOF
apiVersion: dsh.isolated.io/v1alpha1
kind: Cell
metadata:
  name: main
  namespace: ${namespace}
spec:
  image: ${cell_repo}@${cell_digest}
  storage:
    size: 1Gi
    retentionPolicy: Retain
EOF
done
wait_cell tenant-a main True
wait_cell tenant-b main True
cell_a_uid="$(k -n tenant-a get cell main -o jsonpath='{.metadata.uid}')"
cell_b_uid="$(k -n tenant-b get cell main -o jsonpath='{.metadata.uid}')"
cell_a_base="cell-${cell_a_uid}"
cell_b_base="cell-${cell_b_uid}"
cell_a_host="${cell_a_base}.cells.test"
cell_b_host="${cell_b_base}.cells.test"
cell_a_authority="${cell_a_host}:18443"
cell_b_authority="${cell_b_host}:18443"
wait_route tenant-a "$cell_a_base"
wait_route tenant-b "$cell_b_base"
wait_gateway

for namespace in tenant-a tenant-b; do
  k -n "$namespace" get cell main -o json | jq -e '
    (.status | keys | sort) == (["conditions","dshVersion","imageDigest","observedGeneration"] | sort)
  ' >/dev/null
done
test "$(k -n tenant-a get httproute "$cell_a_base" -o jsonpath='{.spec.hostnames[0]}')" = "$cell_a_host"
test "$(k -n tenant-a get httproute "$cell_a_base" -o jsonpath='{.spec.parentRefs[0].namespace}')" = dsh-system
test "$(k -n tenant-a get httproute "$cell_a_base" -o jsonpath='{.spec.parentRefs[0].sectionName}')" = https
test "$(k -n tenant-a get role "${cell_a_base}-access" -o jsonpath='{.rules[0].resourceNames[0]}')" = main
test "$(k -n tenant-a get statefulset "$cell_a_base" -o json | jq -r '.spec.template.spec.containers[0].env[] | select(.name == "CELL_AUTHORITY") | .value')" = "$cell_a_authority"

k -n tenant-a create rolebinding alice-cell-a \
  --role="${cell_a_base}-access" --user="${dex_issuer}#${dex_alice_sub}"
k auth can-i access "cells.dsh.isolated.io/main" \
  --namespace=tenant-a --as="${dex_issuer}#${dex_alice_sub}" | grep -qx yes

gateway_service="$(k -n dsh-system get service \
  -l gateway.envoyproxy.io/owning-gateway-name=dsh,gateway.envoyproxy.io/owning-gateway-namespace=dsh-system \
  -o jsonpath='{.items[0].metadata.name}')"
test -n "$gateway_service"
gateway_forward_pid="$(start_forward dsh-system "service/${gateway_service}" 18443:443 "$test_root/gateway-forward.log" 18443)"
dex_forward_pid="$(start_forward dsh-system service/dex 15556:15556 "$test_root/dex-forward.log" 15556)"
mkdir -p "$test_root/browser"

browser initial alice@example.com
k -n tenant-b create rolebinding alice-cell-b \
  --role="${cell_b_base}-access" --user="${dex_issuer}#${dex_alice_sub}"
browser grant alice@example.com
k -n tenant-b delete rolebinding alice-cell-b
browser deny alice@example.com
k -n tenant-b create rolebinding team-b-cell-b \
  --role="${cell_b_base}-access" --group="${dex_issuer}#team-b"
browser_eventually group bob@example.com

cell_a_pod="$(k -n tenant-a get pod -l "dsh.isolated.io/cell-uid=${cell_a_uid}" -o jsonpath='{.items[0].metadata.name}')"
k -n tenant-a exec "$cell_a_pod" -- node -e \
  "require('node:fs').writeFileSync('/var/lib/dsh/data/workspace/phase2-marker', 'durable')"
old_cell_a_pod_uid="$(k -n tenant-a get pod "$cell_a_pod" -o jsonpath='{.metadata.uid}')"
if k -n tenant-a logs "$cell_a_pod" 2>&1 | grep -Eq '[?&]token=[A-Za-z0-9_-]{43}|eyJ[A-Za-z0-9_-]+\.eyJ'; then
  echo "launch or OIDC token leaked into the pre-restart Cell logs" >&2
  exit 1
fi
k -n tenant-a delete pod "$cell_a_pod" --wait=true
k -n tenant-a rollout status statefulset "$cell_a_base" --timeout=180s
cell_a_pod="$(k -n tenant-a get pod -l "dsh.isolated.io/cell-uid=${cell_a_uid}" -o jsonpath='{.items[0].metadata.name}')"
test "$(k -n tenant-a get pod "$cell_a_pod" -o jsonpath='{.metadata.uid}')" != "$old_cell_a_pod_uid"
k -n tenant-a exec "$cell_a_pod" -- test -f /var/lib/dsh/data/workspace/phase2-marker
browser resume alice@example.com

if k -n dsh-system logs deployment/cell-authorizer 2>&1 | grep -Eq 'dsh-phase2-client-secret|eyJ[A-Za-z0-9_-]+\.eyJ'; then
  echo "OIDC secret or token leaked into pre-restart authorizer logs" >&2
  exit 1
fi
k -n dsh-system scale deployment/cell-authorizer --replicas=0
k -n dsh-system wait pod -l app.kubernetes.io/name=cell-authorizer --for=delete --timeout=90s
browser status alice@example.com "https://${cell_a_authority}/api/settings/describe" 503
k -n dsh-system scale deployment/cell-authorizer --replicas=1
k -n dsh-system rollout status deployment/cell-authorizer --timeout=180s
browser_eventually resume alice@example.com

k -n dsh-system run denied-client --image="${cell_repo}@${cell_digest}" \
  --command -- node -e 'setInterval(()=>{}, 60000)'
k -n dsh-system wait pod/denied-client --for=condition=Ready --timeout=120s
direct_probe="const h=require('http');const r=h.get({host:'${cell_a_base}.tenant-a.svc',port:80,path:'/'},x=>process.exit(0));r.on('error',()=>process.exit(2));setTimeout(()=>process.exit(3),3000)"
expect_failure k -n dsh-system exec denied-client -- node -e "$direct_probe"
management_probe="const n=require('net').connect(8081,'${cell_a_base}-0.${cell_a_base}-headless.tenant-a.svc',()=>process.exit(0));n.on('error',()=>process.exit(2));setTimeout(()=>process.exit(3),3000)"
expect_failure k -n dsh-system exec denied-client -- node -e "$management_probe"
k -n dsh-system run allowed-client --image="${cell_repo}@${cell_digest}" \
  --labels=dsh.isolated.io/access=true \
  --command -- node -e 'setInterval(()=>{}, 60000)'
k -n dsh-system wait pod/allowed-client --for=condition=Ready --timeout=120s
k -n dsh-system exec allowed-client -- node -e "$direct_probe"
expect_failure k -n dsh-system exec allowed-client -- node -e "$management_probe"
k -n tenant-b run wrong-namespace-client --image="${cell_repo}@${cell_digest}" \
  --labels=dsh.isolated.io/access=true \
  --command -- node -e 'setInterval(()=>{}, 60000)'
k -n tenant-b wait pod/wrong-namespace-client --for=condition=Ready --timeout=120s
expect_failure k -n tenant-b exec wrong-namespace-client -- node -e "$direct_probe"
egress_probe="const n=require('net').connect(443,'kubernetes.default.svc',()=>process.exit(0));n.on('error',()=>process.exit(2));setTimeout(()=>process.exit(3),3000)"
k -n tenant-a exec "$cell_a_pod" -- node -e "$egress_probe"

if k logs -n tenant-a "$cell_a_pod" 2>&1 | grep -Eq '[?&]token=[A-Za-z0-9_-]{43}|eyJ[A-Za-z0-9_-]+\.eyJ'; then
  echo "launch or OIDC token leaked into Cell logs" >&2
  exit 1
fi
if k -n dsh-system logs deployment/cell-authorizer 2>&1 | grep -Eq 'dsh-phase2-client-secret|eyJ[A-Za-z0-9_-]+\.eyJ'; then
  echo "OIDC secret or token leaked into authorizer logs" >&2
  exit 1
fi
if k -n dsh-system logs deployment/cell-operator 2>&1 | grep -Eq 'dsh-phase2-client-secret|eyJ[A-Za-z0-9_-]+\.eyJ'; then
  echo "OIDC secret or token leaked into operator logs" >&2
  exit 1
fi
if { k get cells -A -o json; k get events -A -o json; } | grep -Eq 'dsh-phase2-client-secret|eyJ[A-Za-z0-9_-]+\.eyJ|[?&]token=[A-Za-z0-9_-]{43}'; then
  echo "secret or token leaked into Cell status or events" >&2
  exit 1
fi
docker save --output "$test_root/operator-image.tar" "$local_operator"
docker save --output "$test_root/cell-image.tar" "$local_cell"
if grep -aFq dsh-phase2-client-secret "$test_root/operator-image.tar" "$test_root/cell-image.tar"; then
  echo "OIDC client secret leaked into an image layer" >&2
  exit 1
fi

# Phase 3 reuses this exact browser/Gateway fixture and supplies only the CSI
# lifecycle extension. Sourcing keeps one cluster, registry and browser state
# without copying the Phase 2 stack into another test harness.
if [[ -n "${DSH_E2E_EXTENSION:-}" ]]; then
  # shellcheck source=/dev/null
  source "$DSH_E2E_EXTENSION"
fi

echo "Phase 2 trusted browser access passed"
