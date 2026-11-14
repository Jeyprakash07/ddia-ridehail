#!/usr/bin/env bash
# Builds the image, loads it into the kind cluster and applies the manifests.
# Run it again after any code change.
#
#   scripts/deploy.sh
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
CLUSTER="${CLUSTER:-ridehail}"
# A fresh tag per build, so the cluster can never run a stale image by mistake.
TAG="ridehail:$(date +%Y%m%d-%H%M%S)"

docker build -t "$TAG" "$ROOT"
kind load docker-image "$TAG" --name "$CLUSTER"

kubectl apply -f "$ROOT/deploy/namespace.yaml"

# The database password is generated once and exists only inside the cluster.
if [ -z "$(kubectl -n ridehail get secret postgres --ignore-not-found -o name)" ]; then
  kubectl -n ridehail create secret generic postgres \
    "--from-literal=password=$(head -c 24 /dev/urandom | od -An -tx1 | tr -d ' \n')"
fi

kubectl apply -f "$ROOT/deploy/postgres.yaml"
sed "s|ridehail:dev|$TAG|g" "$ROOT/deploy/ingest.yaml" | kubectl apply -f -

# Grafana's sidecar picks up any ConfigMap labelled grafana_dashboard=1.
kubectl -n monitoring create configmap ridehail-dashboards \
  --from-file="$ROOT/deploy/dashboards/ingest.json" --dry-run=client -o yaml |
  kubectl label --local -f - grafana_dashboard=1 -o yaml |
  kubectl apply -f -

kubectl -n ridehail rollout status statefulset/postgres --timeout=300s
kubectl -n ridehail rollout status deployment/ingest --timeout=180s

echo
echo "Deployed $TAG"
