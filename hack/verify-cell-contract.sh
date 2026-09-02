#!/usr/bin/env bash
# Exercise Cell validation against a real Kubernetes API server.
set -euo pipefail

repo_root="$(cd "$(dirname "$0")/.." && pwd)"
cluster_name="dsh-cell-contract-${RANDOM}"
kubeconfig="$(mktemp)"

cleanup() {
  kind delete cluster --name "$cluster_name" >/dev/null 2>&1 || true
  rm -f "$kubeconfig"
}
trap cleanup EXIT

kind create cluster --name "$cluster_name" --kubeconfig "$kubeconfig" --wait 120s
kubectl --kubeconfig "$kubeconfig" apply -f "$repo_root/config/crd/bases/dsh.isolated.io_cells.yaml"
kubectl --kubeconfig "$kubeconfig" wait --for=condition=Established crd/cells.dsh.isolated.io --timeout=60s
kubectl --kubeconfig "$kubeconfig" create namespace tenant-alice
kubectl --kubeconfig "$kubeconfig" create secret generic dsh-provider-credentials \
  --namespace tenant-alice --from-literal=DEEPSEEK_API_KEY=fixture
kubectl --kubeconfig "$kubeconfig" create --validate=strict -f "$repo_root/config/samples/dsh_v1alpha1_cell.yaml"

test "$(kubectl --kubeconfig "$kubeconfig" -n tenant-alice get cell assistant -o jsonpath='{.spec.securityClass}')" = "standard"
test "$(kubectl --kubeconfig "$kubeconfig" -n tenant-alice get cell assistant -o jsonpath='{.spec.storage.retentionPolicy}')" = "Retain"

if kubectl --kubeconfig "$kubeconfig" create --validate=strict -f "$repo_root/testdata/cell-contract/invalid-floating-image.yaml"; then
  echo "floating image was accepted" >&2
  exit 1
fi
if kubectl --kubeconfig "$kubeconfig" create --validate=strict -f "$repo_root/testdata/cell-contract/invalid-legacy-field.yaml"; then
  echo "legacy tenant field was accepted" >&2
  exit 1
fi
if kubectl --kubeconfig "$kubeconfig" -n tenant-alice patch cell assistant --type=merge -p '{"spec":{"storage":{"size":"10Gi"}}}'; then
  echo "storage shrink was accepted" >&2
  exit 1
fi
if kubectl --kubeconfig "$kubeconfig" -n tenant-alice patch cell assistant --type=merge -p '{"spec":{"storage":{"storageClassName":"other"}}}'; then
  echo "storage class mutation was accepted" >&2
  exit 1
fi
if kubectl --kubeconfig "$kubeconfig" -n tenant-alice patch cell assistant --type=json \
  -p '[{"op":"remove","path":"/spec/storage/storageClassName"}]'; then
  echo "storage class removal was accepted" >&2
  exit 1
fi
kubectl --kubeconfig "$kubeconfig" create --validate=strict \
  -f "$repo_root/testdata/cell-contract/valid-no-storage-class.yaml"
if kubectl --kubeconfig "$kubeconfig" -n tenant-alice patch cell no-storage-class --type=merge \
  -p '{"spec":{"storage":{"storageClassName":"standard"}}}'; then
  echo "storage class addition was accepted after creation" >&2
  exit 1
fi

kubectl --kubeconfig "$kubeconfig" -n tenant-alice patch cell assistant --subresource=status --type=merge \
  -p '{"status":{"observedGeneration":1,"conditions":[{"type":"Ready","status":"True","reason":"ContractVerified","message":"contract fixture is ready","lastTransitionTime":"2026-01-01T00:00:00Z"}],"dshVersion":"0.1.2-alpha.4","imageDigest":"sha256:0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"}}'
test "$(kubectl --kubeconfig "$kubeconfig" -n tenant-alice get cell assistant -o jsonpath='{.status.dshVersion}')" = "0.1.2-alpha.4"
if kubectl --kubeconfig "$kubeconfig" -n tenant-alice patch cell assistant --subresource=status --type=merge \
  -p '{"status":{"conditions":[{"type":"PodReady","status":"True","reason":"LegacyTopology","message":"must be rejected","lastTransitionTime":"2026-01-01T00:00:00Z"}]}}'; then
  echo "unsupported status condition was accepted" >&2
  exit 1
fi
