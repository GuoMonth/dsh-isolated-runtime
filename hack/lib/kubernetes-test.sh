#!/usr/bin/env bash
# Test-only baseline; published local archives retain their own image identity.
# shellcheck disable=SC2034 # Consumed by the sourcing test runners.
case "${DSH_TEST_K8S_VERSION:-1.37.0}" in
  1.37.0) kind_node_image="kindest/node:v1.37.0@sha256:a1ed56cfb0e7b93589bdf97c8cd566405a265939e3620fc4f5de89adff580ae5" ;;
  1.36.4) kind_node_image="kindest/node:v1.36.4@sha256:099e049362a1526b2db71494e1947aae99bd16290d7c895f2b7ea312e3cbfaed" ;;
  *) echo "unsupported test Kubernetes version: ${DSH_TEST_K8S_VERSION}" >&2; return 1 ;;
esac

configure_test_registry() {
  local node="$1" port="$2" registry="$3"
  local directory="/etc/containerd/certs.d/localhost:${port}"
  docker exec "$node" mkdir -p "$directory"
  printf '[host."http://%s:5000"]\n' "$registry" |
    docker exec -i "$node" cp /dev/stdin "$directory/hosts.toml"
}
