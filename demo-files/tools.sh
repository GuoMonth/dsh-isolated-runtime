# shellcheck shell=bash
# shellcheck disable=SC2154
# Private, pinned development tools; never writes to /usr or the user's config.
prepare_demo_tools() {
  for command in curl tar sha256sum openssl git; do
    command -v "$command" >/dev/null || { echo "Missing prerequisite: $command" >&2; return 1; }
  done
  mkdir -p "$demo_root/tools/bin"
  export PATH="$demo_root/tools/bin:$PATH"
  if ! command -v kind >/dev/null || [[ "$(kind version)" != *v0.32.0* ]]; then
    curl -fsSL --retry 5 --retry-all-errors --connect-timeout 20 --max-time 600 "https://kind.sigs.k8s.io/dl/v0.32.0/kind-$demo_os-$demo_arch" -o "$demo_root/tools/bin/kind"
    curl -fsSL --retry 5 --retry-all-errors --connect-timeout 20 --max-time 600 "https://kind.sigs.k8s.io/dl/v0.32.0/kind-$demo_os-$demo_arch.sha256sum" | awk '{print $1 "  kind"}' > "$demo_root/tools/bin/kind.sha256"
    (cd "$demo_root/tools/bin" && sha256sum -c kind.sha256)
    chmod 755 "$demo_root/tools/bin/kind"
  fi
  if ! command -v kubectl >/dev/null || [[ "$(kubectl version --client -o json)" != *v1.34.0* ]]; then
    curl -fsSL --retry 5 --retry-all-errors --connect-timeout 20 --max-time 600 "https://dl.k8s.io/release/v1.34.0/bin/$demo_os/$demo_arch/kubectl" -o "$demo_root/tools/bin/kubectl"
    printf '%s  kubectl\n' "$(curl -fsSL --retry 5 --retry-all-errors --connect-timeout 20 --max-time 120 "https://dl.k8s.io/release/v1.34.0/bin/$demo_os/$demo_arch/kubectl.sha256")" > "$demo_root/tools/bin/kubectl.sha256"
    (cd "$demo_root/tools/bin" && sha256sum -c kubectl.sha256)
    chmod 755 "$demo_root/tools/bin/kubectl"
  fi
  if ! command -v jq >/dev/null || [[ "$(jq --version)" != jq-1.8.1 ]]; then
    curl -fsSL --retry 5 --retry-all-errors --connect-timeout 20 --max-time 600 "https://github.com/jqlang/jq/releases/download/jq-1.8.1/jq-$jq_os-$demo_arch" -o "$demo_root/tools/bin/jq"
    curl -fsSL --retry 5 --retry-all-errors --connect-timeout 20 --max-time 120 https://github.com/jqlang/jq/releases/download/jq-1.8.1/sha256sum.txt | awk -v name="jq-$jq_os-$demo_arch" '$2==name {print $1 "  jq"}' > "$demo_root/tools/bin/jq.sha256"
    test -s "$demo_root/tools/bin/jq.sha256"
    (cd "$demo_root/tools/bin" && sha256sum -c jq.sha256)
    chmod 755 "$demo_root/tools/bin/jq"
  fi
  if ! command -v node >/dev/null || [[ "$(node --version)" != v24.18.0 || "$(node -p process.arch)" != "$node_arch" ]]; then
    local node_name="node-v24.18.0-$demo_os-$node_arch"
    curl -fsSL --retry 5 --retry-all-errors --connect-timeout 20 --max-time 600 "https://nodejs.org/dist/v24.18.0/$node_name.tar.gz" -o "$demo_root/tools/node.tar.gz"
    curl -fsSL --retry 5 --retry-all-errors --connect-timeout 20 --max-time 120 https://nodejs.org/dist/v24.18.0/SHASUMS256.txt | awk -v name="$node_name.tar.gz" '$2==name {print $1 "  node.tar.gz"}' > "$demo_root/tools/node.sha256"
    test -s "$demo_root/tools/node.sha256"
    (cd "$demo_root/tools" && sha256sum -c node.sha256 && tar -xf node.tar.gz)
    ln -sf "../$node_name/bin/node" "$demo_root/tools/bin/node"
    ln -sf "../$node_name/bin/npm" "$demo_root/tools/bin/npm"
  fi
}
