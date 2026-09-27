package dashboard

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

type fakeRepo struct {
	summary Summary
	err     error
}

func (f fakeRepo) Summary(context.Context) (Summary, error) { return f.summary, f.err }

func serve(t *testing.T, repo fakeRepo) *httptest.ResponseRecorder {
	t.Helper()
	mux := http.NewServeMux()
	NewHandler(NewService(repo)).Register(mux)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/dashboard/summary", nil))
	return rec
}

func TestHandlerSummaryNulls(t *testing.T) {
	rec := serve(t, fakeRepo{summary: Summary{MetersCount: 12}})

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	for _, k := range []string{"meters_count", "period", "total_consumption_kwh", "last_analysis", "anomalies"} {
		if _, ok := body[k]; !ok {
			t.Errorf("missing key %q", k)
		}
	}
	for _, k := range []string{"period", "last_analysis", "anomalies"} {
		if body[k] != nil {
			t.Errorf("%s = %v, want null", k, body[k])
		}
	}
}

func TestHandlerSummaryStats(t *testing.T) {
	finished := time.Date(2026, 9, 15, 8, 5, 0, 0, time.UTC)
	rec := serve(t, fakeRepo{summary: Summary{
		MetersCount: 12,
		Anomalies: &AnomalyStats{AnalysisID: 6, AnalysisFinishedAt: finished, Detected: 4, HighPriority: 2,
			ByType: map[string]int{"REAL_ANOMALY": 1, "EXPLAINABLE_ANOMALY": 1, "FALSE_POSITIVE": 1, "DATA_QUALITY": 1}},
	}})

	var body struct {
		Anomalies map[string]any `json:"anomalies"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	for _, k := range []string{"analysis_id", "analysis_finished_at", "detected", "high_priority", "avg_confidence", "by_type"} {
		if _, ok := body.Anomalies[k]; !ok {
			t.Errorf("missing anomalies.%s", k)
		}
	}
}

func TestHandlerSummaryError(t *testing.T) {
	rec := serve(t, fakeRepo{err: errors.New("db down")})

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", rec.Code)
	}
}
