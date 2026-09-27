package dbtest

import (
	"context"
	"testing"
)

func TestIntegrationSeedFixture(t *testing.T) {
	tx := Tx(t)
	ctx := context.Background()

	f := SeedFixture(t, tx)

	var metrics, anomalies int
	if err := tx.QueryRow(ctx, `SELECT COUNT(*) FROM meter_metrics WHERE analysis_id = $1`, f.RunID).Scan(&metrics); err != nil {
		t.Fatalf("count metrics: %v", err)
	}
	if err := tx.QueryRow(ctx, `SELECT COUNT(*) FROM anomalies WHERE analysis_id = $1`, f.RunID).Scan(&anomalies); err != nil {
		t.Fatalf("count anomalies: %v", err)
	}
	if metrics != 12 {
		t.Errorf("metrics rows = %d, want 12", metrics)
	}
	if anomalies != 4 || len(f.AnomalyIDs) != 4 {
		t.Errorf("anomalies rows = %d (ids %d), want 4", anomalies, len(f.AnomalyIDs))
	}
}

func TestIntegrationTxStartsEmpty(t *testing.T) {
	tx := Tx(t)

	var runs int
	if err := tx.QueryRow(context.Background(), `SELECT COUNT(*) FROM analysis_runs`).Scan(&runs); err != nil {
		t.Fatalf("count runs: %v", err)
	}
	if runs != 0 {
		t.Errorf("analysis_runs = %d, want 0 inside Tx", runs)
	}
}
