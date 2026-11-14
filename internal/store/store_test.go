package store

import (
	"context"
	"os"
	"slices"
	"testing"
	"time"

	"ridehail/internal/geo"
	"ridehail/internal/model"
)

const (
	centreLat = 12.9716
	centreLng = 77.5946
)

// northOf builds a ping the given distance due north of the centre.
func northOf(id int64, metres float64, st model.Status) model.Ping {
	return model.Ping{
		DriverID: id,
		Lat:      centreLat + geo.MetersToLatDeg(metres),
		Lng:      centreLng,
		Status:   st,
		TsMs:     1_700_000_000_000,
	}
}

func ids(ds []model.NearbyDriver) []int64 {
	out := make([]int64, len(ds))
	for i, d := range ds {
		out[i] = d.DriverID
	}
	return out
}

// storeContract is the behaviour every Store must have, whatever is underneath.
// open must return an empty store.
func storeContract(t *testing.T, open func(t *testing.T) Store) {
	ctx := context.Background()

	record := func(t *testing.T, s Store, pings ...model.Ping) {
		t.Helper()
		for _, p := range pings {
			if err := s.RecordPing(ctx, p); err != nil {
				t.Fatalf("RecordPing(driver %d): %v", p.DriverID, err)
			}
		}
	}

	t.Run("nearby returns available drivers closest first", func(t *testing.T) {
		s := open(t)
		record(t, s,
			northOf(1, 900, model.StatusAvailable),
			northOf(2, 100, model.StatusAvailable),
			northOf(3, 5000, model.StatusAvailable), // outside the radius
			northOf(4, 50, model.StatusOnTrip),      // closest, but busy
			northOf(5, 400, model.StatusAvailable),
		)

		got, err := s.Nearby(ctx, centreLat, centreLng, 1000, 10)
		if err != nil {
			t.Fatal(err)
		}
		if want := []int64{2, 5, 1}; !slices.Equal(ids(got), want) {
			t.Errorf("Nearby = %v, want %v", ids(got), want)
		}

		got, err = s.Nearby(ctx, centreLat, centreLng, 1000, 2)
		if err != nil {
			t.Fatal(err)
		}
		if want := []int64{2, 5}; !slices.Equal(ids(got), want) {
			t.Errorf("Nearby with limit 2 = %v, want %v", ids(got), want)
		}
	})

	t.Run("distance is measured, not just boxed", func(t *testing.T) {
		// 900 m north and 900 m east is about 1273 m away: inside the square
		// that encloses a 1000 m circle, but outside the circle itself.
		s := open(t)
		corner := northOf(1, 900, model.StatusAvailable)
		corner.Lng += geo.MetersToLngDeg(900, centreLat)
		record(t, s, corner)

		got, err := s.Nearby(ctx, centreLat, centreLng, 1000, 10)
		if err != nil {
			t.Fatal(err)
		}
		if len(got) != 0 {
			t.Errorf("driver %.0f m away was returned for a 1000 m search", got[0].DistanceM)
		}
	})

	t.Run("only the latest ping counts", func(t *testing.T) {
		s := open(t)
		record(t, s,
			northOf(1, 100, model.StatusAvailable),
			northOf(1, 3000, model.StatusAvailable),
		)

		got, err := s.Nearby(ctx, centreLat, centreLng, 1000, 10)
		if err != nil {
			t.Fatal(err)
		}
		if len(got) != 0 {
			t.Errorf("driver moved 3 km away but Nearby still returned %v", ids(got))
		}
	})

	t.Run("no drivers yields an empty slice, not nil", func(t *testing.T) {
		got, err := open(t).Nearby(ctx, centreLat, centreLng, 1000, 10)
		if err != nil {
			t.Fatal(err)
		}
		if got == nil {
			t.Error("Nearby returned nil; the API would encode that as JSON null instead of []")
		}
	})
}

func TestMemory(t *testing.T) {
	storeContract(t, func(*testing.T) Store { return NewMemory() })
}

// openTestPostgres connects to TEST_DATABASE_URL and empties the tables, or
// skips the test when the variable is not set.
func openTestPostgres(t *testing.T) *Postgres {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set TEST_DATABASE_URL to run against a real PostgreSQL")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	pg, err := OpenPostgres(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { pg.Close() })
	if _, err := pg.pool.Exec(ctx, `TRUNCATE pings, driver_latest`); err != nil {
		t.Fatal(err)
	}
	return pg
}

func TestPostgres(t *testing.T) {
	storeContract(t, func(t *testing.T) Store { return openTestPostgres(t) })
}

func TestPostgresKeepsEveryPingAsHistory(t *testing.T) {
	pg := openTestPostgres(t)
	ctx := context.Background()
	for _, metres := range []float64{100, 200, 300} {
		if err := pg.RecordPing(ctx, northOf(1, metres, model.StatusAvailable)); err != nil {
			t.Fatal(err)
		}
	}

	var history, latest int
	if err := pg.pool.QueryRow(ctx, `SELECT count(*) FROM pings WHERE driver_id = 1`).Scan(&history); err != nil {
		t.Fatal(err)
	}
	if err := pg.pool.QueryRow(ctx, `SELECT count(*) FROM driver_latest WHERE driver_id = 1`).Scan(&latest); err != nil {
		t.Fatal(err)
	}
	if history != 3 || latest != 1 {
		t.Errorf("after 3 pings: %d history rows and %d latest rows, want 3 and 1", history, latest)
	}
}
