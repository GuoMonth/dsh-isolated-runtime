#!/usr/bin/env bash
# Build the production images and exercise the real DSH process under the
# non-root, read-only Cell runtime contract.
set -euo pipefail

repo_root="$(cd "$(dirname "$0")/.." && pwd)"
revision="$(git -C "$repo_root" rev-parse HEAD)"
cell_image="${CELL_IMAGE:-dsh-cell:test}"
operator_image="${OPERATOR_IMAGE:-dsh-operator:test}"
container_name="dsh-image-smoke-${RANDOM}"
smoke_root="$(mktemp -d)"
state_file="$smoke_root/probe-state.json"

cleanup() {
  docker rm -f "$container_name" >/dev/null 2>&1 || true
  if [[ -d "$smoke_root" ]] && docker image inspect "$cell_image" >/dev/null 2>&1; then
    docker run --rm --user 0 --entrypoint chmod \
      --mount "type=bind,src=$smoke_root,dst=/smoke" "$cell_image" -R a+rwx /smoke >/dev/null 2>&1 || true
  fi
  rm -rf "$smoke_root"
}
trap cleanup EXIT

if [[ "${SKIP_IMAGE_BUILD:-0}" != "1" ]]; then
  docker buildx build --platform linux/amd64 --load \
    --build-arg "SOURCE_REVISION=$revision" \
    -f "$repo_root/images/operator/Dockerfile" -t "$operator_image" "$repo_root"
  docker buildx build --platform linux/amd64 --load \
    --build-arg "SOURCE_REVISION=$revision" \
    -f "$repo_root/images/cell/Dockerfile" -t "$cell_image" "$repo_root"
fi

docker run --rm "$operator_image" --help >/dev/null
docker run --rm --entrypoint /cell-authorizer "$operator_image" --help >/dev/null
docker run --rm --entrypoint node "$cell_image" -e \
  "if (require('/opt/dsh/node_modules/@deepseek-ai/dsh/package.json').version !== '0.1.2-rc.1') process.exit(1)"

mkdir -p "$smoke_root/data" "$smoke_root/private" "$smoke_root/tmp"
chmod 0777 "$smoke_root/data" "$smoke_root/private" "$smoke_root/tmp"

start_cell() {
  docker run --detach --name "$container_name" \
    --read-only --security-opt no-new-privileges --cap-drop ALL \
    --mount "type=bind,src=$smoke_root/tmp,dst=/tmp" \
    --mount "type=bind,src=$smoke_root/data,dst=/var/lib/dsh/data" \
    --mount "type=bind,src=$smoke_root/private,dst=/var/lib/dsh-private" \
    --env CELL_AUTHORITY=cell.example.test \
    --publish 127.0.0.1::8080 --publish 127.0.0.1::8081 \
    "$cell_image" >/dev/null
}

wait_ready() {
  local management_port
  management_port="$(docker port "$container_name" 8081/tcp)"
  management_port="${management_port##*:}"
  for _ in $(seq 1 90); do
    if curl --silent --show-error --fail "http://127.0.0.1:$management_port/readyz" >/dev/null 2>&1; then
      curl --silent --show-error --fail "http://127.0.0.1:$management_port/version" \
        | jq -e '.contractVersion == "v1alpha1" and .dshVersion == "0.1.2-rc.1"' >/dev/null
      return
    fi
    sleep 1
  done
  docker logs "$container_name" >&2 || true
  echo "Cell image did not become ready" >&2
  exit 1
}

proxy_address() {
  local address
  address="$(docker port "$container_name" 8080/tcp)"
  printf '127.0.0.1:%s\n' "${address##*:}"
}

start_cell
wait_ready
go run "$repo_root/test/e2e/dshprobe" \
  --connect "$(proxy_address)" --authority cell.example.test --state-file "$state_file"
docker exec "$container_name" node -e \
  "require('node:fs').writeFileSync('/var/lib/dsh/data/workspace/phase1-marker', 'durable')"
docker stop --time 30 "$container_name" >/dev/null
if docker logs "$container_name" 2>&1 | grep -Eq '[?&]token=[A-Za-z0-9_-]{43}'; then
  echo "launch token leaked into Cell logs" >&2
  exit 1
fi
docker rm "$container_name" >/dev/null

start_cell
wait_ready
go run "$repo_root/test/e2e/dshprobe" \
  --connect "$(proxy_address)" --authority cell.example.test --state-file "$state_file" --resume
docker exec "$container_name" test -f /var/lib/dsh/data/workspace/phase1-marker
docker exec "$container_name" test -f /var/lib/dsh-private/.credentials.yaml
docker exec "$container_name" sh -c 'test ! -e /var/lib/dsh/data/home/.credentials.yaml'

management_address="$(docker port "$container_name" 8081/tcp)"
management_port="${management_address##*:}"
test "$(curl --silent --output /dev/null --write-out '%{http_code}' -X POST \
  "http://127.0.0.1:$management_port/quiesce")" = 404
curl --silent --show-error --fail "http://127.0.0.1:$management_port/livez" >/dev/null
curl --silent --show-error --fail "http://127.0.0.1:$management_port/readyz" >/dev/null
docker stop --time 30 "$container_name" >/dev/null

test "$(docker inspect "$container_name" --format '{{.Config.User}}')" = "1000:1000"
test "$(docker inspect "$container_name" --format '{{.HostConfig.ReadonlyRootfs}}')" = "true"
