package main

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/prometheus/client_golang/prometheus"

	"ridehail/internal/model"
	"ridehail/internal/store"
)

func newTestServer(t *testing.T, debug bool) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(newHandler(store.NewMemory(), prometheus.NewRegistry(), slog.New(slog.DiscardHandler), debug))
	t.Cleanup(srv.Close)
	return srv
}

func postPing(t *testing.T, srv *httptest.Server, body string) *http.Response {
	t.Helper()
	resp, err := http.Post(srv.URL+"/v1/pings", "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { resp.Body.Close() })
	return resp
}

func TestPingThenNearby(t *testing.T) {
	srv := newTestServer(t, false)

	resp := postPing(t, srv, `{"driver_id":7,"lat":12.9716,"lng":77.5946,"status":1,"ts_ms":1}`)
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("POST /v1/pings = %d, want 204", resp.StatusCode)
	}

	resp, err := http.Get(srv.URL + "/v1/drivers/nearby?lat=12.9716&lng=77.5946&radius_m=500")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET nearby = %d, want 200", resp.StatusCode)
	}
	var out struct {
		Drivers []model.NearbyDriver `json:"drivers"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	if len(out.Drivers) != 1 || out.Drivers[0].DriverID != 7 {
		t.Errorf("nearby drivers = %+v, want exactly driver 7", out.Drivers)
	}
}

func TestPingRejectsBadInput(t *testing.T) {
	srv := newTestServer(t, false)
	cases := map[string]string{
		"not JSON":         `{`,
		"zero driver id":   `{"driver_id":0,"lat":12.97,"lng":77.59,"status":1}`,
		"latitude > 90":    `{"driver_id":1,"lat":91,"lng":77.59,"status":1}`,
		"longitude < -180": `{"driver_id":1,"lat":12.97,"lng":-181,"status":1}`,
		"unknown status":   `{"driver_id":1,"lat":12.97,"lng":77.59,"status":9}`,
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			if resp := postPing(t, srv, body); resp.StatusCode != http.StatusBadRequest {
				t.Errorf("status = %d, want 400", resp.StatusCode)
			}
		})
	}
}

func TestNearbyRejectsBadQuery(t *testing.T) {
	srv := newTestServer(t, false)
	for _, query := range []string{
		"",
		"lat=12.97",
		"lat=abc&lng=77.59",
		"lat=12.97&lng=77.59&radius_m=0",
		"lat=12.97&lng=77.59&radius_m=NaN",
		"lat=12.97&lng=77.59&limit=1000",
	} {
		resp, err := http.Get(srv.URL + "/v1/drivers/nearby?" + query)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusBadRequest {
			t.Errorf("query %q: status = %d, want 400", query, resp.StatusCode)
		}
	}
}

func TestMetricsCountIngestedPings(t *testing.T) {
	srv := newTestServer(t, false)
	postPing(t, srv, `{"driver_id":1,"lat":12.97,"lng":77.59,"status":1}`)
	postPing(t, srv, `{"driver_id":2,"lat":12.97,"lng":77.59,"status":1}`)

	resp, err := http.Get(srv.URL + "/metrics")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(body), "pings_ingested_total 2\n") {
		t.Error("metrics output does not report pings_ingested_total 2")
	}
	if !strings.Contains(string(body), `http_request_duration_seconds_count{code="204",route="pings"} 2`) {
		t.Error("metrics output does not report two timed 204 responses on the pings route")
	}
}

func TestStallEndpointOnlyExistsInDebugMode(t *testing.T) {
	off := newTestServer(t, false)
	resp, err := http.Post(off.URL+"/debug/stall?ms=1", "", nil)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("without -debug-endpoints: status = %d, want 404", resp.StatusCode)
	}

	on := newTestServer(t, true)
	resp, err = http.Post(on.URL+"/debug/stall?ms=1", "", nil)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		t.Errorf("with -debug-endpoints: status = %d, want 204", resp.StatusCode)
	}
}
