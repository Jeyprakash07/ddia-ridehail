# Ops tasks

What to build for CI/CD and observability. Sources: G1–G8 and FV#1–5 are the gaps and
"further value" items in the handoff doc; "checklist" is the senior-backend CI/CD checklist.
Nothing is deferred: every open item below is in scope.

## 1. Dev: owned by the service team

- [ ] Health checks: decide what readiness and liveness each check (G1)
- [ ] Graceful shutdown order: fail readiness, wait, then shut down (G1)
- [ ] Requests and limits from load-test data; confirm `GOMAXPROCS` (G2)
- [ ] Autoscaler settings: min/max replicas and CPU target (G2)
- [ ] Rollout settings in `deploy/ingest.yaml`: `maxSurge`, `maxUnavailable` (G3)
- [ ] Rollback rule: which metric, what threshold, how long to watch (G3)
- [ ] Write the expand-then-contract rule for schema changes (G3)
- [ ] SLI and SLO definition, plus a burn-rate alert rule (G4)
- [ ] Pool sizing (FV#3)
- [ ] Capacity numbers per pod and per Postgres (FV#4)
- [ ] Profiling under load (FV#5)
- [ ] Results log: before/after numbers for every experiment (FV#1)
- [ ] Trace instrumentation and span placement in code (G5)
- [ ] Structured log fields and what to log (G8)
- [ ] Load shedding, retries, idempotency (FV#2)
- [ ] Security in the service: API authentication, TLS on ingest, per-client rate limits
- [ ] Recovery targets for ridehail data (how much data may be lost, how fast it must be back) and a restore drill that meets them
- [ ] Incident practice: game days (stall ingest, kill Postgres, ship a bad build), detect from alerts and dashboards, mitigate, write a blameless postmortem (needs G1, G3, G4)
- [ ] Reading dashboards and debugging on call (ongoing)
- [x] CI stages, test strategy, integration tests against real Postgres (checklist)
- [x] Dockerfile quality, images tagged by commit (checklist)
- [x] Lint, triaging security findings, merging dependency updates (checklist)

## 2. Platform: only what the Dev list depends on

- [ ] One deploy path and one artifact: build once, scan and deploy the same digest (G7, needed by G3)
- [ ] metrics-server in the cluster (needed by G2)
- [ ] Deploy annotation in Grafana (needed by G3 and reading dashboards)
- [ ] Tempo (needed by G5)
- [ ] Loki and a log collector (needed by G8)
- [ ] Terraform for the local platform: the kind cluster, the monitoring Helm release and namespaces, using the kind, helm and kubernetes providers (replaces `platform/cluster/cluster-up.sh`)
- [ ] Postgres backups: a scheduled dump plus WAL archiving for point-in-time recovery (needed by the restore drill)
- [ ] TLS certificates for ingest, e.g. cert-manager with a local CA (needed by service security)
- [x] kind cluster (needed by everything)
- [x] Prometheus and Grafana (needed by G3's rollback check, G4 and every measurement)
- [x] Jenkins and its deploy RBAC (needed by G3's rollback and canary)
