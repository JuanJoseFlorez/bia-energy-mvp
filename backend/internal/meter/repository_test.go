package meter

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/JuanJoseFlorez/bia-energy-mvp/backend/internal/platform/database/dbtest"
)

var allParams = ListParams{Sort: SortMeterID, Limit: 50}

func mustList(t *testing.T, repo *PostgresRepository, p ListParams) ([]Meter, int) {
	t.Helper()
	items, total, err := repo.List(context.Background(), p)
	if err != nil {
		t.Fatalf("List(%+v) error = %v", p, err)
	}
	return items, total
}

func byMeterID(items []Meter) map[string]Meter {
	out := make(map[string]Meter, len(items))
	for _, m := range items {
		out[m.MeterID] = m
	}
	return out
}

func meterIDs(items []Meter) []string {
	out := make([]string, len(items))
	for i, m := range items {
		out[i] = m.MeterID
	}
	return out
}

func periodSum(t *testing.T, tx pgx.Tx, meterID string) float64 {
	t.Helper()
	var sum float64
	err := tx.QueryRow(context.Background(),
		`SELECT ROUND(SUM(consumption_kwh), 2) FROM readings WHERE meter_id = $1`, meterID).Scan(&sum)
	if err != nil {
		t.Fatalf("period sum: %v", err)
	}
	return sum
}

func TestIntegrationListNoRuns(t *testing.T) {
	tx := dbtest.Tx(t)
	repo := NewRepository(tx)

	items, total := mustList(t, repo, allParams)

	if total != 12 || len(items) != 12 {
		t.Fatalf("total = %d, items = %d, want 12", total, len(items))
	}
	for _, m := range items {
		if m.Health != nil || m.Metrics != nil || m.Anomaly != nil {
			t.Errorf("%s: health/metrics/anomaly = %v/%v/%v, want nil", m.MeterID, m.Health, m.Metrics, m.Anomaly)
		}
		if want := periodSum(t, tx, m.MeterID); m.PeriodConsumptionKWh <= 0 || m.PeriodConsumptionKWh != want {
			t.Errorf("%s: period = %v, want %v", m.MeterID, m.PeriodConsumptionKWh, want)
		}
		if m.ID == 0 || m.Status == nil || m.CreatedAt == nil {
			t.Errorf("%s: missing §16 fields: %+v", m.MeterID, m)
		}
	}
}

func TestIntegrationListHealth(t *testing.T) {
	tx := dbtest.Tx(t)
	dbtest.SeedFixture(t, tx)
	repo := NewRepository(tx)

	items, _ := mustList(t, repo, allParams)
	got := byMeterID(items)

	want := map[string]string{"M-109": "CRITICAL", "M-104": "ALERT", "M-112": "ALERT", "M-106": "OK", "M-101": "OK"}
	for id, health := range want {
		m := got[id]
		if m.Health == nil || *m.Health != health {
			t.Errorf("%s health = %v, want %s", id, m.Health, health)
		}
	}
	if m := got["M-109"]; m.Metrics == nil || m.Metrics.VariationPct != 110.5 || m.Anomaly == nil || m.Anomaly.MeterID != "M-109" {
		t.Errorf("M-109 = %+v, want metrics and anomaly", m)
	}
}

func TestIntegrationListIgnoresNonCompletedRuns(t *testing.T) {
	tx := dbtest.Tx(t)
	dbtest.SeedFixture(t, tx)
	later := dbtest.FixtureFinishedAt.Add(24 * time.Hour)

	running := dbtest.InsertRun(t, tx, "RUNNING", later, nil)
	dbtest.InsertMetrics(t, tx, running, "M-101", 5000, 1000, 400)

	failedEnd := later.Add(2 * time.Hour)
	failed := dbtest.InsertRun(t, tx, "FAILED", later.Add(time.Hour), &failedEnd)
	dbtest.InsertMetrics(t, tx, failed, "M-101", 5000, 1000, 400)
	dbtest.InsertAnomaly(t, tx, failed, "M-101", "REAL_ANOMALY", "HIGH", 0.99, 1)

	noFinish := dbtest.InsertRun(t, tx, "COMPLETED", later.Add(3*time.Hour), nil)
	dbtest.InsertAnomaly(t, tx, noFinish, "M-101", "REAL_ANOMALY", "HIGH", 0.99, 1)

	items, _ := mustList(t, NewRepository(tx), allParams)
	m := byMeterID(items)["M-101"]

	if m.Health == nil || *m.Health != "OK" {
		t.Errorf("M-101 health = %v, want OK", m.Health)
	}
	if m.Metrics == nil || m.Metrics.VariationPct != 0.5 {
		t.Errorf("M-101 metrics = %+v, want fixture variation 0.5", m.Metrics)
	}
}

func TestIntegrationListFilterAndSearch(t *testing.T) {
	tx := dbtest.Tx(t)
	dbtest.SeedFixture(t, tx)
	repo := NewRepository(tx)

	items, total := mustList(t, repo, ListParams{Health: "CRITICAL", Sort: SortMeterID, Limit: 50})
	if total != 1 || len(items) != 1 || items[0].MeterID != "M-109" {
		t.Errorf("critical = %v (total %d), want [M-109]", meterIDs(items), total)
	}
}

func TestIntegrationListSearch(t *testing.T) {
	tx := dbtest.Tx(t)
	repo := NewRepository(tx)

	tests := []struct {
		name   string
		search string // already LIKE-escaped, as the service would pass it
		want   []string
	}{
		{"case-insensitive substring", "m-11", []string{"M-110", "M-111", "M-112"}},
		{"escaped percent is literal", `\%`, []string{}},
		{"escaped underscore is literal", `\_`, []string{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			items, total := mustList(t, repo, ListParams{Search: tt.search, Sort: SortMeterID, Limit: 50})
			got := meterIDs(items)
			if total != len(tt.want) || len(got) != len(tt.want) {
				t.Fatalf("got %v (total %d), want %v", got, total, tt.want)
			}
			for i := range tt.want {
				if got[i] != tt.want[i] {
					t.Errorf("got %v, want %v", got, tt.want)
				}
			}
		})
	}
}

func TestIntegrationListSort(t *testing.T) {
	tx := dbtest.Tx(t)
	dbtest.SeedFixture(t, tx)
	if _, err := tx.Exec(context.Background(), `DELETE FROM meter_metrics WHERE meter_id = 'M-107'`); err != nil {
		t.Fatalf("delete metrics: %v", err)
	}
	repo := NewRepository(tx)

	items, _ := mustList(t, repo, ListParams{Sort: SortVariation, Desc: true, Limit: 50})
	if items[0].MeterID != "M-109" {
		t.Errorf("variation desc first = %s, want M-109", items[0].MeterID)
	}
	if last := items[len(items)-1]; last.MeterID != "M-107" || last.Metrics != nil {
		t.Errorf("variation desc last = %s (metrics %v), want M-107 without metrics", last.MeterID, last.Metrics)
	}

	items, _ = mustList(t, repo, ListParams{Sort: SortSeverity, Desc: true, Limit: 50})
	want := []string{"M-109", "M-112", "M-104", "M-106", "M-101", "M-102", "M-103", "M-105", "M-107", "M-108", "M-110", "M-111"}
	got := meterIDs(items)
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("severity desc = %v, want %v", got, want)
		}
	}
}

func TestIntegrationListPagination(t *testing.T) {
	tx := dbtest.Tx(t)
	repo := NewRepository(tx)

	items, total := mustList(t, repo, ListParams{Sort: SortMeterID, Limit: 5, Offset: 10})
	if len(items) != 2 || total != 12 {
		t.Errorf("limit 5 offset 10 = %d items, total %d; want 2, 12", len(items), total)
	}

	items, total = mustList(t, repo, ListParams{Sort: SortMeterID, Limit: 50, Offset: 50})
	if len(items) != 0 || total != 12 {
		t.Errorf("offset 50 = %d items, total %d; want 0, 12", len(items), total)
	}
}

func TestIntegrationGet(t *testing.T) {
	tx := dbtest.Tx(t)
	f := dbtest.SeedFixture(t, tx)
	repo := NewRepository(tx)
	ctx := context.Background()

	for _, id := range []string{"M-999", "m-109"} {
		d, err := repo.Get(ctx, id)
		if err != nil || d != nil {
			t.Errorf("Get(%q) = %v, %v; want nil, nil", id, d, err)
		}
	}

	d, err := repo.Get(ctx, "M-109")
	if err != nil || d == nil {
		t.Fatalf("Get(M-109) = %v, %v", d, err)
	}
	if len(d.Events) != 1 || d.Events[0].Type == nil || *d.Events[0].Type != "UNKNOWN" || d.Events[0].MeterID != "M-109" {
		t.Errorf("events = %+v, want one UNKNOWN event for M-109", d.Events)
	}
	if want := periodSum(t, tx, "M-109"); d.PeriodConsumptionKWh != want {
		t.Errorf("period = %v, want %v", d.PeriodConsumptionKWh, want)
	}
	a := d.Anomaly
	if a == nil || a.ID != f.AnomalyIDs["M-109"] || a.MeterID != "M-109" || a.Reason == nil || a.RecommendedAction == nil || a.Status == nil || a.DetectedAt == nil {
		t.Errorf("anomaly = %+v, want full fixture record", a)
	}
}

func TestIntegrationReadings(t *testing.T) {
	tx := dbtest.Tx(t)
	repo := NewRepository(tx)
	ctx := context.Background()

	all, err := repo.Readings(ctx, "M-109", ReadingsParams{})
	if err != nil {
		t.Fatalf("Readings() error = %v", err)
	}
	if len(all) != 336 {
		t.Fatalf("readings = %d, want 336", len(all))
	}
	if first := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC); !all[0].Timestamp.Equal(first) || all[0].Timestamp.Location() != time.UTC {
		t.Errorf("first timestamp = %v, want %v UTC", all[0].Timestamp, first)
	}
	for i := 1; i < len(all); i++ {
		if !all[i].Timestamp.After(all[i-1].Timestamp) {
			t.Fatalf("readings not ascending at %d", i)
		}
	}
	if all[0].ID == 0 || all[0].MeterID != "M-109" || all[0].ConsumptionKWh == nil || all[0].Status == nil {
		t.Errorf("reading = %+v, missing §16 fields", all[0])
	}

	from := time.Date(2026, 9, 14, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 9, 14, 23, 59, 59, 999999000, time.UTC)
	day, err := repo.Readings(ctx, "M-109", ReadingsParams{From: &from, To: &to})
	if err != nil {
		t.Fatalf("Readings(day) error = %v", err)
	}
	if len(day) != 24 {
		t.Errorf("day readings = %d, want 24", len(day))
	}
}

func TestIntegrationExists(t *testing.T) {
	tx := dbtest.Tx(t)
	repo := NewRepository(tx)
	ctx := context.Background()

	if ok, err := repo.Exists(ctx, "M-101"); err != nil || !ok {
		t.Errorf("Exists(M-101) = %v, %v; want true", ok, err)
	}
	if ok, err := repo.Exists(ctx, "M-999"); err != nil || ok {
		t.Errorf("Exists(M-999) = %v, %v; want false", ok, err)
	}
}
