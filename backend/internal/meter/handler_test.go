package meter

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func serve(t *testing.T, repo *fakeRepo, target string) *httptest.ResponseRecorder {
	t.Helper()
	mux := http.NewServeMux()
	NewHandler(NewService(repo)).Register(mux)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, target, nil))
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

func strPtr(s string) *string { return &s }

func TestHandlerListPassesParams(t *testing.T) {
	repo := &fakeRepo{}

	rec := serve(t, repo, "/meters?status=critical&q=m-1&sort=variation&order=asc&limit=5&offset=2")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body %s", rec.Code, rec.Body)
	}
	want := ListParams{Health: "CRITICAL", Search: "m-1", Sort: SortVariation, Desc: false, Limit: 5, Offset: 2}
	if repo.gotList != want {
		t.Errorf("params = %+v, want %+v", repo.gotList, want)
	}
}

func TestHandlerListFields(t *testing.T) {
	created := time.Date(2026, 9, 26, 22, 0, 0, 0, time.UTC)
	repo := &fakeRepo{
		items: []Meter{{ID: 1, MeterID: "M-101", Name: strPtr("Meter 101"), Location: strPtr("Planta A"),
			Status: strPtr("ACTIVE"), CreatedAt: &created, PeriodConsumptionKWh: 10226.5}},
		total: 1,
	}

	body := decodeMap(t, serve(t, repo, "/meters"))

	if body["total"] != float64(1) {
		t.Errorf("total = %v, want 1", body["total"])
	}
	item := body["items"].([]any)[0].(map[string]any)
	requireKeys(t, item, "id", "meter_id", "name", "location", "status", "created_at",
		"period_consumption_kwh", "health", "metrics", "anomaly")
	for _, k := range []string{"health", "metrics", "anomaly"} {
		if item[k] != nil {
			t.Errorf("%s = %v, want null", k, item[k])
		}
	}
}

func TestHandlerListEmptyItems(t *testing.T) {
	rec := serve(t, &fakeRepo{}, "/meters?q=zzz")

	if !strings.Contains(rec.Body.String(), `"items":[]`) {
		t.Errorf("body = %s, want items []", rec.Body)
	}
}

func TestHandlerListInvalidParam(t *testing.T) {
	rec := serve(t, &fakeRepo{}, "/meters?sort=bogus")

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
	errObj := decodeMap(t, rec)["error"].(map[string]any)
	if errObj["code"] != "validation_error" || !strings.Contains(errObj["message"].(string), "sort") {
		t.Errorf("error = %v", errObj)
	}
}

func TestHandlerGetNotFound(t *testing.T) {
	rec := serve(t, &fakeRepo{}, "/meters/M-999")

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
	if code := decodeMap(t, rec)["error"].(map[string]any)["code"]; code != "not_found" {
		t.Errorf("code = %v, want not_found", code)
	}
}

func TestHandlerGetFields(t *testing.T) {
	at := time.Date(2026, 9, 12, 14, 0, 0, 0, time.UTC)
	sev, typ := "HIGH", "REAL_ANOMALY"
	repo := &fakeRepo{detail: &MeterDetail{
		Meter: Meter{ID: 9, MeterID: "M-109"},
		Anomaly: &AnomalyDetail{
			AnomalySummary: AnomalySummary{ID: 3, MeterID: "M-109", Type: &typ, Severity: &sev},
			Reason:         strPtr("reason"), RecommendedAction: strPtr("action"), Status: strPtr("PENDING"), DetectedAt: &at,
		},
		Events: []Event{{ID: 1, MeterID: "M-109", Timestamp: at, Type: strPtr("UNKNOWN"), Description: strPtr("none")}},
	}}

	body := decodeMap(t, serve(t, repo, "/meters/M-109"))

	requireKeys(t, body, "id", "meter_id", "name", "location", "status", "created_at", "period_consumption_kwh",
		"health", "metrics", "anomaly", "events")
	requireKeys(t, body["anomaly"].(map[string]any), "id", "meter_id", "detected_at", "type", "severity",
		"confidence", "priority", "reason", "recommended_action", "status")
	requireKeys(t, body["events"].([]any)[0].(map[string]any), "id", "meter_id", "timestamp", "type", "description")
}

func TestHandlerGetEmptyEvents(t *testing.T) {
	rec := serve(t, &fakeRepo{detail: &MeterDetail{Meter: Meter{MeterID: "M-101"}}}, "/meters/M-101")

	if !strings.Contains(rec.Body.String(), `"events":[]`) {
		t.Errorf("body = %s, want events []", rec.Body)
	}
}

func TestHandlerReadingsFields(t *testing.T) {
	v := 23.5
	repo := &fakeRepo{exists: true, readings: []Reading{{ID: 2689, MeterID: "M-109",
		Timestamp: time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC), ConsumptionKWh: &v, VoltageV: &v,
		CurrentA: &v, PowerFactor: &v, Status: strPtr("OK")}}}

	body := decodeMap(t, serve(t, repo, "/meters/M-109/readings?from=2026-09-01"))

	requireKeys(t, body, "meter_id", "items", "total")
	item := body["items"].([]any)[0].(map[string]any)
	requireKeys(t, item, "id", "meter_id", "timestamp", "consumption_kwh", "voltage_v", "current_a", "power_factor", "status")
	if ts := item["timestamp"].(string); !strings.HasSuffix(ts, "Z") {
		t.Errorf("timestamp = %q, want UTC with Z", ts)
	}
}

func TestHandlerReadingsErrors(t *testing.T) {
	if rec := serve(t, &fakeRepo{exists: false}, "/meters/M-999/readings"); rec.Code != http.StatusNotFound {
		t.Errorf("unknown meter status = %d, want 404", rec.Code)
	}
	if rec := serve(t, &fakeRepo{exists: true}, "/meters/M-101/readings?from=nope"); rec.Code != http.StatusBadRequest {
		t.Errorf("bad from status = %d, want 400", rec.Code)
	}
}
