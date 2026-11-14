package sim

import (
	"testing"
	"time"

	"ridehail/internal/geo"
)

func TestFleetStaysInsideTheCity(t *testing.T) {
	f := NewFleet(500, 42, geo.Bengaluru)
	for step := 0; step < 200; step++ {
		for i := 0; i < f.Len(); i++ {
			lat, lng := f.Step(i, 4*time.Second)
			if !geo.Bengaluru.Contains(lat, lng) {
				t.Fatalf("driver %d left the city at step %d: (%v, %v)", i, step, lat, lng)
			}
		}
	}
}

func TestFleetIsDeterministicForASeed(t *testing.T) {
	a := NewFleet(50, 7, geo.Bengaluru)
	b := NewFleet(50, 7, geo.Bengaluru)
	for i := 0; i < a.Len(); i++ {
		aLat, aLng := a.Step(i, time.Second)
		bLat, bLng := b.Step(i, time.Second)
		if aLat != bLat || aLng != bLng {
			t.Fatalf("driver %d diverged: (%v, %v) vs (%v, %v)", i, aLat, aLng, bLat, bLng)
		}
	}
}

func TestFleetActuallyMoves(t *testing.T) {
	f := NewFleet(1, 3, geo.Bengaluru)
	startLat, startLng := f.drivers[0].lat, f.drivers[0].lng
	var moved float64
	for range 10 {
		lat, lng := f.Step(0, 4*time.Second)
		moved = geo.HaversineM(startLat, startLng, lat, lng)
	}
	if moved == 0 {
		t.Error("driver did not move after ten steps")
	}
}
