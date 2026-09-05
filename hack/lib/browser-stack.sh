# shellcheck shell=bash
install_reference_network() {
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
pull_image "$envoy_gateway_image"
docker save "$envoy_gateway_image" | docker exec --privileged -i "${cluster_name}-control-plane" \
  ctr --namespace=k8s.io images import --snapshotter=overlayfs - >/dev/null
pull_image "$dex_image"
docker tag "$dex_image" "$dex_cache_image"
docker save "$dex_cache_image" | docker exec --privileged -i "${cluster_name}-control-plane" \
  ctr --namespace=k8s.io images import --snapshotter=overlayfs - >/dev/null
docker exec "${cluster_name}-control-plane" ctr --namespace=k8s.io images tag \
  "docker.io/library/${dex_cache_image}" "$dex_image" >/dev/null
pull_image "$envoy_data_plane_image"
docker tag "$envoy_data_plane_image" "$envoy_data_plane_cache_image"
docker save "$envoy_data_plane_cache_image" | docker exec --privileged -i "${cluster_name}-control-plane" \
  ctr --namespace=k8s.io images import --snapshotter=overlayfs - >/dev/null
docker exec "${cluster_name}-control-plane" ctr --namespace=k8s.io images tag \
  "docker.io/library/${envoy_data_plane_cache_image}" "$envoy_data_plane_image" >/dev/null
pull_image "$envoy_shutdown_image"
docker tag "$envoy_shutdown_image" "$envoy_shutdown_cache_image"
docker save "$envoy_shutdown_cache_image" | docker exec --privileged -i "${cluster_name}-control-plane" \
  ctr --namespace=k8s.io images import --snapshotter=overlayfs - >/dev/null
docker exec "${cluster_name}-control-plane" ctr --namespace=k8s.io images tag \
  "docker.io/library/${envoy_shutdown_cache_image}" "docker.io/envoyproxy/gateway-dev:latest" >/dev/null

}
install_reference_identity() {
curl -fsSL --retry 5 --retry-all-errors --retry-delay 2 \
  "https://github.com/envoyproxy/gateway/releases/download/${envoy_gateway_version}/install.yaml" \
  -o "$test_root/envoy-gateway-install.yaml"
echo "${envoy_gateway_sha256}  ${test_root}/envoy-gateway-install.yaml" | sha256sum --check
grep -q 'gateway.networking.k8s.io/bundle-version: v1.6.1' "$test_root/envoy-gateway-install.yaml"
k apply --server-side -f "$test_root/envoy-gateway-install.yaml"
k -n envoy-gateway-system rollout status deployment/envoy-gateway --timeout=600s
k apply -f "$repo_root/test/e2e/phase2/envoy-gateway-rbac.yaml"
k -n envoy-gateway-system create configmap envoy-gateway-config \
  --from-file="envoy-gateway.yaml=$repo_root/config/browser/envoy-gateway.yaml" \
  --dry-run=client -o yaml | k apply -f -
k -n envoy-gateway-system rollout restart deployment/envoy-gateway
k -n envoy-gateway-system rollout status deployment/envoy-gateway --timeout=600s

k apply -f "$repo_root/config/crd/bases/dsh.isolated.io_cells.yaml"
k wait --for=condition=Established crd/cells.dsh.isolated.io --timeout=60s
k apply -f "$repo_root/config/default/namespace.yaml"
k label namespace dsh-system dsh.isolated.io/routes=enabled --overwrite

openssl req -x509 -newkey rsa:2048 -nodes -days 365 \
  -keyout "$test_root/ca.key" -out "$test_root/ca.crt" \
  -subj /CN=dsh-phase2-test-ca >/dev/null 2>&1
openssl req -new -newkey rsa:2048 -nodes \
  -keyout "$test_root/gateway.key" -out "$test_root/gateway.csr" \
  -subj '/CN=*.cells.test' \
  -addext 'subjectAltName=DNS:*.cells.test,DNS:auth.cells.test' >/dev/null 2>&1
openssl x509 -req -days 365 -in "$test_root/gateway.csr" \
  -CA "$test_root/ca.crt" -CAkey "$test_root/ca.key" -CAcreateserial \
  -copy_extensions copy -out "$test_root/gateway.crt" >/dev/null 2>&1
openssl req -new -newkey rsa:2048 -nodes \
  -keyout "$test_root/dex.key" -out "$test_root/dex.csr" \
  -subj /CN=dex.dsh-system.svc \
  -addext 'subjectAltName=DNS:dex.dsh-system.svc' >/dev/null 2>&1
openssl x509 -req -days 365 -in "$test_root/dex.csr" \
  -CA "$test_root/ca.crt" -CAkey "$test_root/ca.key" -CAserial "$test_root/ca.srl" \
  -copy_extensions copy -out "$test_root/dex.crt" >/dev/null 2>&1
k -n dsh-system create secret tls dsh-gateway-tls \
  --cert="$test_root/gateway.crt" --key="$test_root/gateway.key" --dry-run=client -o yaml | k apply -f -
k -n dsh-system create secret tls dex-tls \
  --cert="$test_root/dex.crt" --key="$test_root/dex.key" --dry-run=client -o yaml | k apply -f -
k -n dsh-system create secret generic dsh-oidc-client \
  --from-literal=client-secret=dsh-phase2-client-secret --dry-run=client -o yaml | k apply -f -
k -n dsh-system create configmap dex-ca --from-file="ca.crt=$test_root/ca.crt" --dry-run=client -o yaml | k apply -f -
k apply -f "$repo_root/test/e2e/phase2/dex.yaml"
k -n dsh-system rollout status deployment/dex --timeout=600s

}
configure_reference_access() {
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

}
