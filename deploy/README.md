# deploy

What runs in the cluster for this service, and how it is configured. In a company this is
usually its own "deploy" or "config" repo, owned by the service team. CD tools read it:
`scripts/deploy.sh` for local builds, and the `Jenkinsfile` for every commit on `main`.

| File | What it is |
|---|---|
| `namespace.yaml` | The `ridehail` namespace |
| `postgres.yaml` | Postgres 17 StatefulSet, its metrics exporter and ServiceMonitor |
| `ingest.yaml` | The ingest Deployment, Service and ServiceMonitor. `ridehail:dev` is a placeholder that the deploy step replaces with the real image |
| `dashboards/` | The Grafana dashboard for ingest, loaded through a labelled ConfigMap |

There is one environment here. With several, each would get its own folder or overlay
(`dev/`, `staging/`, `prod/`) with its own replicas, resources and image tag.
