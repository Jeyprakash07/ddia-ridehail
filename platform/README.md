# platform

Shared infrastructure that service teams build on. In a company this is the platform team's
repo, usually with Terraform in place of these scripts.

| Path | What it is |
|---|---|
| `cluster/cluster-up.sh` | Creates the kind cluster and installs Prometheus and Grafana |
| `cluster/monitoring-values.yaml` | Settings for the kube-prometheus-stack Helm chart |
| `jenkins/jenkins-up.sh` | Starts Jenkins on http://localhost:8081, connected to the cluster |
| `jenkins/Dockerfile` | The Jenkins image: Jenkins plus kubectl |
| `jenkins/rbac.yaml` | The `jenkins-deployer` account: can roll out ingest in `ridehail`, nothing else |
| `jenkins/wait-for-image.sh` | Used by the service's `Jenkinsfile`: waits until CI has published an image |

Run the scripts from the repo root, in this order: `platform/cluster/cluster-up.sh`, then the
service's `scripts/deploy.sh`, then `platform/jenkins/jenkins-up.sh`.
