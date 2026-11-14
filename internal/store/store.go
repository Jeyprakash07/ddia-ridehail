// Package store is where driver positions live.
//
// The interface is deliberately small so the storage engine underneath can be
// swapped stage by stage without touching the HTTP layer.
package store

import (
	"context"
	"sort"
	"sync"

	"ridehail/internal/geo"
	"ridehail/internal/model"
)

// Store records pings and answers proximity queries.
type Store interface {
	RecordPing(ctx context.Context, p model.Ping) error
	// Nearby returns available drivers within radiusM of the point, closest
	// first, at most limit of them.
	Nearby(ctx context.Context, lat, lng, radiusM float64, limit int) ([]model.NearbyDriver, error)
	Close() error
}

// Memory keeps each driver's latest ping in one map behind one lock.
//
// It is the stage 0 baseline. It forgets history, loses everything on restart,
// and scans every driver for each Nearby call. Whichever ping arrives last
// wins, even if the phone sent it earlier.
type Memory struct {
	mu     sync.RWMutex
	latest map[int64]model.Ping
}

func NewMemory() *Memory {
	return &Memory{latest: make(map[int64]model.Ping)}
}

func (m *Memory) RecordPing(_ context.Context, p model.Ping) error {
	m.mu.Lock()
	m.latest[p.DriverID] = p
	m.mu.Unlock()
	return nil
}

func (m *Memory) Nearby(_ context.Context, lat, lng, radiusM float64, limit int) ([]model.NearbyDriver, error) {
	out := []model.NearbyDriver{}

	m.mu.RLock()
	for _, p := range m.latest {
		if p.Status != model.StatusAvailable {
			continue
		}
		if d := geo.HaversineM(lat, lng, p.Lat, p.Lng); d <= radiusM {
			out = append(out, model.NearbyDriver{DriverID: p.DriverID, Lat: p.Lat, Lng: p.Lng, DistanceM: d})
		}
	}
	m.mu.RUnlock()

	sort.Slice(out, func(i, j int) bool { return out[i].DistanceM < out[j].DistanceM })
	if len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

func (m *Memory) Close() error { return nil }
