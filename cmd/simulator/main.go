// Command simulator drives a fleet of virtual drivers around Bengaluru and
// reports their positions to the ingest service.
//
// It is deliberately the load generator most people write first: a fixed pool
// of workers, each timing its own request. Exercise 1 is about what that
// design cannot see.
package main

import (
	"bytes"
	"cmp"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"maps"
	"net/http"
	"os"
	"os/signal"
	"slices"
	"sync"
	"time"

	"ridehail/internal/geo"
	"ridehail/internal/loadgen"
	"ridehail/internal/model"
	"ridehail/internal/sim"
)

func main() {
	target := flag.String("target", "http://127.0.0.1:8080", "ingest base URL")
	drivers := flag.Int("drivers", 10000, "number of simulated drivers")
	interval := flag.Duration("ping-interval", 4*time.Second, "how often each driver reports its position")
	duration := flag.Duration("duration", 30*time.Second, "how long to generate load")
	workers := flag.Int("workers", 64, "maximum concurrent in-flight requests")
	seed := flag.Uint64("seed", 1, "seed for fleet placement and movement")
	stallAt := flag.Duration("stall-at", 0, "freeze the server this far into the run (0 = never; needs ingest -debug-endpoints)")
	stallFor := flag.Duration("stall-for", 2*time.Second, "how long the injected freeze lasts")
	flag.Parse()

	if *drivers < 1 || *workers < 1 || *interval <= 0 || *duration <= 0 {
		fmt.Fprintln(os.Stderr, "drivers, workers, ping-interval and duration must all be positive")
		os.Exit(2)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	client := &http.Client{
		Timeout: 10 * time.Second,
		Transport: &http.Transport{
			// One warm connection per worker. The default keeps only two idle
			// connections per host, which at this request rate would open and
			// close thousands of sockets a second.
			MaxIdleConns:        *workers,
			MaxIdleConnsPerHost: *workers,
			MaxConnsPerHost:     *workers,
			IdleConnTimeout:     90 * time.Second,
		},
	}

	if err := waitReady(ctx, client, *target); err != nil {
		fmt.Fprintln(os.Stderr, "ingest is not reachable:", err)
		os.Exit(1)
	}

	fleet := sim.NewFleet(*drivers, *seed, geo.Bengaluru)
	rec := &loadgen.Recorder{}
	rate := float64(*drivers) / interval.Seconds()
	scheduled := int(rate * duration.Seconds())
	fmt.Printf("%d drivers, one ping every %s: %.0f pings/s for %s\n", *drivers, *interval, rate, *duration)

	// Workers: take a ping, send it, time it.
	jobs := make(chan model.Ping, *workers)
	pingURL := *target + "/v1/pings"
	var wg sync.WaitGroup
	for i := 0; i < *workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for p := range jobs {
				start := time.Now()
				err := sendPing(client, pingURL, p)
				rec.Record(time.Since(start), err)
			}
		}()
	}

	if *stallAt > 0 {
		go injectStall(ctx, *target, *stallAt, *stallFor)
	}

	started := time.Now()
	progressCtx, stopProgress := context.WithCancel(ctx)
	go progress(progressCtx, rec, started)

	generate(ctx, fleet, jobs, rate, *interval, *duration)
	close(jobs)
	wg.Wait()
	stopProgress()
	elapsed := time.Since(started)

	s := rec.Summary()
	fmt.Println()
	fmt.Printf("scheduled     %d pings\n", scheduled)
	fmt.Printf("sent          %d pings in %s (%.0f/s)\n", s.Count, elapsed.Round(time.Millisecond), float64(s.Count)/elapsed.Seconds())
	fmt.Printf("errors        %d\n", s.Errors)
	printErrorKinds(s.ErrorKinds, 5)
	fmt.Printf("latency mean  %s\n", s.Mean.Round(time.Microsecond))
	fmt.Printf("latency max   %s\n", s.Max.Round(time.Microsecond))
	fmt.Println("percentiles   not implemented yet (exercise 1a)")
}

// generate hands pings to the workers at the target rate until the duration is
// up. Every tick it works out how many pings should have gone out by now and
// emits the ones that have not.
//
// It gives up one second after the run should have ended, so a server that
// cannot keep up shows as a shortfall instead of a run that never finishes.
func generate(ctx context.Context, fleet *sim.Fleet, jobs chan<- model.Ping, rate float64, interval, duration time.Duration) {
	const tick = 10 * time.Millisecond

	ctx, cancel := context.WithTimeout(ctx, duration+time.Second)
	defer cancel()
	ticker := time.NewTicker(tick)
	defer ticker.Stop()

	start := time.Now()
	emitted, next := 0, 0
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}

		elapsed := min(time.Since(start), duration)
		due := int(rate * elapsed.Seconds())
		for ; emitted < due; emitted++ {
			lat, lng := fleet.Step(next, interval)
			p := model.Ping{
				DriverID: int64(next + 1),
				Lat:      lat,
				Lng:      lng,
				Status:   model.StatusAvailable,
				TsMs:     time.Now().UnixMilli(),
			}
			select {
			case jobs <- p: // waits here whenever every worker is busy
			case <-ctx.Done():
				return
			}
			next = (next + 1) % fleet.Len()
		}
		if elapsed == duration {
			return
		}
	}
}

func sendPing(client *http.Client, url string, p model.Ping) error {
	body, err := json.Marshal(p)
	if err != nil {
		return err
	}
	resp, err := client.Post(url, "application/json", bytes.NewReader(body))
	if err != nil {
		return err
	}
	// Drain and close so the connection goes back to the pool.
	_, _ = io.Copy(io.Discard, resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("unexpected status %d", resp.StatusCode)
	}
	return nil
}

// waitReady polls /healthz for up to 15 seconds so the simulator can be
// started at the same moment as the server.
func waitReady(ctx context.Context, client *http.Client, target string) error {
	deadline := time.Now().Add(15 * time.Second)
	for {
		resp, err := client.Get(target + "/healthz")
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return nil
			}
			err = fmt.Errorf("/healthz returned %d", resp.StatusCode)
		}
		if time.Now().After(deadline) {
			return err
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(250 * time.Millisecond):
		}
	}
}

// injectStall asks the server to freeze. It uses its own connection so the
// request is not queued behind the workers it is about to block.
func injectStall(ctx context.Context, target string, at, length time.Duration) {
	select {
	case <-ctx.Done():
		return
	case <-time.After(at):
	}
	fmt.Printf("  >> freezing the server for %s\n", length)
	resp, err := http.Post(fmt.Sprintf("%s/debug/stall?ms=%d", target, length.Milliseconds()), "", nil)
	if err != nil {
		fmt.Fprintln(os.Stderr, "stall request failed:", err)
		return
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		fmt.Fprintf(os.Stderr, "stall request returned %d (is ingest running with -debug-endpoints?)\n", resp.StatusCode)
	}
}

// printErrorKinds lists the most common failure causes, most frequent first.
func printErrorKinds(kinds map[string]int, limit int) {
	names := slices.Collect(maps.Keys(kinds))
	slices.SortFunc(names, func(a, b string) int {
		if c := cmp.Compare(kinds[b], kinds[a]); c != 0 {
			return c
		}
		return cmp.Compare(a, b)
	})
	for _, name := range names[:min(limit, len(names))] {
		fmt.Printf("  %7d  %s\n", kinds[name], name)
	}
}

func progress(ctx context.Context, rec *loadgen.Recorder, started time.Time) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			sent, errs := rec.Counts()
			fmt.Printf("  %3.0fs  sent %d  errors %d\n", time.Since(started).Seconds(), sent, errs)
		}
	}
}
