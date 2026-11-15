# Ops tasks

What to build for CI/CD and observability. Sources: G1–G8 and FV#1–5 are the gaps and
"further value" items in the handoff doc; "checklist" is the senior-backend CI/CD checklist.
Nothing is deferred: every open item below is in scope.

## 1. Dev: owned by the service team

- [ ] Health checks: decide what the startup, liveness and readiness probes each check (G1)
- [ ] Graceful shutdown order: fail readiness, wait, then shut down (G1)
- [ ] Process-pause experiment: stall ingest under a tight liveness probe and see whether Kubernetes kills a pod that's only paused (G1, DDIA ch. 8)
- [ ] Requests and limits from load-test data; confirm `GOMAXPROCS` (G2)
- [ ] Autoscaler settings: min/max replicas and CPU target (G2)
- [ ] Autoscaling measurements: time to scale up, p99 during that gap, CPU throttling, and the bottleneck moving to Postgres (G2)
- [ ] Rollout settings in `deploy/ingest.yaml`: `maxSurge`, `maxUnavailable`, `minReadySeconds`, `revisionHistoryLimit` (G3)
- [ ] Rollback rule: which metric, what threshold, how long to watch (G3)
- [ ] Canary stage in Jenkins: one `ingest-canary` replica, compare its error rate and p99, then promote or delete (G3)
- [ ] Blue-green design: two Deployments switched by the Service selector, and when to choose it over canary (G3)
- [ ] Write the expand-then-contract rule for schema changes (G3)
- [ ] SLI and SLO definition, plus a burn-rate alert rule (G4)
- [ ] Error-budget and burn-rate panels in Grafana (G4)
- [ ] Decide where the SLI is measured: server histogram or client-side timing (G4, exercise 1b)
- [ ] Instrumentation review: RED metrics (rate, errors, duration) per route, and labels that keep cardinality safe
- [ ] Pool sizing (FV#3)
- [ ] Capacity numbers per pod and per Postgres (FV#4)
- [ ] Profiling under load (FV#5)
- [ ] Results log: before/after numbers for every experiment (FV#1)
- [ ] Trace instrumentation and span placement in code: `otelhttp` in ingest and the simulator so trace context travels, and a pgx query tracer (G5)
- [ ] Trace sampling: keep a fixed share up front, plus slow and failed requests (G5)
- [ ] Tracing cost: pprof at 100% vs 1% sampling; exemplars from latency panels to traces (G5)
- [ ] Datadog mapping: agent, APM, monitors and dashboards against this stack, and the cost trade-off (G6)
- [ ] Structured log fields and what to log (G8)
- [ ] Load shedding, retries, idempotency (FV#2)
- [ ] Security in the service: API authentication, TLS on ingest, per-client rate limits
- [ ] Recovery targets for ridehail data (how much data may be lost, how fast it must be back) and a restore drill that meets them
- [ ] Incident practice: one game day per failure type in section 3 (needs G1, G3, G4)
- [ ] Reading dashboards and debugging on call (ongoing)
- [ ] "Explain what's there" sessions: Docker, CI/CD, Kubernetes and Prometheus in this repo, one piece per session
- [ ] Resume stories: a short story for each finished gap, for deep-dive questions
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
- [ ] Ingress for ingest: an ingress controller in kind, ingest reachable from outside the cluster, TLS ending at the Ingress (needed by service security)
- [ ] Kustomize base and overlays for `deploy/`, when Stage 3 adds a second variant (needed by Stage 3 replication)
- [x] kind cluster (needed by everything)
- [x] Prometheus and Grafana (needed by G3's rollback check, G4 and every measurement)
- [x] Jenkins and its deploy RBAC (needed by G3's rollback and canary)

## 3. Incident practice

Every game day follows the same response, which is also the shape of an interview answer:

1. **Detect:** which alert or dashboard showed it.
2. **Assess impact:** who is affected, how badly, and whether it's getting worse.
3. **Mitigate first:** roll back, fail over, shed load, scale up, or turn off a feature flag.
4. **Diagnose:** metrics, then traces, then logs.
5. **Fix:** the real fix, once things are stable.
6. **Postmortem:** blameless, with action items so it can't happen the same way again.

One game day per failure type. Log time-to-detect and time-to-mitigate for each.

- [ ] Bad deploy: roll back. Ship a build that fails 10% of pings (G3)
- [ ] Dependency down: fail readiness, degrade gracefully. Kill `postgres-0` under load (G1)
- [ ] Traffic spike: shed load, autoscale. `DRIVERS=40000` (FV#2, G2)
- [ ] Resource exhaustion (pool, memory, disk): limits, then fix the leak. Shrink the pool, set a low memory limit
- [ ] Slow dependency: timeouts, find it with a trace. `/debug/stall` (G5)
- [ ] Retry storm or cascading failure: backoff, retry budgets. FV#2 with retries switched on
- [ ] Hot partition: split or salt the key. Stage 4 with an airport-heavy fleet
- [ ] Bad migration or corrupted data: restore from backup. The restore drill
- [ ] Replication lag or failover: route reads, promote a replica. Stage 3
- [ ] Expired certificate or token: rotate it, alert before expiry. Let the TLS certificate expire

## Open decisions

- [ ] Where ops work lands: Jenkins deploys every commit on `main`, so either keep ops changes on an `ops` branch until each passes its "done when", or accept that `main` is live
