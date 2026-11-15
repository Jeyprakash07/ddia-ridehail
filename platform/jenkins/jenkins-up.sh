#!/usr/bin/env bash
# Starts a local Jenkins on http://localhost:8081 that can deploy ingest to the
# kind cluster. Safe to run again. Run scripts/deploy.sh once first, so the
# ridehail namespace and the postgres secret exist.
#
#   platform/jenkins/jenkins-up.sh
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
CLUSTER="${CLUSTER:-ridehail}"
STATE="$HOME/.ridehail-jenkins"
mkdir -p "$STATE"

# A namespace-scoped deploy account instead of the cluster-admin kubeconfig.
kubectl apply -f "$ROOT/platform/jenkins/rbac.yaml"
TOKEN="$(kubectl -n ridehail create token jenkins-deployer --duration=8760h)"
SERVER="https://${CLUSTER}-control-plane:6443"
CA="$(kind get kubeconfig --internal --name "$CLUSTER" | sed -n 's/^ *certificate-authority-data: //p' | head -1)"

umask 077
cat > "$STATE/kubeconfig" <<EOF
apiVersion: v1
kind: Config
clusters:
  - name: kind
    cluster:
      server: $SERVER
      certificate-authority-data: $CA
users:
  - name: jenkins-deployer
    user:
      token: $TOKEN
contexts:
  - name: jenkins
    context: {cluster: kind, user: jenkins-deployer, namespace: ridehail}
current-context: jenkins
EOF

docker build -q -t ridehail-jenkins "$ROOT/platform/jenkins" >/dev/null

if docker container inspect jenkins >/dev/null 2>&1; then
  docker restart jenkins >/dev/null
else
  # On the "kind" network so it can reach the cluster's API server by name.
  # Bound to 127.0.0.1 so Jenkins is not reachable from other machines.
  docker run -d --name jenkins --restart unless-stopped \
    --network kind \
    -p 127.0.0.1:8081:8080 \
    -v jenkins_home:/var/jenkins_home \
    -v "$STATE/kubeconfig:/etc/jenkins-kubeconfig:ro" \
    -e KUBECONFIG=/etc/jenkins-kubeconfig \
    ridehail-jenkins >/dev/null
fi

echo "Jenkins is starting on http://localhost:8081"
echo "First time only, get the setup password with:"
echo "  docker exec jenkins cat /var/jenkins_home/secrets/initialAdminPassword"
