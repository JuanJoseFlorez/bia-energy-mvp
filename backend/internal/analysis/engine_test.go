package analysis

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
	"time"
)

var allSteps = []string{"READINGS", "BASELINE", "DETECTION", "CORRELATION", "EVENTS", "EXPLANATION", "RECOMMENDATION"}

const resultLine = `{"type":"result","metrics":[{"meter_id":"M-109","current_kwh":2208.4,"baseline_kwh":1052.7,"variation_pct":109.8,"change_start":"2026-09-12T14:00:00Z"}],` +
	`"anomalies":[{"meter_id":"M-109","anomaly":true,"type":"REAL_ANOMALY","severity":"HIGH","confidence":0.96,"priority":1,` +
	`"reason":"r","explanation":"e","recommended_action":"a","explanation_source":"template","evidence":{"related_event_ids":[3],"signals":["current"]}}]}`

func stepLines(steps ...string) []string {
	lines := make([]string, len(steps))
	for i, s := range steps {
		lines[i] = `{"type":"step","step":"` + s + `"}`
	}
	return lines
}

// ndjsonServer answers POST /analyze with the given lines, flushing after each one.
func ndjsonServer(t *testing.T, status int, lines ...string) (*httptest.Server, *[]byte) {
	t.Helper()
	var gotBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/analyze" {
			t.Errorf("request = %s %s, want POST /analyze", r.Method, r.URL.Path)
		}
		gotBody, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/x-ndjson")
		w.WriteHeader(status)
		for _, l := range lines {
			_, _ = io.WriteString(w, l+"\n")
			w.(http.Flusher).Flush()
		}
	}))
	t.Cleanup(srv.Close)
	return srv, &gotBody
}

func analyze(t *testing.T, url string) ([]string, Result, error) {
	t.Helper()
	var steps []string
	res, err := NewEngineClient(url).Analyze(context.Background(), 7, func(s string) { steps = append(steps, s) })
	return steps, res, err
}

func TestEngineAnalyzeHappyPath(t *testing.T) {
	srv, body := ndjsonServer(t, http.StatusOK, append(stepLines(allSteps...), resultLine)...)

	steps, res, err := analyze(t, srv.URL+"/")

	if err != nil {
		t.Fatalf("Analyze() error = %v", err)
	}
	if string(*body) != `{"analysis_id":7}` {
		t.Errorf("request body = %s, want {\"analysis_id\":7}", *body)
	}
	if !slices.Equal(steps, allSteps) {
		t.Errorf("steps = %v, want %v", steps, allSteps)
	}
	if len(res.Metrics) != 1 || res.Metrics[0].MeterID != "M-109" || res.Metrics[0].VariationPct != 109.8 {
		t.Errorf("metrics = %+v", res.Metrics)
	}
	wantStart := time.Date(2026, 9, 12, 14, 0, 0, 0, time.UTC)
	if res.Metrics[0].ChangeStart == nil || !res.Metrics[0].ChangeStart.Equal(wantStart) {
		t.Errorf("change_start = %v, want %v", res.Metrics[0].ChangeStart, wantStart)
	}
	if len(res.Anomalies) != 1 {
		t.Fatalf("anomalies = %d, want 1", len(res.Anomalies))
	}
	a := res.Anomalies[0]
	if a.Type != "REAL_ANOMALY" || a.Severity != "HIGH" || a.Priority != 1 || a.ExplanationSource != "template" {
		t.Errorf("anomaly = %+v", a)
	}
	if string(a.Evidence) != `{"related_event_ids":[3],"signals":["current"]}` {
		t.Errorf("evidence = %s, want verbatim JSON", a.Evidence)
	}
}

func TestEngineAnalyzeFailures(t *testing.T) {
	tests := []struct {
		name   string
		status int
		lines  []string
		want   error
	}{
		{"error line", http.StatusOK, append(stepLines("READINGS"), `{"type":"error","message":"analysis failed"}`), errEngineFailed},
		{"stream without result", http.StatusOK, stepLines(allSteps...), errNoResult},
		{"empty stream", http.StatusOK, nil, errNoResult},
		{"invalid JSON line", http.StatusOK, []string{`{"type":"step"`}, errNoResult},
		{"engine database down", http.StatusServiceUnavailable, []string{`{"error":"database unavailable"}`}, errEngineUnavailable},
		{"engine internal error", http.StatusInternalServerError, nil, errEngineUnavailable},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv, _ := ndjsonServer(t, tt.status, tt.lines...)

			_, _, err := analyze(t, srv.URL)

			if !errors.Is(err, tt.want) {
				t.Errorf("error = %v, want %v", err, tt.want)
			}
		})
	}
}

func TestEngineAnalyzeUnreachable(t *testing.T) {
	srv, _ := ndjsonServer(t, http.StatusOK)
	url := srv.URL
	srv.Close()

	_, _, err := analyze(t, url)

	if !errors.Is(err, errEngineUnavailable) {
		t.Errorf("error = %v, want %v", err, errEngineUnavailable)
	}
}

func TestEngineAnalyzeIgnoresUnknownAndBlankLines(t *testing.T) {
	srv, _ := ndjsonServer(t, http.StatusOK, `{"type":"heartbeat"}`, "", `{"type":"step","step":"READINGS"}`, resultLine)

	steps, res, err := analyze(t, srv.URL)

	if err != nil {
		t.Fatalf("Analyze() error = %v", err)
	}
	if !slices.Equal(steps, []string{"READINGS"}) || len(res.Anomalies) != 1 {
		t.Errorf("steps = %v, anomalies = %d", steps, len(res.Anomalies))
	}
}

func TestEngineAnalyzeLargeResultLine(t *testing.T) {
	outliers := make([]string, 20000)
	for i := range outliers {
		outliers[i] = `"2026-09-12T14:00:00Z"`
	}
	evidence := `{"outlier_timestamps":[` + strings.Join(outliers, ",") + `]}`
	line := `{"type":"result","metrics":[],"anomalies":[{"meter_id":"M-109","type":"REAL_ANOMALY","severity":"HIGH","evidence":` + evidence + `}]}`
	if len(line) < 64<<10 {
		t.Fatalf("test line is %d bytes, want > 64 KiB", len(line))
	}
	srv, _ := ndjsonServer(t, http.StatusOK, line)

	_, res, err := analyze(t, srv.URL)

	if err != nil {
		t.Fatalf("Analyze() error = %v", err)
	}
	var ev struct {
		OutlierTimestamps []string `json:"outlier_timestamps"`
	}
	if err := json.Unmarshal(res.Anomalies[0].Evidence, &ev); err != nil || len(ev.OutlierTimestamps) != 20000 {
		t.Errorf("evidence outliers = %d (err %v), want 20000", len(ev.OutlierTimestamps), err)
	}
}

func TestEngineAnalyzeDeadline(t *testing.T) {
	release := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, `{"type":"step","step":"READINGS"}`+"\n")
		w.(http.Flusher).Flush()
		select {
		case <-release:
		case <-r.Context().Done():
		}
	}))
	t.Cleanup(func() { close(release); srv.Close() })
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	var steps []string
	_, err := NewEngineClient(srv.URL).Analyze(ctx, 7, func(s string) { steps = append(steps, s) })

	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("error = %v, want context.DeadlineExceeded", err)
	}
	if !slices.Equal(steps, []string{"READINGS"}) {
		t.Errorf("steps = %v, want [READINGS] before the deadline", steps)
	}
}
