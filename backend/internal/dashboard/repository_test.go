package dashboard

import (
	"context"
	"testing"
	"time"

	"github.com/JuanJoseFlorez/bia-energy-mvp/backend/internal/platform/database/dbtest"
)

func TestIntegrationSummaryNoRuns(t *testing.T) {
	tx := dbtest.Tx(t)
	ctx := context.Background()

	s, err := NewRepository(tx).Summary(ctx)
	if err != nil {
		t.Fatalf("Summary() error = %v", err)
	}

	var wantTotal float64
	if err := tx.QueryRow(ctx, `SELECT ROUND(SUM(consumption_kwh), 2) FROM readings`).Scan(&wantTotal); err != nil {
		t.Fatalf("sum: %v", err)
	}
	if s.MetersCount != 12 {
		t.Errorf("MetersCount = %d, want 12", s.MetersCount)
	}
	if s.TotalConsumptionKWh != wantTotal {
		t.Errorf("TotalConsumptionKWh = %v, want %v", s.TotalConsumptionKWh, wantTotal)
	}
	from := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 9, 14, 23, 0, 0, 0, time.UTC)
	if s.Period == nil || !s.Period.From.Equal(from) || !s.Period.To.Equal(to) {
		t.Errorf("Period = %+v, want %v → %v", s.Period, from, to)
	}
	if s.LastAnalysis != nil || s.Anomalies != nil {
		t.Errorf("LastAnalysis/Anomalies = %+v/%+v, want nil", s.LastAnalysis, s.Anomalies)
	}
}

func TestIntegrationSummaryFixture(t *testing.T) {
	tx := dbtest.Tx(t)
	f := dbtest.SeedFixture(t, tx)

	s, err := NewRepository(tx).Summary(context.Background())
	if err != nil {
		t.Fatalf("Summary() error = %v", err)
	}
	a := s.Anomalies
	if a == nil {
		t.Fatal("Anomalies = nil")
	}
	if a.AnalysisID != f.RunID || !a.AnalysisFinishedAt.Equal(dbtest.FixtureFinishedAt) {
		t.Errorf("analysis = %d @ %v, want %d @ %v", a.AnalysisID, a.AnalysisFinishedAt, f.RunID, dbtest.FixtureFinishedAt)
	}
	if a.Detected != 4 || a.HighPriority != 2 {
		t.Errorf("detected/high = %d/%d, want 4/2", a.Detected, a.HighPriority)
	}
	// Fixture confidences 0.96, 0.92, 0.90, 0.70 → mean 0.87.
	if a.AvgConfidence == nil || *a.AvgConfidence != 0.87 {
		t.Errorf("AvgConfidence = %v, want 0.87", a.AvgConfidence)
	}
	for _, typ := range []string{"REAL_ANOMALY", "EXPLAINABLE_ANOMALY", "FALSE_POSITIVE", "DATA_QUALITY"} {
		if a.ByType[typ] != 1 {
			t.Errorf("ByType[%s] = %d, want 1", typ, a.ByType[typ])
		}
	}
	if s.LastAnalysis == nil || s.LastAnalysis.ID != f.RunID || s.LastAnalysis.Status != "COMPLETED" {
		t.Errorf("LastAnalysis = %+v, want the fixture run", s.LastAnalysis)
	}
}

func TestIntegrationSummaryNewerRunning(t *testing.T) {
	tx := dbtest.Tx(t)
	f := dbtest.SeedFixture(t, tx)
	running := dbtest.InsertRun(t, tx, "RUNNING", dbtest.FixtureFinishedAt.Add(time.Hour), nil)

	s, err := NewRepository(tx).Summary(context.Background())
	if err != nil {
		t.Fatalf("Summary() error = %v", err)
	}
	if s.LastAnalysis == nil || s.LastAnalysis.ID != running || s.LastAnalysis.Status != "RUNNING" || s.LastAnalysis.FinishedAt != nil {
		t.Errorf("LastAnalysis = %+v, want RUNNING run %d", s.LastAnalysis, running)
	}
	if s.Anomalies == nil || s.Anomalies.AnalysisID != f.RunID {
		t.Errorf("Anomalies = %+v, want stats of completed run %d", s.Anomalies, f.RunID)
	}
}

func TestIntegrationSummaryCompletedWithoutAnomalies(t *testing.T) {
	tx := dbtest.Tx(t)
	finished := dbtest.FixtureFinishedAt
	run := dbtest.InsertRun(t, tx, "COMPLETED", dbtest.FixtureStartedAt, &finished)

	s, err := NewRepository(tx).Summary(context.Background())
	if err != nil {
		t.Fatalf("Summary() error = %v", err)
	}
	a := s.Anomalies
	if a == nil || a.AnalysisID != run || a.Detected != 0 || a.HighPriority != 0 || a.AvgConfidence != nil {
		t.Fatalf("Anomalies = %+v, want zero stats for run %d", a, run)
	}
	if len(a.ByType) != 4 {
		t.Errorf("ByType = %v, want 4 keys", a.ByType)
	}
	for typ, n := range a.ByType {
		if n != 0 {
			t.Errorf("ByType[%s] = %d, want 0", typ, n)
		}
	}
}
