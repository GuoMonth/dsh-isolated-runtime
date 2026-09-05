# shellcheck shell=bash
# shellcheck disable=SC2154
# Private, pinned development tools; never writes to /usr or the user's config.
prepare_demo_tools() {
  for command in docker curl tar sha256sum openssl flock git; do
    command -v "$command" >/dev/null || { echo "Missing prerequisite: $command" >&2; return 1; }
  done
  [[ "$(uname -s)/$(uname -m)" == Linux/x86_64 ]] || { echo 'Supported platform: Linux x86_64' >&2; return 1; }
  docker info >/dev/null 2>&1 || { echo 'Docker daemon is unavailable to this user' >&2; return 1; }
  mkdir -p "$demo_root/tools/bin"
  export PATH="$demo_root/tools/bin:$PATH"
  if ! command -v kind >/dev/null || [[ "$(kind version)" != *v0.32.0* ]]; then
    curl -fsSL --retry 5 --retry-all-errors --connect-timeout 20 --max-time 600 https://kind.sigs.k8s.io/dl/v0.32.0/kind-linux-amd64 -o "$demo_root/tools/bin/kind"
    curl -fsSL --retry 5 --retry-all-errors --connect-timeout 20 --max-time 600 https://kind.sigs.k8s.io/dl/v0.32.0/kind-linux-amd64.sha256sum | awk '{print $1 "  kind"}' > "$demo_root/tools/bin/kind.sha256"
    (cd "$demo_root/tools/bin" && sha256sum -c kind.sha256)
    chmod 755 "$demo_root/tools/bin/kind"
  fi
  if ! command -v kubectl >/dev/null || [[ "$(kubectl version --client -o json)" != *v1.34.0* ]]; then
    curl -fsSL --retry 5 --retry-all-errors --connect-timeout 20 --max-time 600 https://dl.k8s.io/release/v1.34.0/bin/linux/amd64/kubectl -o "$demo_root/tools/bin/kubectl"
    printf '%s  kubectl\n' "$(curl -fsSL --retry 5 --retry-all-errors --connect-timeout 20 --max-time 120 https://dl.k8s.io/release/v1.34.0/bin/linux/amd64/kubectl.sha256)" > "$demo_root/tools/bin/kubectl.sha256"
    (cd "$demo_root/tools/bin" && sha256sum -c kubectl.sha256)
    chmod 755 "$demo_root/tools/bin/kubectl"
  fi
  if ! command -v jq >/dev/null || [[ "$(jq --version)" != jq-1.8.1 ]]; then
    curl -fsSL --retry 5 --retry-all-errors --connect-timeout 20 --max-time 600 https://github.com/jqlang/jq/releases/download/jq-1.8.1/jq-linux-amd64 -o "$demo_root/tools/bin/jq"
    curl -fsSL --retry 5 --retry-all-errors --connect-timeout 20 --max-time 120 https://github.com/jqlang/jq/releases/download/jq-1.8.1/sha256sum.txt | awk '$2=="jq-linux-amd64" {print $1 "  jq"}' > "$demo_root/tools/bin/jq.sha256"
    test -s "$demo_root/tools/bin/jq.sha256"
    (cd "$demo_root/tools/bin" && sha256sum -c jq.sha256)
    chmod 755 "$demo_root/tools/bin/jq"
  fi
  if ! command -v node >/dev/null || [[ "$(node --version)" != v24.18.0 ]]; then
    curl -fsSL --retry 5 --retry-all-errors --connect-timeout 20 --max-time 600 https://nodejs.org/dist/v24.18.0/node-v24.18.0-linux-x64.tar.xz -o "$demo_root/tools/node.tar.xz"
    curl -fsSL --retry 5 --retry-all-errors --connect-timeout 20 --max-time 120 https://nodejs.org/dist/v24.18.0/SHASUMS256.txt | awk '$2=="node-v24.18.0-linux-x64.tar.xz" {print $1 "  node.tar.xz"}' > "$demo_root/tools/node.sha256"
    (cd "$demo_root/tools" && sha256sum -c node.sha256 && tar -xf node.tar.xz)
    ln -sf ../node-v24.18.0-linux-x64/bin/node "$demo_root/tools/bin/node"
    ln -sf ../node-v24.18.0-linux-x64/bin/npm "$demo_root/tools/bin/npm"
  fi
}
