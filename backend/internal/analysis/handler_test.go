package analysis

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/JuanJoseFlorez/bia-energy-mvp/backend/internal/platform/apperr"
)

func serve(t *testing.T, svc *Service, method, target string) *httptest.ResponseRecorder {
	t.Helper()
	mux := http.NewServeMux()
	NewHandler(svc).Register(mux)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(method, target, nil))
	return rec
}

func decodeMap(t *testing.T, rec *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &m); err != nil {
		t.Fatalf("invalid JSON %q: %v", rec.Body.String(), err)
	}
	return m
}

func TestHandlerStartAccepted(t *testing.T) {
	svc := NewService(&fakeRepo{}, &fakeEngine{}, time.Minute)

	rec := serve(t, svc, http.MethodPost, "/ai/analyze")
	svc.wg.Wait()

	if rec.Code != http.StatusAccepted {
		t.Fatalf("status = %d, body %s", rec.Code, rec.Body)
	}
	if loc := rec.Header().Get("Location"); loc != "/ai/analysis/7" {
		t.Errorf("Location = %q, want /ai/analysis/7", loc)
	}
	body := decodeMap(t, rec)
	for _, k := range []string{"id", "status", "current_step", "started_at", "updated_at", "finished_at", "summary", "error"} {
		if _, ok := body[k]; !ok {
			t.Errorf("missing key %q in %v", k, body)
		}
	}
	if body["id"] != float64(7) || body["status"] != StatusPending {
		t.Errorf("body = %v, want id 7 PENDING", body)
	}
}

func TestHandlerStartConflict(t *testing.T) {
	repo := &fakeRepo{createErr: fmt.Errorf("an analysis is already running: %w", apperr.ErrConflict)}

	rec := serve(t, NewService(repo, &fakeEngine{}, time.Minute), http.MethodPost, "/ai/analyze")

	if rec.Code != http.StatusConflict {
		t.Errorf("status = %d, want 409", rec.Code)
	}
}

func TestHandlerGet(t *testing.T) {
	summary := &Summary{AnomaliesDetected: 4, HighPriority: 2}
	tests := []struct {
		name   string
		target string
		repo   *fakeRepo
		want   int
	}{
		{"found", "/ai/analysis/7", &fakeRepo{run: &Run{ID: 7, Status: StatusCompleted, Summary: summary}}, http.StatusOK},
		{"not found", "/ai/analysis/7", &fakeRepo{}, http.StatusNotFound},
		{"invalid id", "/ai/analysis/abc", &fakeRepo{}, http.StatusBadRequest},
		{"latest", "/ai/analysis/latest", &fakeRepo{run: &Run{ID: 7, Status: StatusRunning}}, http.StatusOK},
		{"latest without runs", "/ai/analysis/latest", &fakeRepo{}, http.StatusNotFound},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := serve(t, NewService(tt.repo, &fakeEngine{}, time.Minute), http.MethodGet, tt.target)

			if rec.Code != tt.want {
				t.Fatalf("status = %d, want %d, body %s", rec.Code, tt.want, rec.Body)
			}
			if tt.want == http.StatusOK && decodeMap(t, rec)["id"] != float64(7) {
				t.Errorf("body = %s, want id 7", rec.Body)
			}
		})
	}
}

func TestHandlerGetCompletedSummary(t *testing.T) {
	repo := &fakeRepo{run: &Run{ID: 7, Status: StatusCompleted, Summary: &Summary{AnomaliesDetected: 4, HighPriority: 2}}}

	body := decodeMap(t, serve(t, NewService(repo, &fakeEngine{}, time.Minute), http.MethodGet, "/ai/analysis/7"))

	s, ok := body["summary"].(map[string]any)
	if !ok || s["anomalies_detected"] != float64(4) || s["high_priority"] != float64(2) {
		t.Errorf("summary = %v, want {anomalies_detected: 4, high_priority: 2}", body["summary"])
	}
}
