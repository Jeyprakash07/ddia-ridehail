package store

import (
	"context"
	"fmt"
	"math"
	"sort"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus"

	"ridehail/internal/geo"
	"ridehail/internal/model"
)

// schema is the stage 0 design: the tables most people would write first.
//
//   - pings keeps every report forever, one row each.
//   - driver_latest holds one row per driver and is overwritten on every ping.
//     Its (lat, lng) index covers columns that change on every write.
//
// Both choices are here to be measured, not defended.
var schema = []string{
	`CREATE TABLE IF NOT EXISTS pings (
		id          BIGSERIAL        PRIMARY KEY,
		driver_id   BIGINT           NOT NULL,
		lat         DOUBLE PRECISION NOT NULL,
		lng         DOUBLE PRECISION NOT NULL,
		status      SMALLINT         NOT NULL,
		device_ts   TIMESTAMPTZ      NOT NULL,
		received_at TIMESTAMPTZ      NOT NULL DEFAULT now()
	)`,
	`CREATE INDEX IF NOT EXISTS pings_driver_received ON pings (driver_id, received_at)`,
	`CREATE TABLE IF NOT EXISTS driver_latest (
		driver_id  BIGINT           PRIMARY KEY,
		lat        DOUBLE PRECISION NOT NULL,
		lng        DOUBLE PRECISION NOT NULL,
		status     SMALLINT         NOT NULL,
		updated_at TIMESTAMPTZ      NOT NULL
	)`,
	`CREATE INDEX IF NOT EXISTS driver_latest_lat_lng ON driver_latest (lat, lng)`,
}

const (
	insertPing = `
		INSERT INTO pings (driver_id, lat, lng, status, device_ts)
		VALUES ($1, $2, $3, $4, $5)`

	upsertLatest = `
		INSERT INTO driver_latest (driver_id, lat, lng, status, updated_at)
		VALUES ($1, $2, $3, $4, now())
		ON CONFLICT (driver_id) DO UPDATE
		SET lat = EXCLUDED.lat, lng = EXCLUDED.lng,
		    status = EXCLUDED.status, updated_at = EXCLUDED.updated_at`

	nearbyInBox = `
		SELECT driver_id, lat, lng
		FROM driver_latest
		WHERE status = $1 AND lat BETWEEN $2 AND $3 AND lng BETWEEN $4 AND $5`

	// migrationLock is an arbitrary pg_advisory_lock key. It stops several
	// replicas that start together from running the DDL at the same moment.
	migrationLock = 7_301_001
)

// Postgres stores pings in a single PostgreSQL database.
type Postgres struct {
	pool *pgxpool.Pool
}

// OpenPostgres connects, waits for the database to accept connections (it may
// still be starting), and creates the tables if they are missing. The pool is
// left at pgx's default size.
func OpenPostgres(ctx context.Context, dsn string) (*Postgres, error) {
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return nil, fmt.Errorf("configure postgres pool: %w", err)
	}
	p := &Postgres{pool: pool}
	if err := p.waitUntilReachable(ctx); err != nil {
		pool.Close()
		return nil, err
	}
	if err := p.migrate(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("create schema: %w", err)
	}
	return p, nil
}

// waitUntilReachable pings once a second until the database answers or ctx ends.
func (p *Postgres) waitUntilReachable(ctx context.Context) error {
	for {
		pingCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
		err := p.pool.Ping(pingCtx)
		cancel()
		if err == nil {
			return nil
		}
		select {
		case <-ctx.Done():
			return fmt.Errorf("postgres not reachable: %w", err)
		case <-time.After(time.Second):
		}
	}
}

func (p *Postgres) migrate(ctx context.Context) error {
	conn, err := p.pool.Acquire(ctx)
	if err != nil {
		return err
	}
	defer conn.Release()

	if _, err := conn.Exec(ctx, `SELECT pg_advisory_lock($1)`, migrationLock); err != nil {
		return err
	}
	// Unlock on a fresh context: if ctx has expired we still want to release.
	defer conn.Exec(context.WithoutCancel(ctx), `SELECT pg_advisory_unlock($1)`, migrationLock)

	for _, stmt := range schema {
		if _, err := conn.Exec(ctx, stmt); err != nil {
			return err
		}
	}
	return nil
}

// RecordPing appends to the history and overwrites the driver's latest
// position, in one transaction per ping.
func (p *Postgres) RecordPing(ctx context.Context, ping model.Ping) error {
	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx) // does nothing once Commit has succeeded

	status := int16(ping.Status)
	if _, err := tx.Exec(ctx, insertPing, ping.DriverID, ping.Lat, ping.Lng, status, time.UnixMilli(ping.TsMs)); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, upsertLatest, ping.DriverID, ping.Lat, ping.Lng, status); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// Nearby asks the database for every available driver inside the square that
// encloses the search circle, then measures exact distances here.
func (p *Postgres) Nearby(ctx context.Context, lat, lng, radiusM float64, limit int) ([]model.NearbyDriver, error) {
	dLat := geo.MetersToLatDeg(radiusM)
	// A circle spans the most degrees of longitude at its edge furthest from
	// the equator, so size the box there. Capped short of the pole, where a
	// degree of longitude has no width.
	dLng := geo.MetersToLngDeg(radiusM, math.Min(math.Abs(lat)+dLat, 89))

	rows, err := p.pool.Query(ctx, nearbyInBox,
		int16(model.StatusAvailable), lat-dLat, lat+dLat, lng-dLng, lng+dLng)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []model.NearbyDriver{}
	for rows.Next() {
		var d model.NearbyDriver
		if err := rows.Scan(&d.DriverID, &d.Lat, &d.Lng); err != nil {
			return nil, err
		}
		d.DistanceM = geo.HaversineM(lat, lng, d.Lat, d.Lng)
		if d.DistanceM <= radiusM {
			out = append(out, d)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	sort.Slice(out, func(i, j int) bool { return out[i].DistanceM < out[j].DistanceM })
	if len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

func (p *Postgres) Close() error {
	p.pool.Close()
	return nil
}

// Connection-pool metrics. A request that cannot get a connection waits in
// line, and that wait is invisible in database-side statistics.
var (
	poolMaxDesc = prometheus.NewDesc("pgpool_max_conns",
		"Most connections the pool is allowed to open.", nil, nil)
	poolAcquiredDesc = prometheus.NewDesc("pgpool_acquired_conns",
		"Connections checked out by requests right now.", nil, nil)
	poolIdleDesc = prometheus.NewDesc("pgpool_idle_conns",
		"Open connections waiting for work right now.", nil, nil)
	poolAcquiresDesc = prometheus.NewDesc("pgpool_acquires_total",
		"Connection checkouts since start.", nil, nil)
	poolWaitedDesc = prometheus.NewDesc("pgpool_acquires_waited_total",
		"Checkouts that had to wait because every connection was busy.", nil, nil)
	poolWaitSecondsDesc = prometheus.NewDesc("pgpool_acquire_wait_seconds_total",
		"Total time requests have spent obtaining a connection.", nil, nil)
)

// Describe implements prometheus.Collector.
func (p *Postgres) Describe(ch chan<- *prometheus.Desc) {
	ch <- poolMaxDesc
	ch <- poolAcquiredDesc
	ch <- poolIdleDesc
	ch <- poolAcquiresDesc
	ch <- poolWaitedDesc
	ch <- poolWaitSecondsDesc
}

// Collect implements prometheus.Collector.
func (p *Postgres) Collect(ch chan<- prometheus.Metric) {
	s := p.pool.Stat()
	ch <- prometheus.MustNewConstMetric(poolMaxDesc, prometheus.GaugeValue, float64(s.MaxConns()))
	ch <- prometheus.MustNewConstMetric(poolAcquiredDesc, prometheus.GaugeValue, float64(s.AcquiredConns()))
	ch <- prometheus.MustNewConstMetric(poolIdleDesc, prometheus.GaugeValue, float64(s.IdleConns()))
	ch <- prometheus.MustNewConstMetric(poolAcquiresDesc, prometheus.CounterValue, float64(s.AcquireCount()))
	ch <- prometheus.MustNewConstMetric(poolWaitedDesc, prometheus.CounterValue, float64(s.EmptyAcquireCount()))
	ch <- prometheus.MustNewConstMetric(poolWaitSecondsDesc, prometheus.CounterValue, s.AcquireDuration().Seconds())
}
