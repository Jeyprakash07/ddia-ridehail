package main

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"ridehail/internal/geo"
	"ridehail/internal/model"
	"ridehail/internal/store"
)

const (
	maxPingBodyBytes = 4096
	defaultRadiusM   = 2000.0
	maxRadiusM       = 20000.0
	defaultLimit     = 10
	maxLimit         = 100
	maxStallMs       = 30000
)

type api struct {
	store  store.Store
	logger *slog.Logger

	// gate lets /debug/stall freeze every request at once, the way a long
	// garbage-collection pause or a stuck disk would. Requests hold the read
	// side; the stall takes the write side.
	gate sync.RWMutex

	reqDuration   *prometheus.HistogramVec
	storeDuration *prometheus.HistogramVec
	pings         prometheus.Counter

	lastErrorLog atomic.Int64 // unix second of the most recent logged store error
}

func newHandler(st store.Store, reg *prometheus.Registry, logger *slog.Logger, debug bool) http.Handler {
	buckets := prometheus.ExponentialBuckets(0.0001, 2, 16) // 0.1 ms to 3.3 s
	a := &api{
		store:  st,
		logger: logger,
		reqDuration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "http_request_duration_seconds",
			Help:    "Time from a request reaching its handler to the response being written.",
			Buckets: buckets,
		}, []string{"route", "code"}),
		storeDuration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "store_op_duration_seconds",
			Help:    "Time spent inside one storage call, including any wait for a connection.",
			Buckets: buckets,
		}, []string{"op"}),
		pings: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "pings_ingested_total",
			Help: "Location pings accepted and stored.",
		}),
	}
	reg.MustRegister(
		collectors.NewGoCollector(),
		collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
		a.reqDuration,
		a.storeDuration,
		a.pings,
	)
	// Stores that have something to report (a connection pool, say) expose it.
	if c, ok := st.(prometheus.Collector); ok {
		reg.MustRegister(c)
	}

	mux := http.NewServeMux()
	mux.Handle("POST /v1/pings", a.instrument("pings", a.handlePing))
	mux.Handle("GET /v1/drivers/nearby", a.instrument("nearby", a.handleNearby))
	mux.Handle("GET /metrics", promhttp.HandlerFor(reg, promhttp.HandlerOpts{}))
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	if debug {
		mux.HandleFunc("POST /debug/stall", a.handleStall)
	}
	return mux
}

// handlerFunc writes a response and reports the status code it used.
type handlerFunc func(w http.ResponseWriter, r *http.Request) int

// instrument times a handler and holds the stall gate while it runs. The clock
// starts before the gate, so time spent frozen counts as request time.
func (a *api) instrument(route string, h handlerFunc) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		a.gate.RLock()
		code := h(w, r)
		a.gate.RUnlock()
		a.reqDuration.WithLabelValues(route, strconv.Itoa(code)).Observe(time.Since(start).Seconds())
	})
}

func (a *api) handlePing(w http.ResponseWriter, r *http.Request) int {
	var p model.Ping
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxPingBodyBytes)).Decode(&p); err != nil {
		return writeError(w, http.StatusBadRequest, "body must be a JSON ping")
	}
	if p.DriverID <= 0 {
		return writeError(w, http.StatusBadRequest, "driver_id must be positive")
	}
	if !geo.ValidLatLng(p.Lat, p.Lng) {
		return writeError(w, http.StatusBadRequest, "lat and lng must be valid coordinates")
	}
	if p.Status > model.StatusOnTrip {
		return writeError(w, http.StatusBadRequest, "status must be 0 (offline), 1 (available) or 2 (on trip)")
	}

	start := time.Now()
	err := a.store.RecordPing(r.Context(), p)
	a.storeDuration.WithLabelValues("record_ping").Observe(time.Since(start).Seconds())
	if err != nil {
		a.logStoreError("record_ping", err)
		return writeError(w, http.StatusInternalServerError, "could not store ping")
	}
	a.pings.Inc()
	w.WriteHeader(http.StatusNoContent)
	return http.StatusNoContent
}

func (a *api) handleNearby(w http.ResponseWriter, r *http.Request) int {
	q := r.URL.Query()

	lat, errLat := strconv.ParseFloat(q.Get("lat"), 64)
	lng, errLng := strconv.ParseFloat(q.Get("lng"), 64)
	if errLat != nil || errLng != nil || !geo.ValidLatLng(lat, lng) {
		return writeError(w, http.StatusBadRequest, "lat and lng are required and must be valid coordinates")
	}

	radius := defaultRadiusM
	if s := q.Get("radius_m"); s != "" {
		v, err := strconv.ParseFloat(s, 64)
		if err != nil || !(v > 0 && v <= maxRadiusM) {
			return writeError(w, http.StatusBadRequest, "radius_m must be above 0 and at most 20000")
		}
		radius = v
	}

	limit := defaultLimit
	if s := q.Get("limit"); s != "" {
		v, err := strconv.Atoi(s)
		if err != nil || v < 1 || v > maxLimit {
			return writeError(w, http.StatusBadRequest, "limit must be between 1 and 100")
		}
		limit = v
	}

	start := time.Now()
	drivers, err := a.store.Nearby(r.Context(), lat, lng, radius, limit)
	a.storeDuration.WithLabelValues("nearby").Observe(time.Since(start).Seconds())
	if err != nil {
		a.logStoreError("nearby", err)
		return writeError(w, http.StatusInternalServerError, "could not look up drivers")
	}
	return writeJSON(w, http.StatusOK, map[string]any{"drivers": drivers})
}

// handleStall blocks every instrumented request for the requested time.
func (a *api) handleStall(w http.ResponseWriter, r *http.Request) {
	ms, err := strconv.Atoi(r.URL.Query().Get("ms"))
	if err != nil || ms < 1 || ms > maxStallMs {
		writeError(w, http.StatusBadRequest, "ms must be between 1 and 30000")
		return
	}
	a.gate.Lock()
	time.Sleep(time.Duration(ms) * time.Millisecond)
	a.gate.Unlock()
	w.WriteHeader(http.StatusNoContent)
}

// logStoreError logs at most one storage failure per second. When the database
// is down every request fails, and a line per request would bury the cause.
func (a *api) logStoreError(op string, err error) {
	now := time.Now().Unix()
	last := a.lastErrorLog.Load()
	if last == now || !a.lastErrorLog.CompareAndSwap(last, now) {
		return
	}
	a.logger.Error("store call failed", "op", op, "err", err)
}

func writeJSON(w http.ResponseWriter, code int, v any) int {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
	return code
}

func writeError(w http.ResponseWriter, code int, msg string) int {
	return writeJSON(w, code, map[string]string{"error": msg})
}
