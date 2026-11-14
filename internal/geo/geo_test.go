package geo

import (
	"math"
	"testing"
)

func TestHaversineM(t *testing.T) {
	if d := HaversineM(12.97, 77.59, 12.97, 77.59); d != 0 {
		t.Errorf("distance from a point to itself = %v, want 0", d)
	}

	// One degree of latitude is about 111.2 km anywhere on a sphere.
	if d := HaversineM(12, 77, 13, 77); math.Abs(d-111_195) > 50 {
		t.Errorf("one degree of latitude = %.0f m, want about 111195 m", d)
	}

	// MG Road to the airport is about 28 km as the crow flies.
	if d := HaversineM(12.9716, 77.5946, 13.1986, 77.7066); d < 27_500 || d > 28_500 {
		t.Errorf("MG Road to airport = %.0f m, want about 28000 m", d)
	}
}

func TestMetersToDegreesRoundTrip(t *testing.T) {
	const lat, lng = 12.97, 77.59

	north := HaversineM(lat, lng, lat+MetersToLatDeg(1000), lng)
	if math.Abs(north-1000) > 1 {
		t.Errorf("1000 m north measured back as %.2f m", north)
	}

	east := HaversineM(lat, lng, lat, lng+MetersToLngDeg(1000, lat))
	if math.Abs(east-1000) > 1 {
		t.Errorf("1000 m east measured back as %.2f m", east)
	}
}

func TestValidLatLng(t *testing.T) {
	cases := []struct {
		lat, lng float64
		want     bool
	}{
		{12.97, 77.59, true},
		{-90, 180, true},
		{90.01, 0, false},
		{0, -180.01, false},
		{math.NaN(), 0, false},
		{0, math.NaN(), false},
	}
	for _, c := range cases {
		if got := ValidLatLng(c.lat, c.lng); got != c.want {
			t.Errorf("ValidLatLng(%v, %v) = %v, want %v", c.lat, c.lng, got, c.want)
		}
	}
}
