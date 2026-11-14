// Command ingest accepts driver location pings and answers "who is near here?".
//
// Stage 0 is one stateless process in front of one store: an in-memory map for
// local runs, or a single PostgreSQL database in the cluster. It is the
// baseline we measure before making anything better.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"ridehail/internal/store"
)

func main() {
	addr := flag.String("addr", ":8080", "listen address")
	storeKind := flag.String("store", "memory", "where positions are kept: memory or postgres (reads DATABASE_URL)")
	debug := flag.Bool("debug-endpoints", false, "expose POST /debug/stall for fault-injection demos")
	flag.Parse()

	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	st, err := openStore(*storeKind)
	if err != nil {
		logger.Error("could not open store", "store", *storeKind, "err", err)
		os.Exit(1)
	}

	srv := &http.Server{
		Addr:              *addr,
		Handler:           newHandler(st, prometheus.NewRegistry(), logger, *debug),
		ReadHeaderTimeout: 5 * time.Second,
		IdleTimeout:       2 * time.Minute,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// On a signal, stop accepting connections and let in-flight requests finish.
	drained := make(chan struct{})
	go func() {
		defer close(drained)
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := srv.Shutdown(shutdownCtx); err != nil {
			logger.Error("shutdown did not finish cleanly", "err", err)
		}
	}()

	logger.Info("ingest listening", "addr", *addr, "store", *storeKind, "debug_endpoints", *debug)
	if err := srv.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
		logger.Error("server failed", "err", err)
		os.Exit(1)
	}
	<-drained
	_ = st.Close()
	logger.Info("ingest stopped")
}

// openStore builds the store named by the -store flag. The database address
// comes from the environment only, so its password never shows up in the
// process's command line.
func openStore(kind string) (store.Store, error) {
	switch kind {
	case "memory":
		return store.NewMemory(), nil
	case "postgres":
		dsn := os.Getenv("DATABASE_URL")
		if dsn == "" {
			return nil, errors.New("-store=postgres needs DATABASE_URL to be set")
		}
		// Generous, because in the cluster the database may start after us.
		ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
		defer cancel()
		return store.OpenPostgres(ctx, dsn)
	default:
		return nil, fmt.Errorf("unknown store %q: want memory or postgres", kind)
	}
}
