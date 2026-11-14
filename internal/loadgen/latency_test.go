package loadgen

import (
	"errors"
	"net"
	"net/url"
	"testing"
	"time"
)

func ms(n int) time.Duration { return time.Duration(n) * time.Millisecond }

// TestPercentile fails until exercise 1a is done.
func TestPercentile(t *testing.T) {
	hundred := make([]time.Duration, 100) // 1ms, 2ms, ... 100ms
	for i := range hundred {
		hundred[i] = ms(i + 1)
	}
	four := []time.Duration{ms(1), ms(2), ms(3), ms(4)}

	cases := []struct {
		name string
		in   []time.Duration
		p    float64
		want time.Duration
	}{
		{"empty input", nil, 99, 0},
		{"single sample", []time.Duration{ms(7)}, 50, ms(7)},
		{"p50 of 1..100", hundred, 50, ms(50)},
		{"p95 of 1..100", hundred, 95, ms(95)},
		{"p99 of 1..100", hundred, 99, ms(99)},
		{"p99.9 of 1..100 rounds up to the max", hundred, 99.9, ms(100)},
		{"p100 is the max", hundred, 100, ms(100)},
		{"p50 of four samples", four, 50, ms(2)},
		{"p75 of four samples", four, 75, ms(3)},
		{"p99 of four samples is the max", four, 99, ms(4)},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := Percentile(c.in, c.p); got != c.want {
				t.Errorf("Percentile(p=%v) = %v, want %v", c.p, got, c.want)
			}
		})
	}
}

func TestRecorderSummary(t *testing.T) {
	var r Recorder
	r.Record(ms(30), nil)
	r.Record(ms(10), nil)
	r.Record(ms(20), errors.New("boom"))

	if n, errs := r.Counts(); n != 3 || errs != 1 {
		t.Errorf("Counts = (%d, %d), want (3, 1)", n, errs)
	}

	s := r.Summary()
	if s.Count != 3 || s.Errors != 1 {
		t.Errorf("Count, Errors = %d, %d, want 3, 1", s.Count, s.Errors)
	}
	if s.Mean != ms(20) {
		t.Errorf("Mean = %v, want 20ms", s.Mean)
	}
	if s.Max != ms(30) {
		t.Errorf("Max = %v, want 30ms", s.Max)
	}
	if len(s.Sorted) != 3 || s.Sorted[0] != ms(10) || s.Sorted[2] != ms(30) {
		t.Errorf("Sorted = %v, want ascending [10ms 20ms 30ms]", s.Sorted)
	}
	if len(s.ErrorKinds) != 1 || s.ErrorKinds["boom"] != 1 {
		t.Errorf("ErrorKinds = %v, want map[boom:1]", s.ErrorKinds)
	}
}

func TestErrorKindDropsAddressesAndURLs(t *testing.T) {
	refused := errors.New("connectex: actively refused")
	dial := func(localPort int) error {
		return &url.Error{
			Op:  "Post",
			URL: "http://127.0.0.1:8080/v1/pings",
			Err: &net.OpError{
				Op:     "dial",
				Net:    "tcp",
				Source: &net.TCPAddr{IP: net.IPv4(127, 0, 0, 1), Port: localPort},
				Addr:   &net.TCPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 8080},
				Err:    refused,
			},
		}
	}

	a, b := errorKind(dial(50001)), errorKind(dial(50002))
	if a != b {
		t.Errorf("same failure on two sockets got different labels: %q vs %q", a, b)
	}
	if want := "dial: connectex: actively refused"; a != want {
		t.Errorf("errorKind = %q, want %q", a, want)
	}

	timeout := &url.Error{Op: "Post", URL: "http://x/y", Err: errors.New("deadline exceeded")}
	if got := errorKind(timeout); got != "deadline exceeded" {
		t.Errorf("errorKind of a URL error = %q, want the inner message only", got)
	}

	if got := errorKind(errors.New("unexpected status 500")); got != "unexpected status 500" {
		t.Errorf("errorKind of a plain error = %q, want it unchanged", got)
	}
}

func TestEmptyRecorderSummary(t *testing.T) {
	var r Recorder
	if s := r.Summary(); s.Count != 0 || s.Mean != 0 || s.Max != 0 {
		t.Errorf("empty Summary = %+v, want zero values", s)
	}
}
