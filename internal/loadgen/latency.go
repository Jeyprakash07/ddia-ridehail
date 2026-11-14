// Package loadgen holds the measurement side of the load generator.
package loadgen

import (
	"errors"
	"maps"
	"net"
	"net/url"
	"slices"
	"sync"
	"time"
)

// Recorder collects one latency sample per request. Safe for concurrent use.
type Recorder struct {
	mu         sync.Mutex
	samples    []time.Duration
	errors     int
	errorKinds map[string]int
}

// Record adds a sample. Failed requests are counted as errors, and their
// latency is kept too: a timeout is the slowest response there is.
func (r *Recorder) Record(d time.Duration, err error) {
	var kind string
	if err != nil {
		kind = errorKind(err)
	}

	r.mu.Lock()
	r.samples = append(r.samples, d)
	if err != nil {
		r.errors++
		if r.errorKinds == nil {
			r.errorKinds = make(map[string]int)
		}
		r.errorKinds[kind]++
	}
	r.mu.Unlock()
}

// errorKind reduces an error to a label that is identical for every occurrence
// of the same failure. Network errors normally carry the URL and both socket
// addresses, which would make each one look unique.
func errorKind(err error) string {
	var opErr *net.OpError
	if errors.As(err, &opErr) {
		return opErr.Op + ": " + opErr.Err.Error()
	}
	var urlErr *url.Error
	if errors.As(err, &urlErr) {
		return urlErr.Err.Error()
	}
	return err.Error()
}

// Counts returns how many requests have finished and how many of them failed.
func (r *Recorder) Counts() (requests, errors int) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.samples), r.errors
}

// Summary is what a run looked like once it is over.
type Summary struct {
	Count  int
	Errors int
	// ErrorKinds counts failures by cause, e.g. "dial: connectex: ...".
	ErrorKinds map[string]int
	Mean       time.Duration
	Max        time.Duration
	// Sorted holds every sample in ascending order, ready for Percentile.
	Sorted []time.Duration
}

// Summary sorts a copy of the samples and computes the aggregate figures.
func (r *Recorder) Summary() Summary {
	r.mu.Lock()
	sorted := slices.Clone(r.samples)
	errs := r.errors
	kinds := maps.Clone(r.errorKinds)
	r.mu.Unlock()

	slices.Sort(sorted)
	s := Summary{Count: len(sorted), Errors: errs, ErrorKinds: kinds, Sorted: sorted}
	if len(sorted) == 0 {
		return s
	}
	var total time.Duration
	for _, d := range sorted {
		total += d
	}
	s.Mean = total / time.Duration(len(sorted))
	s.Max = sorted[len(sorted)-1]
	return s
}

// Percentile returns the p-th percentile (0 < p <= 100) of samples that are
// already sorted ascending, using the nearest-rank method: the smallest sample
// such that at least p percent of all samples are less than or equal to it.
// An empty input returns 0.
//
// EXERCISE 1a: this is yours to write. latency_test.go says what "done" means.
func Percentile(sorted []time.Duration, p float64) time.Duration {
	return 0
}
