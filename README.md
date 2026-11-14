# ddia-ridehail

A ride-hailing dispatch backend, built naively on purpose and then repaired one chapter of
*Designing Data-Intensive Applications* at a time.

Simulated drivers move around Bengaluru and report their position every few seconds. Riders
ask for a car, dispatch picks a nearby driver, pricing reacts to demand, and payments settle
the trip. Each of those becomes a different data problem once there is enough load.

## How every stage runs

1. Build the simplest thing that works.
2. Load it until something gives.
3. Find the bottleneck in the metrics rather than by guessing.
4. Read the part of the book that explains it.
5. You write the fix (the "Your piece" column). Claude writes the scaffolding and reviews.
6. Measure again.

Code marked `EXERCISE` is yours. Everything else is scaffolding.

## Roadmap

Later rows will shift as real bottlenecks show up.

| Stage | Chapter | We build | What breaks | Your piece |
|---|---|---|---|---|
| 0 | 1 · Reliability, scalability | Cluster, Prometheus and Grafana, simulator, ingest on one Postgres | The load generator misreports latency | Percentiles; measuring from the scheduled send time |
| 1 | 2–3 · Data models, storage engines | Ping history and latest-position tables, geo queries | B-tree write amplification, index bloat, full-scan `nearby` | A small LSM store; a geohash index |
| 2 | 4 · Encoding and evolution | JSON to Protobuf, rolling upgrades | CPU burned on JSON; old and new versions running together | A schema change that is compatible both ways |
| 3 | 5 · Replication | Postgres primary with replicas, read splitting | Replication lag, stale reads, lost writes on failover | Read-your-writes routing |
| 4 | 6 · Partitioning | Sharded location store | The airport becomes a hot partition; rebalancing | The partitioner and request router |
| 5 | 7 · Transactions | Trip matching, payments | Two riders get the same driver (write skew); double charges | A correct matching transaction; idempotency keys |
| 6 | 8 · Trouble with distributed systems | Fault injection: partitions, pauses, clock skew | Zombie leaders, misleading timeouts | Fencing tokens |
| 7 | 9 · Consistency and consensus | Leader election and leases on etcd | Split brain | A lease-based lock |
| 8 | 11 · Stream processing | Redpanda log, change data capture, surge pricing | Duplicate and out-of-order events | An idempotent consumer; windowed surge pricing |
| 9 | 10 · Batch processing | Event lake on MinIO, columnar analytics | Row-store scans that never finish | A batch job for driver earnings |
| 10 | 12 · The future of data systems | Derived data end to end | Systems drifting out of agreement | A reconciliation audit |

Streams come before batch here because the batch stage reads events the log has already captured.

## Hardware budget

One laptop: 6 cores / 12 threads, 16 GB RAM, NVMe SSD. The cluster gets about 10 GB, so
"scale" means tens of thousands of requests per second and tens of millions of rows. Heavy
components are swapped in per stage instead of all running at once.

## Layout

| Path | What it is |
|---|---|
| `cmd/ingest` | HTTP service: accepts pings, answers nearby-driver queries, exposes `/metrics` |
| `cmd/simulator` | Load generator: a fleet of virtual drivers |
| `internal/store` | Storage interface: in-memory baseline and Postgres, sharing one contract test |
| `internal/sim` | Fleet placement and movement |
| `internal/loadgen` | Latency recording (exercise 1 lives here) |
| `internal/geo`, `internal/model` | Distance maths and shared types |
| `deploy` | Kubernetes manifests, kube-prometheus-stack values, Grafana dashboard |
| `scripts` | WSL setup and the cluster scripts below |

Everything runs inside Ubuntu 24.04 on WSL2. Run the commands below from an Ubuntu shell in
the repo root.

## One-time setup

Install Docker Engine, kubectl, kind v0.30, helm 3 and Go 1.24 inside Ubuntu, with your user
in the `docker` group and systemd enabled in `/etc/wsl.conf`. Then give WSL its share of the
laptop in `C:\Users\<you>\.wslconfig` and restart it with `wsl --shutdown`:

```ini
[wsl2]
memory=10GB
processors=10
vmIdleTimeout=-1
```

WSL stops Ubuntu, and with it the cluster, a few minutes after the last WSL process exits.
Keep an Ubuntu shell open while you work. The cluster survives a restart; it just takes a
minute to come back.

## Run it locally

Start the service:

```bash
go run ./cmd/ingest -addr 127.0.0.1:8080 -debug-endpoints
```

In a second shell, run 20 seconds of load and freeze the server for 2 seconds partway through:

```bash
go run ./cmd/simulator -drivers 400 -duration 20s -stall-at 8s -stall-for 2s
```

`-drivers 400` means 100 pings per second. Local runs are for checking behaviour;
throughput is measured inside the cluster, where each component has a fixed CPU limit.

Run the tests:

```bash
go test ./...
```

`TestPercentile` fails until exercise 1a is done. Everything else should pass. The Postgres
store tests skip unless `TEST_DATABASE_URL` points at a database they may empty.

## Run it on the cluster

```bash
scripts/cluster-up.sh    # kind cluster "ridehail" plus Prometheus and Grafana
scripts/deploy.sh        # build, load the image, apply manifests and the dashboard
scripts/load.sh          # 10,000 drivers for 60 s, run as a Job inside the cluster
```

`load.sh` takes `DRIVERS`, `PING_INTERVAL`, `DURATION`, `WORKERS`, `STALL_AT` and `STALL_FOR`
as environment variables, e.g. `DRIVERS=20000 DURATION=2m scripts/load.sh`. Run `deploy.sh`
again after any code change.

Grafana, with the "Ridehail ingest" dashboard:

```bash
kubectl -n monitoring port-forward svc/monitoring-grafana 3000:80
```

then open http://localhost:3000.

## Exercise 1: make the load generator tell the truth

Reading: chapter 1, "Describing Performance", including the sidebar on percentiles in practice.

Run the two commands above. The server is frozen for 2 of the 20 seconds, which is 10% of the
run. The report gives one mean and one max, which is not enough to tell how many pings were
affected or by how much.

**1a. Percentiles.** Implement `Percentile` in `internal/loadgen/latency.go` so that
`go test ./internal/loadgen` passes. Then print p50, p95, p99 and p99.9 in the simulator's
report and run the freeze again. Note what each percentile says about the freeze, and which
ones say nothing.

**1b. Coordinated omission.** The server was unreachable for 10% of the run, so p95 should
look terrible. Work out why it does not. Two questions to start from: which pings were
waiting to be sent during the freeze, and when did their stopwatch start? Change the
simulator so each ping's latency is measured from the moment it was *scheduled* to go out.
You are done when the same run reports a p95 above 500 ms.
