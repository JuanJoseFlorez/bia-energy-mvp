package analysis

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/JuanJoseFlorez/bia-energy-mvp/backend/internal/platform/apperr"
	"github.com/JuanJoseFlorez/bia-energy-mvp/backend/internal/platform/database/dbtest"
)

func mustCreate(t *testing.T, repo *PostgresRepository) Run {
	t.Helper()
	run, err := repo.CreateRun(context.Background())
	if err != nil {
		t.Fatalf("CreateRun() error = %v", err)
	}
	return run
}

func mustGet(t *testing.T, repo *PostgresRepository, id int64) Run {
	t.Helper()
	run, err := repo.Get(context.Background(), id)
	if err != nil || run == nil {
		t.Fatalf("Get(%d) = %v, %v", id, run, err)
	}
	return *run
}

// runningRun creates a run and moves it to RUNNING.
func runningRun(t *testing.T, repo *PostgresRepository) Run {
	t.Helper()
	run := mustCreate(t, repo)
	if err := repo.MarkRunning(context.Background(), run.ID); err != nil {
		t.Fatalf("MarkRunning() error = %v", err)
	}
	return run
}

func sampleResult() Result {
	start := time.Date(2026, 9, 12, 14, 0, 0, 0, time.UTC)
	metrics := make([]MetricResult, len(dbtest.MeterIDs))
	for i, id := range dbtest.MeterIDs {
		metrics[i] = MetricResult{MeterID: id, CurrentKWh: 1000, BaselineKWh: 1000, VariationPct: 0}
	}
	metrics[8] = MetricResult{MeterID: "M-109", CurrentKWh: 2208.4, BaselineKWh: 1052.7, VariationPct: 109.8, ChangeStart: &start}
	return Result{
		Metrics: metrics,
		Anomalies: []AnomalyResult{
			{MeterID: "M-109", Type: "REAL_ANOMALY", Severity: "HIGH", Confidence: 0.96, Priority: 1,
				Reason: "reason", Explanation: "explanation", RecommendedAction: "action", ExplanationSource: "llm",
				Evidence: json.RawMessage(`{"related_event_ids": [3], "change_start": "2026-09-12T14:00:00Z"}`)},
			{MeterID: "M-106", Type: "FALSE_POSITIVE", Severity: "LOW", Confidence: 0.7, Priority: 2,
				Reason: "r", Explanation: "e", RecommendedAction: "a", ExplanationSource: "template",
				Evidence: json.RawMessage(`{}`)},
		},
	}
}

func countRows(t *testing.T, tx pgx.Tx, table string, runID int64) int {
	t.Helper()
	var n int
	if err := tx.QueryRow(context.Background(), `SELECT COUNT(*) FROM `+table+` WHERE analysis_id = $1`, runID).Scan(&n); err != nil {
		t.Fatalf("count %s: %v", table, err)
	}
	return n
}

func TestIntegrationCreateRunSingleActive(t *testing.T) {
	tx := dbtest.Tx(t)
	repo := NewRepository(tx)
	ctx := context.Background()

	first := mustCreate(t, repo)
	if first.Status != StatusPending || first.CurrentStep != nil || first.Summary != nil || first.Error != nil {
		t.Errorf("new run = %+v, want PENDING with null step/summary/error", first)
	}

	_, err := repo.CreateRun(ctx)
	if !errors.Is(err, apperr.ErrConflict) {
		t.Fatalf("second CreateRun() error = %v, want ErrConflict", err)
	}

	if err := repo.Fail(ctx, first.ID, "boom"); err != nil {
		t.Fatalf("Fail() error = %v", err)
	}
	if second := mustCreate(t, repo); second.ID == first.ID {
		t.Errorf("new run reused id %d", first.ID)
	}
}

func TestIntegrationRunLifecycle(t *testing.T) {
	tx := dbtest.Tx(t)
	repo := NewRepository(tx)
	ctx := context.Background()
	run := runningRun(t, repo)

	if err := repo.SetStep(ctx, run.ID, "DETECTION"); err != nil {
		t.Fatalf("SetStep() error = %v", err)
	}
	if got := mustGet(t, repo, run.ID); got.Status != StatusRunning || got.CurrentStep == nil || *got.CurrentStep != "DETECTION" {
		t.Fatalf("running run = %+v", got)
	}

	res := sampleResult()
	if err := repo.Complete(ctx, run.ID, res, Summary{AnomaliesDetected: 2, HighPriority: 1}); err != nil {
		t.Fatalf("Complete() error = %v", err)
	}

	got := mustGet(t, repo, run.ID)
	if got.Status != StatusCompleted || got.FinishedAt == nil {
		t.Errorf("completed run = %+v", got)
	}
	if got.Summary == nil || *got.Summary != (Summary{AnomaliesDetected: 2, HighPriority: 1}) {
		t.Errorf("summary = %+v, want {2 1}", got.Summary)
	}
	if n := countRows(t, tx, "meter_metrics", run.ID); n != 12 {
		t.Errorf("meter_metrics rows = %d, want 12", n)
	}

	var (
		reason, explanation, action, source string
		priority                            int
		changeStart                         *time.Time
		relatedEvent                        int
	)
	err := tx.QueryRow(ctx, `
SELECT a.reason, a.explanation, a.recommended_action, a.explanation_source, a.priority,
       (a.evidence->'related_event_ids'->>0)::int, mm.change_start
FROM anomalies a JOIN meter_metrics mm ON mm.analysis_id = a.analysis_id AND mm.meter_id = a.meter_id
WHERE a.analysis_id = $1 AND a.meter_id = 'M-109'`, run.ID).
		Scan(&reason, &explanation, &action, &source, &priority, &relatedEvent, &changeStart)
	if err != nil {
		t.Fatalf("read M-109 anomaly: %v", err)
	}
	if reason != "reason" || explanation != "explanation" || action != "action" || source != "llm" || priority != 1 || relatedEvent != 3 {
		t.Errorf("anomaly = %q %q %q %q %d %d", reason, explanation, action, source, priority, relatedEvent)
	}
	if want := time.Date(2026, 9, 12, 14, 0, 0, 0, time.UTC); changeStart == nil || !changeStart.Equal(want) {
		t.Errorf("change_start = %v, want %v", changeStart, want)
	}
}

func TestIntegrationCompleteIsAtomic(t *testing.T) {
	tx := dbtest.Tx(t)
	repo := NewRepository(tx)
	run := runningRun(t, repo)
	res := sampleResult()
	res.Anomalies[1].MeterID = "M-999" // unknown meter → FK violation after the first anomaly

	if err := repo.Complete(context.Background(), run.ID, res, Summary{}); err == nil {
		t.Fatal("Complete() error = nil, want FK error")
	}

	if got := mustGet(t, repo, run.ID); got.Status != StatusRunning {
		t.Errorf("status = %s, want RUNNING", got.Status)
	}
	if n := countRows(t, tx, "meter_metrics", run.ID) + countRows(t, tx, "anomalies", run.ID); n != 0 {
		t.Errorf("rows left after failed Complete = %d, want 0", n)
	}
}

func TestIntegrationInactiveRunIsNotRevived(t *testing.T) {
	tx := dbtest.Tx(t)
	repo := NewRepository(tx)
	ctx := context.Background()
	run := runningRun(t, repo)
	if err := repo.Fail(ctx, run.ID, "interrupted: backend restarted"); err != nil {
		t.Fatalf("Fail() error = %v", err)
	}

	if err := repo.SetStep(ctx, run.ID, "EVENTS"); !errors.Is(err, errRunNotActive) {
		t.Errorf("SetStep() error = %v, want errRunNotActive", err)
	}
	if err := repo.Complete(ctx, run.ID, sampleResult(), Summary{}); !errors.Is(err, errRunNotActive) {
		t.Errorf("Complete() error = %v, want errRunNotActive", err)
	}
	if err := repo.MarkRunning(ctx, run.ID); !errors.Is(err, errRunNotActive) {
		t.Errorf("MarkRunning() error = %v, want errRunNotActive", err)
	}
	if err := repo.Fail(ctx, run.ID, "other"); err != nil {
		t.Errorf("Fail() on failed run error = %v, want nil", err)
	}

	got := mustGet(t, repo, run.ID)
	if got.Status != StatusFailed || got.Error == nil || *got.Error != "interrupted: backend restarted" || got.CurrentStep != nil {
		t.Errorf("run = %+v, want untouched FAILED", got)
	}
	if n := countRows(t, tx, "anomalies", run.ID); n != 0 {
		t.Errorf("anomalies = %d, want 0", n)
	}
}

func TestIntegrationSweepStale(t *testing.T) {
	tx := dbtest.Tx(t)
	repo := NewRepository(tx)
	ctx := context.Background()
	stale := runningRun(t, repo)
	if _, err := tx.Exec(ctx, `UPDATE analysis_runs SET updated_at = now() - interval '10 minutes' WHERE id = $1`, stale.ID); err != nil {
		t.Fatalf("age run: %v", err)
	}
	done := dbtest.InsertRun(t, tx, "COMPLETED", dbtest.FixtureStartedAt, &dbtest.FixtureFinishedAt)
	if _, err := tx.Exec(ctx, `UPDATE analysis_runs SET updated_at = now() - interval '1 day' WHERE id = $1`, done); err != nil {
		t.Fatalf("age completed run: %v", err)
	}

	n, err := repo.SweepStale(ctx, 5*time.Minute)
	if err != nil || n != 1 {
		t.Fatalf("SweepStale() = %d, %v; want 1", n, err)
	}
	if got := mustGet(t, repo, stale.ID); got.Status != StatusFailed || got.Error == nil || got.FinishedAt == nil {
		t.Errorf("stale run = %+v, want FAILED with error", got)
	}
	if got := mustGet(t, repo, done); got.Status != StatusCompleted {
		t.Errorf("completed run status = %s, want COMPLETED", got.Status)
	}

	fresh := mustCreate(t, repo)
	if n, err := repo.SweepStale(ctx, 5*time.Minute); err != nil || n != 0 {
		t.Errorf("SweepStale() on fresh run = %d, %v; want 0", n, err)
	}
	if got := mustGet(t, repo, fresh.ID); got.Status != StatusPending {
		t.Errorf("fresh run status = %s, want PENDING", got.Status)
	}
}

func TestIntegrationGetAndLatest(t *testing.T) {
	tx := dbtest.Tx(t)
	repo := NewRepository(tx)
	ctx := context.Background()

	if run, err := repo.Latest(ctx); err != nil || run != nil {
		t.Fatalf("Latest() on empty = %v, %v; want nil", run, err)
	}
	if run, err := repo.Get(ctx, 999999); err != nil || run != nil {
		t.Fatalf("Get(missing) = %v, %v; want nil", run, err)
	}

	sameStart := dbtest.FixtureStartedAt
	older := dbtest.InsertRun(t, tx, "FAILED", sameStart.Add(-time.Hour), nil)
	first := dbtest.InsertRun(t, tx, "COMPLETED", sameStart, &dbtest.FixtureFinishedAt)
	second := dbtest.InsertRun(t, tx, "FAILED", sameStart, nil)

	latest, err := repo.Latest(ctx)
	if err != nil || latest == nil || latest.ID != second {
		t.Errorf("Latest() = %+v, %v; want id %d (tie on started_at broken by id; older %d, first %d)", latest, err, second, older, first)
	}
}
