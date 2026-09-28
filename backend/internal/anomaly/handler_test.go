package anomaly

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func serve(t *testing.T, repo *fakeRepo, method, target, body string) *httptest.ResponseRecorder {
	t.Helper()
	mux := http.NewServeMux()
	NewHandler(NewService(repo)).Register(mux)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(method, target, strings.NewReader(body)))
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

func requireKeys(t *testing.T, obj map[string]any, keys ...string) {
	t.Helper()
	for _, k := range keys {
		if _, ok := obj[k]; !ok {
			t.Errorf("missing key %q in %v", k, obj)
		}
	}
}

var itemKeys = []string{"id", "analysis_id", "meter_id", "detected_at", "anomaly", "type", "severity", "confidence",
	"priority", "reason", "recommended_action", "explanation_source", "status"}

func TestHandlerList(t *testing.T) {
	typ := "REAL_ANOMALY"
	repo := &fakeRepo{latest: int64Ptr(7), items: []Item{{ID: 31, AnalysisID: 7, MeterID: "M-109", IsAnomaly: true, Type: &typ, Status: StatusPending}}}

	rec := serve(t, repo, http.MethodGet, "/anomalies?type=REAL_ANOMALY", "")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body %s", rec.Code, rec.Body)
	}
	body := decodeMap(t, rec)
	if body["analysis_id"] != float64(7) || body["total"] != float64(1) {
		t.Errorf("envelope = %v", body)
	}
	item := body["items"].([]any)[0].(map[string]any)
	requireKeys(t, item, itemKeys...)
	if item["anomaly"] != true {
		t.Errorf("anomaly = %v, want true", item["anomaly"])
	}
	if _, ok := item["evidence"]; ok {
		t.Error("list item exposes evidence, want it only in the detail")
	}
}

func TestHandlerListBeforeAnyRun(t *testing.T) {
	rec := serve(t, &fakeRepo{}, http.MethodGet, "/anomalies", "")

	if rec.Code != http.StatusOK || strings.TrimSpace(rec.Body.String()) != `{"analysis_id":null,"items":[],"total":0}` {
		t.Errorf("status = %d, body %s", rec.Code, rec.Body)
	}
}

func TestHandlerListInvalidFilter(t *testing.T) {
	if rec := serve(t, &fakeRepo{}, http.MethodGet, "/anomalies?severity=CRITICAL", ""); rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}
}

func TestHandlerGet(t *testing.T) {
	repo := &fakeRepo{detail: detailWith(StatusPending, `{"change_start":"2026-09-12T14:00:00Z"}`), first: &periodStart, last: &periodEnd}

	rec := serve(t, repo, http.MethodGet, "/anomalies/31", "")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body %s", rec.Code, rec.Body)
	}
	body := decodeMap(t, rec)
	requireKeys(t, body, append(itemKeys, "explanation", "evidence", "related_events", "readings_window", "next_statuses")...)
	if ev := body["evidence"].(map[string]any); ev["change_start"] != "2026-09-12T14:00:00Z" {
		t.Errorf("evidence = %v, want verbatim", ev)
	}
	if w := body["readings_window"].(map[string]any); w["from"] != "2026-09-05T14:00:00Z" || w["to"] != "2026-09-14T23:00:00Z" {
		t.Errorf("readings_window = %v", w)
	}
}

func TestHandlerGetNotFound(t *testing.T) {
	if rec := serve(t, &fakeRepo{}, http.MethodGet, "/anomalies/31", ""); rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", rec.Code)
	}
}

func TestHandlerPatch(t *testing.T) {
	tests := []struct {
		name string
		from string
		body string
		want int
	}{
		{"allowed", StatusPending, `{"status":"INVESTIGATING"}`, http.StatusOK},
		{"terminal", StatusDismissed, `{"status":"INVESTIGATING"}`, http.StatusConflict},
		{"unknown status", StatusPending, `{"status":"DONE"}`, http.StatusBadRequest},
		{"unknown field", StatusPending, `{"status":"INVESTIGATING","note":"x"}`, http.StatusBadRequest},
		{"empty body", StatusPending, ``, http.StatusBadRequest},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &fakeRepo{detail: detailWith(tt.from, `{}`), updateOK: true}

			rec := serve(t, repo, http.MethodPatch, "/anomalies/31", tt.body)

			if rec.Code != tt.want {
				t.Fatalf("status = %d, want %d, body %s", rec.Code, tt.want, rec.Body)
			}
			if tt.want == http.StatusOK {
				body := decodeMap(t, rec)
				if body["status"] != StatusInvestigating {
					t.Errorf("status = %v, want INVESTIGATING", body["status"])
				}
				next, _ := json.Marshal(body["next_statuses"])
				if string(next) != `["VALIDATED","RESOLVED","DISMISSED"]` {
					t.Errorf("next_statuses = %s", next)
				}
			}
		})
	}
}

func TestHandlerPatchNotFound(t *testing.T) {
	if rec := serve(t, &fakeRepo{}, http.MethodPatch, "/anomalies/31", `{"status":"INVESTIGATING"}`); rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", rec.Code)
	}
}
