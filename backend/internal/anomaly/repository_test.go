package anomaly

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/JuanJoseFlorez/bia-energy-mvp/backend/internal/platform/database/dbtest"
)

func ids(items []Item) []int64 {
	out := make([]int64, len(items))
	for i, it := range items {
		out[i] = it.ID
	}
	return out
}

func eventID(t *testing.T, tx pgx.Tx, meterID string) int64 {
	t.Helper()
	var id int64
	if err := tx.QueryRow(context.Background(), `SELECT id FROM events WHERE meter_id = $1`, meterID).Scan(&id); err != nil {
		t.Fatalf("event of %s: %v", meterID, err)
	}
	return id
}

func setEvidence(t *testing.T, tx pgx.Tx, anomalyID int64, evidence string) {
	t.Helper()
	if _, err := tx.Exec(context.Background(), `UPDATE anomalies SET evidence = $2, explanation = 'fixture explanation' WHERE id = $1`,
		anomalyID, evidence); err != nil {
		t.Fatalf("set evidence: %v", err)
	}
}

func mustList(t *testing.T, repo *PostgresRepository, runID int64, f Filters) []Item {
	t.Helper()
	items, err := repo.List(context.Background(), runID, f)
	if err != nil {
		t.Fatalf("List(%d, %+v) error = %v", runID, f, err)
	}
	return items
}

func TestIntegrationLatestCompletedRun(t *testing.T) {
	tx := dbtest.Tx(t)
	repo := NewRepository(tx)
	ctx := context.Background()

	if id, err := repo.LatestCompletedRun(ctx); err != nil || id != nil {
		t.Fatalf("LatestCompletedRun() on empty = %v, %v; want nil", id, err)
	}

	finished := dbtest.FixtureFinishedAt
	first := dbtest.InsertRun(t, tx, "COMPLETED", dbtest.FixtureStartedAt, &finished)
	second := dbtest.InsertRun(t, tx, "COMPLETED", dbtest.FixtureStartedAt, &finished) // same finished_at → higher id wins
	later := finished.Add(time.Hour)
	dbtest.InsertRun(t, tx, "FAILED", dbtest.FixtureStartedAt, &later)
	dbtest.InsertRun(t, tx, "RUNNING", later, nil)

	id, err := repo.LatestCompletedRun(ctx)
	if err != nil || id == nil || *id != second {
		t.Errorf("LatestCompletedRun() = %v, %v; want %d (first %d)", id, err, second, first)
	}
	if ok, err := repo.RunExists(ctx, first); err != nil || !ok {
		t.Errorf("RunExists(%d) = %v, %v; want true", first, ok, err)
	}
	if ok, err := repo.RunExists(ctx, 999999); err != nil || ok {
		t.Errorf("RunExists(missing) = %v, %v; want false", ok, err)
	}
}

func TestIntegrationListOrderAndFilters(t *testing.T) {
	tx := dbtest.Tx(t)
	repo := NewRepository(tx)
	f := dbtest.SeedFixture(t, tx)
	// Two more anomalies on M-109: same priority as M-112 (tie broken by id) and a lower one.
	tie := dbtest.InsertAnomaly(t, tx, f.RunID, "M-109", "DATA_QUALITY", "HIGH", 0.5, 2)
	last := dbtest.InsertAnomaly(t, tx, f.RunID, "M-109", "EXPLAINABLE_ANOMALY", "LOW", 0.5, 9)

	items := mustList(t, repo, f.RunID, Filters{})

	want := []int64{f.AnomalyIDs["M-109"], f.AnomalyIDs["M-112"], tie, f.AnomalyIDs["M-104"], f.AnomalyIDs["M-106"], last}
	if got := ids(items); !slices.Equal(got, want) {
		t.Errorf("order = %v, want %v", got, want)
	}
	top := items[0]
	if !top.IsAnomaly || top.AnalysisID != f.RunID || top.MeterID != "M-109" || *top.Type != "REAL_ANOMALY" ||
		*top.Confidence != 0.96 || *top.Priority != 1 || top.Status != StatusPending || top.DetectedAt == nil {
		t.Errorf("top item = %+v", top)
	}
	if fp := items[4]; fp.IsAnomaly {
		t.Errorf("FALSE_POSITIVE item anomaly = true, want false")
	}

	filtered := mustList(t, repo, f.RunID, Filters{Type: "DATA_QUALITY", Severity: "HIGH"})
	if got := ids(filtered); !slices.Equal(got, []int64{f.AnomalyIDs["M-112"], tie}) {
		t.Errorf("filtered = %v", got)
	}
	if got := mustList(t, repo, f.RunID, Filters{Status: StatusResolved}); len(got) != 0 {
		t.Errorf("status filter = %v, want none", ids(got))
	}
	other := dbtest.InsertRun(t, tx, "FAILED", dbtest.FixtureStartedAt, nil)
	if got := mustList(t, repo, other, Filters{}); len(got) != 0 {
		t.Errorf("other run items = %v, want none", ids(got))
	}
}

func TestIntegrationGetDetail(t *testing.T) {
	tx := dbtest.Tx(t)
	repo := NewRepository(tx)
	ctx := context.Background()
	f := dbtest.SeedFixture(t, tx)
	id := f.AnomalyIDs["M-109"]
	ev := eventID(t, tx, "M-109")
	other := eventID(t, tx, "M-104") // another meter's event id must not leak in
	setEvidence(t, tx, id, fmt.Sprintf(`{"change_start": "2026-09-12T14:00:00Z", "related_event_ids": [%d, %d]}`, ev, other))

	d, err := repo.Get(ctx, id)
	if err != nil || d == nil {
		t.Fatalf("Get() = %v, %v", d, err)
	}
	if d.ID != id || d.Explanation == nil || *d.Explanation != "fixture explanation" {
		t.Errorf("detail = %+v", d)
	}
	var evidence map[string]any
	if err := json.Unmarshal(d.Evidence, &evidence); err != nil || evidence["change_start"] != "2026-09-12T14:00:00Z" {
		t.Errorf("evidence = %s (%v)", d.Evidence, err)
	}

	events, err := repo.RelatedEvents(ctx, id)
	if err != nil || len(events) != 1 || events[0].ID != ev || events[0].MeterID != "M-109" || *events[0].Type != "UNKNOWN" {
		t.Errorf("RelatedEvents() = %+v, %v; want only event %d", events, err, ev)
	}

	if d, err := repo.Get(ctx, 999999); err != nil || d != nil {
		t.Errorf("Get(missing) = %v, %v; want nil", d, err)
	}
}

func TestIntegrationRelatedEventsWithoutEvidence(t *testing.T) {
	tx := dbtest.Tx(t)
	repo := NewRepository(tx)
	f := dbtest.SeedFixture(t, tx)
	setEvidence(t, tx, f.AnomalyIDs["M-104"], `{"related_event_ids": null}`)

	for _, meter := range []string{"M-104", "M-106"} { // JSON null, SQL NULL
		events, err := repo.RelatedEvents(context.Background(), f.AnomalyIDs[meter])
		if err != nil || events == nil || len(events) != 0 {
			t.Errorf("RelatedEvents(%s) = %v, %v; want empty", meter, events, err)
		}
	}
}

func TestIntegrationReadingsBounds(t *testing.T) {
	tx := dbtest.Tx(t)
	repo := NewRepository(tx)

	first, last, err := repo.ReadingsBounds(context.Background(), "M-109")
	if err != nil || first == nil || last == nil || !last.After(*first) {
		t.Fatalf("ReadingsBounds(M-109) = %v, %v, %v", first, last, err)
	}
	first, last, err = repo.ReadingsBounds(context.Background(), "M-999")
	if err != nil || first != nil || last != nil {
		t.Errorf("ReadingsBounds(unknown) = %v, %v, %v; want nil, nil", first, last, err)
	}
}

func TestIntegrationUpdateStatusCompareAndSet(t *testing.T) {
	tx := dbtest.Tx(t)
	repo := NewRepository(tx)
	ctx := context.Background()
	id := dbtest.SeedFixture(t, tx).AnomalyIDs["M-109"]

	if ok, err := repo.UpdateStatus(ctx, id, StatusPending, StatusInvestigating); err != nil || !ok {
		t.Fatalf("UpdateStatus(PENDING→INVESTIGATING) = %v, %v; want true", ok, err)
	}
	if ok, err := repo.UpdateStatus(ctx, id, StatusPending, StatusDismissed); err != nil || ok {
		t.Errorf("UpdateStatus with stale from = %v, %v; want false", ok, err)
	}
	d, err := repo.Get(ctx, id)
	if err != nil || d.Status != StatusInvestigating {
		t.Errorf("status = %v (%v), want INVESTIGATING", d, err)
	}
}
