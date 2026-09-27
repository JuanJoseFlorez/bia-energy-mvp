// Package dbtest provides PostgreSQL helpers for integration tests.
// Import it only from _test.go files: it depends on package testing.
package dbtest

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// MeterIDs lists the 12 meters seeded by db-init/init.sql.
var MeterIDs = []string{
	"M-101", "M-102", "M-103", "M-104", "M-105", "M-106",
	"M-107", "M-108", "M-109", "M-110", "M-111", "M-112",
}

// Timestamps of the fixture's completed analysis run.
var (
	FixtureStartedAt  = time.Date(2026, 9, 15, 8, 0, 0, 0, time.UTC)
	FixtureFinishedAt = time.Date(2026, 9, 15, 8, 5, 0, 0, time.UTC)
)

// Fixture identifies the rows inserted by SeedFixture.
type Fixture struct {
	RunID      int64
	AnomalyIDs map[string]int64 // by meter_id
}

// Tx opens a transaction on TEST_DATABASE_URL, clears the analysis tables inside it and
// rolls everything back when the test ends. Seed meters, readings and events stay untouched.
// The test is skipped when TEST_DATABASE_URL is not set.
func Tx(t *testing.T) pgx.Tx {
	t.Helper()
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}
	ctx := context.Background()

	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(pool.Close)

	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	// Registered after pool.Close, so it runs first (cleanups are LIFO).
	t.Cleanup(func() { _ = tx.Rollback(ctx) })

	if _, err := tx.Exec(ctx, `DELETE FROM anomalies; DELETE FROM meter_metrics; DELETE FROM analysis_runs`); err != nil {
		t.Fatalf("clear analysis tables: %v", err)
	}
	return tx
}

// InsertRun inserts an analysis run and returns its id.
func InsertRun(t *testing.T, tx pgx.Tx, status string, startedAt time.Time, finishedAt *time.Time) int64 {
	t.Helper()
	var id int64
	err := tx.QueryRow(context.Background(),
		`INSERT INTO analysis_runs (status, started_at, finished_at) VALUES ($1, $2, $3) RETURNING id`,
		status, startedAt, finishedAt,
	).Scan(&id)
	if err != nil {
		t.Fatalf("insert run: %v", err)
	}
	return id
}

// InsertMetrics inserts one meter_metrics row without a change point.
func InsertMetrics(t *testing.T, tx pgx.Tx, runID int64, meterID string, currentKWh, baselineKWh, variationPct float64) {
	t.Helper()
	_, err := tx.Exec(context.Background(),
		`INSERT INTO meter_metrics (analysis_id, meter_id, current_kwh, baseline_kwh, variation_pct)
		 VALUES ($1, $2, $3, $4, $5)`,
		runID, meterID, currentKWh, baselineKWh, variationPct,
	)
	if err != nil {
		t.Fatalf("insert metrics for %s: %v", meterID, err)
	}
}

// InsertAnomaly inserts an anomaly with fixed reason/action/status and returns its id.
func InsertAnomaly(t *testing.T, tx pgx.Tx, runID int64, meterID, anomalyType, severity string, confidence float64, priority int) int64 {
	t.Helper()
	var id int64
	err := tx.QueryRow(context.Background(),
		`INSERT INTO anomalies (analysis_id, meter_id, detected_at, type, severity, confidence, priority,
		                        reason, recommended_action, status)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, 'fixture reason', 'fixture action', 'PENDING')
		 RETURNING id`,
		runID, meterID, FixtureFinishedAt, anomalyType, severity, confidence, priority,
	).Scan(&id)
	if err != nil {
		t.Fatalf("insert anomaly for %s: %v", meterID, err)
	}
	return id
}

// SeedFixture inserts one completed run with metrics for every meter and the four
// anomaly cases described by the technical brief.
func SeedFixture(t *testing.T, tx pgx.Tx) Fixture {
	t.Helper()
	finished := FixtureFinishedAt
	runID := InsertRun(t, tx, "COMPLETED", FixtureStartedAt, &finished)

	variations := map[string]float64{"M-109": 110.5, "M-104": 47.5}
	for _, id := range MeterIDs {
		v, ok := variations[id]
		if !ok {
			v = 0.5
		}
		const baseline = 1000.0
		InsertMetrics(t, tx, runID, id, baseline*(1+v/100), baseline, v)
	}

	f := Fixture{RunID: runID, AnomalyIDs: map[string]int64{}}
	f.AnomalyIDs["M-109"] = InsertAnomaly(t, tx, runID, "M-109", "REAL_ANOMALY", "HIGH", 0.96, 1)
	f.AnomalyIDs["M-112"] = InsertAnomaly(t, tx, runID, "M-112", "DATA_QUALITY", "HIGH", 0.92, 2)
	f.AnomalyIDs["M-104"] = InsertAnomaly(t, tx, runID, "M-104", "EXPLAINABLE_ANOMALY", "MEDIUM", 0.90, 3)
	f.AnomalyIDs["M-106"] = InsertAnomaly(t, tx, runID, "M-106", "FALSE_POSITIVE", "LOW", 0.70, 4)
	return f
}
