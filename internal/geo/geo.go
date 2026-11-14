// Package geo has the small amount of spherical maths the project needs.
package geo

import "math"

const earthRadiusM = 6_371_000.0

// BBox is a latitude/longitude rectangle.
type BBox struct {
	MinLat, MinLng, MaxLat, MaxLng float64
}

// Bengaluru is where the simulated fleet drives: about 41 km north to south
// (far enough to take in the airport) and 32 km east to west.
var Bengaluru = BBox{MinLat: 12.85, MinLng: 77.45, MaxLat: 13.22, MaxLng: 77.75}

// Contains reports whether the point lies inside the box, edges included.
func (b BBox) Contains(lat, lng float64) bool {
	return lat >= b.MinLat && lat <= b.MaxLat && lng >= b.MinLng && lng <= b.MaxLng
}

// ValidLatLng reports whether the pair is a real point on Earth. NaN fails
// every comparison, so it is rejected too.
func ValidLatLng(lat, lng float64) bool {
	return lat >= -90 && lat <= 90 && lng >= -180 && lng <= 180
}

// HaversineM returns the great-circle distance between two points in metres.
func HaversineM(lat1, lng1, lat2, lng2 float64) float64 {
	const rad = math.Pi / 180
	sinLat := math.Sin((lat2 - lat1) * rad / 2)
	sinLng := math.Sin((lng2 - lng1) * rad / 2)
	a := sinLat*sinLat + math.Cos(lat1*rad)*math.Cos(lat2*rad)*sinLng*sinLng
	return 2 * earthRadiusM * math.Asin(math.Min(1, math.Sqrt(a)))
}

// MetersToLatDeg converts a north-south distance to degrees of latitude.
func MetersToLatDeg(m float64) float64 {
	return m / earthRadiusM * 180 / math.Pi
}

// MetersToLngDeg converts an east-west distance to degrees of longitude at the
// given latitude. Lines of longitude converge towards the poles, so the same
// distance covers more degrees the further you are from the equator.
func MetersToLngDeg(m, atLat float64) float64 {
	return m / (earthRadiusM * math.Cos(atLat*math.Pi/180)) * 180 / math.Pi
}
