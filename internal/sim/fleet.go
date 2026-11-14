// Package sim models the fleet of drivers that the load generator pretends to be.
package sim

import (
	"math"
	"math/rand/v2"
	"time"

	"ridehail/internal/geo"
)

// hotspotShare is the fraction of drivers that start clustered around a
// hotspot instead of spread evenly over the city.
const hotspotShare = 0.6

// hotspots are where drivers bunch up. Real fleets are never evenly spread,
// and that skew is what makes partitioning hard in stage 4.
var hotspots = [...]struct{ lat, lng, sigmaM float64 }{
	{12.9716, 77.5946, 2500}, // MG Road, city centre
	{12.9352, 77.6245, 2000}, // Koramangala
	{12.9698, 77.7400, 2000}, // Whitefield
	{13.0358, 77.5970, 1500}, // Hebbal
	{13.1986, 77.7066, 800},  // airport: small area, very dense
}

type driver struct {
	lat, lng float64
	heading  float64 // radians clockwise from north
	speed    float64 // metres per second
}

// Fleet is a set of drivers doing a random walk inside a bounding box.
//
// A Fleet is not safe for concurrent use; one goroutine owns it.
type Fleet struct {
	box     geo.BBox
	rng     *rand.Rand
	drivers []driver
}

// NewFleet places n drivers in the box. The same seed always produces the same
// fleet and the same movements.
func NewFleet(n int, seed uint64, box geo.BBox) *Fleet {
	f := &Fleet{
		box:     box,
		rng:     rand.New(rand.NewPCG(seed, seed^0x9e3779b97f4a7c15)),
		drivers: make([]driver, n),
	}
	for i := range f.drivers {
		lat, lng := f.startPosition()
		f.drivers[i] = driver{
			lat:     lat,
			lng:     lng,
			heading: f.rng.Float64() * 2 * math.Pi,
			speed:   5 + f.rng.Float64()*9, // 18 to 50 km/h
		}
	}
	return f
}

func (f *Fleet) startPosition() (lat, lng float64) {
	if f.rng.Float64() < hotspotShare {
		h := hotspots[f.rng.IntN(len(hotspots))]
		// A few tries to land inside the box, then fall through to uniform.
		for range 8 {
			lat = h.lat + geo.MetersToLatDeg(f.rng.NormFloat64()*h.sigmaM)
			lng = h.lng + geo.MetersToLngDeg(f.rng.NormFloat64()*h.sigmaM, h.lat)
			if f.box.Contains(lat, lng) {
				return lat, lng
			}
		}
	}
	lat = f.box.MinLat + f.rng.Float64()*(f.box.MaxLat-f.box.MinLat)
	lng = f.box.MinLng + f.rng.Float64()*(f.box.MaxLng-f.box.MinLng)
	return lat, lng
}

// Len is the number of drivers.
func (f *Fleet) Len() int { return len(f.drivers) }

// Step moves driver i forward by dt of travel and returns the new position.
// A driver that would leave the box turns around and waits out this step.
func (f *Fleet) Step(i int, dt time.Duration) (lat, lng float64) {
	d := &f.drivers[i]
	d.heading += f.rng.NormFloat64() * 0.3

	dist := d.speed * dt.Seconds()
	lat = d.lat + geo.MetersToLatDeg(dist*math.Cos(d.heading))
	lng = d.lng + geo.MetersToLngDeg(dist*math.Sin(d.heading), d.lat)
	if !f.box.Contains(lat, lng) {
		d.heading += math.Pi
		return d.lat, d.lng
	}
	d.lat, d.lng = lat, lng
	return lat, lng
}
