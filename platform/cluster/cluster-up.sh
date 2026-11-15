#!/usr/bin/env bash
# Starts the local kind cluster and its monitoring stack. Safe to run again.
#
#   platform/cluster/cluster-up.sh
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
CLUSTER="${CLUSTER:-ridehail}"

if kind get clusters 2>/dev/null | grep -qx "$CLUSTER"; then
  echo "kind cluster '$CLUSTER' already exists"
else
  kind create cluster --name "$CLUSTER" --wait 120s
fi
kubectl config use-context "kind-$CLUSTER"

helm repo add prometheus-community https://prometheus-community.github.io/helm-charts --force-update

helm upgrade --install monitoring prometheus-community/kube-prometheus-stack \
  --namespace monitoring --create-namespace \
  --values "$ROOT/platform/cluster/monitoring-values.yaml" \
  --wait --timeout 10m

echo
echo "Cluster is up. To open Grafana on http://localhost:3000 run:"
echo "  kubectl -n monitoring port-forward svc/monitoring-grafana 3000:80"
