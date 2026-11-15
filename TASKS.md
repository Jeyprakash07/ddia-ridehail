# Ops tasks

What to build for CI/CD and observability. Sources: G1–G8 and FV#1–5 are the gaps and
"further value" items in the handoff doc; "checklist" is the senior-backend CI/CD checklist.
Nothing is deferred: sections 1–3 are worked through in order, and section 4 is picked up
whenever its trigger comes up while practising.

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
- [ ] Instrumentation review: RED metrics (rate, errors, duration) per route, and labels that keep cardinality safe
- [ ] Pool sizing (FV#3)
- [ ] Trace instrumentation and span placement in code: `otelhttp` in ingest and the simulator so trace context travels, and a pgx query tracer (G5)
- [ ] Trace sampling: keep a fixed share up front, plus slow and failed requests (G5)
- [ ] Tracing cost: pprof at 100% vs 1% sampling; exemplars from latency panels to traces (G5)
- [ ] Datadog mapping: agent, APM, monitors and dashboards against this stack, and the cost trade-off (G6)
- [ ] Structured log fields and what to log (G8)
- [ ] Load shedding, retries, idempotency (FV#2)
- [ ] Security in the service: API authentication, TLS on ingest, per-client rate limits
- [ ] Recovery targets for ridehail data (how much data may be lost, how fast it must be back) and a restore drill that meets them
- [ ] Incident practice: one game day per failure type in section 3 (needs G1, G3, G4)
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
- [ ] Ingress for ingest: a classic Ingress with an ingress controller (e.g. Traefik) in kind, ingest reachable from outside the cluster, TLS ending at the Ingress (needed by service security)
- [ ] Gateway for ingest: the Gateway API with Envoy Gateway in kind, routing and TLS at the gateway, a rate limit per client, and weighted traffic splitting for the G3 canary (needed by service security and G3)
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

One game day per failure type. Log time-to-detect and time-to-mitigate for each. The hot
partition and replication game days are in section 4, because they wait for their stages.

- [ ] Bad deploy: roll back. Ship a build that fails 10% of pings (G3)
- [ ] Dependency down: fail readiness, degrade gracefully. Kill `postgres-0` under load (G1)
- [ ] Traffic spike: shed load, autoscale. `DRIVERS=40000` (FV#2, G2)
- [ ] Resource exhaustion (pool, memory, disk): limits, then fix the leak. Shrink the pool, set a low memory limit
- [ ] Slow dependency: timeouts, find it with a trace. `/debug/stall` (G5)
- [ ] Retry storm or cascading failure: backoff, retry budgets. FV#2 with retries switched on
- [ ] Bad migration or corrupted data: restore from backup. The restore drill
- [ ] Expired certificate or token: rotate it, alert before expiry. Let the TLS certificate expire

## 4. As needed while practising

Each task waits for its trigger, then gets done in that session.

- [ ] **Next load test:** check the dashboard's CPU panels show correct ingest figures
- [ ] **Every load test:** read the dashboards and debug as if on call
- [ ] **After every experiment:** add its before/after numbers to the results log (FV#1)
- [ ] **Exercise 1b done:** decide where the SLI is measured, server histogram or client-side timing (G4)
- [ ] **Stage 0 done:** Postgres tuning (`shared_buffers`, `synchronous_commit`, WAL settings), measured with the same load test
- [ ] **Stage 0 and Stage 1 load runs:** capacity numbers per pod and per Postgres (FV#4)
- [ ] **A load test looks CPU-bound:** profile under load (FV#5)
- [ ] **Stage 3 adds a second variant:** Kustomize base and overlays for `deploy/`
- [ ] **Stage 3:** game day for replication lag or failover. Route reads, promote a replica
- [ ] **Stage 4:** game day for a hot partition. Split or salt the key, with an airport-heavy fleet
- [ ] **Revising a tool:** an "explain what's there" session for Docker, CI/CD, Kubernetes or Prometheus
- [ ] **Each gap in section 1 finished:** write its resume story

## 5. Deep dives

What to know about each system for interviews, and where it's practised. Know how it works
and when to choose it; running it in production is out of scope.

### Kafka
- [ ] Partitions, ordering within a partition, and choosing the partition key
- [ ] Consumer groups, offsets and rebalancing; consumer lag
- [ ] Replication: leaders, in-sync replicas, `acks` and `min.insync.replicas`
- [ ] Delivery guarantees: idempotent producers, transactions, exactly-once
- [ ] Retention and log compaction

Practised in: the Kafka HLD mock (Thu 3 Dec) and ridehail's stream-processing stage (Redpanda).

### Redis
- [ ] Data structures and what each is for: strings, hashes, sorted sets, streams
- [ ] Caching patterns, TTLs and eviction policies (LRU, LFU)
- [ ] Single-threaded execution, Lua scripts, and rate limiting with them
- [ ] Persistence: RDB snapshots vs the append-only file
- [ ] Replication, Sentinel, and Redis Cluster's hash slots
- [ ] Locks: why Redlock is debated, and fencing tokens

Practised in: the Distributed Cache and Rate Limiter HLD mocks (week 1) and the LRU Cache LLD round.

### MongoDB
- [ ] Document model and schema design: embed vs reference
- [ ] Indexes: compound and multikey
- [ ] Replica sets, elections, and read and write concerns
- [ ] Sharding: choosing a shard key, chunks and the balancer
- [ ] Multi-document transactions, and when you'd choose MongoDB over Postgres

Practised in: concepts only. DDIA ch. 2 (document vs relational) and ch. 6 (partitioning) through ridehail's stages; no hands-on MongoDB.

### Postgres
- [ ] MVCC, vacuum and table bloat
- [ ] Isolation levels and locking, including advisory locks
- [ ] Index types (B-tree, GIN, GiST, BRIN) and reading `EXPLAIN`
- [ ] WAL, checkpoints and the settings that trade durability for speed
- [ ] Replication: streaming vs logical
- [ ] Connection pooling (PgBouncer) and table partitioning

Practised in: ridehail Stages 1, 3 and 5, the Postgres tuning task (section 4), pool sizing, and the DB slots in your plan (Mon 30 Nov, Tue 1 Dec).

## Open decisions

- [ ] Where ops work lands: Jenkins deploys every commit on `main`, so either keep ops changes on an `ops` branch until each passes its "done when", or accept that `main` is live
