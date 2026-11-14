// Package model holds the wire and domain types shared by every service.
package model

// Status is what a driver is doing at the moment of a ping.
type Status uint8

const (
	StatusOffline Status = iota
	StatusAvailable
	StatusOnTrip
)

// Ping is one location report from a driver's phone.
//
// TsMs comes from the phone's clock, not ours. Stage 6 (chapter 8) is where
// trusting it comes back to bite us.
type Ping struct {
	DriverID int64   `json:"driver_id"`
	Lat      float64 `json:"lat"`
	Lng      float64 `json:"lng"`
	Status   Status  `json:"status"`
	TsMs     int64   `json:"ts_ms"`
}

// NearbyDriver is a driver's last known position and its distance from the
// point that was asked about.
type NearbyDriver struct {
	DriverID  int64   `json:"driver_id"`
	Lat       float64 `json:"lat"`
	Lng       float64 `json:"lng"`
	DistanceM float64 `json:"distance_m"`
}
