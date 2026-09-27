package main

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/JuanJoseFlorez/bia-energy-mvp/backend/internal/dashboard"
	"github.com/JuanJoseFlorez/bia-energy-mvp/backend/internal/health"
	"github.com/JuanJoseFlorez/bia-energy-mvp/backend/internal/meter"
)

type okPinger struct{}

func (okPinger) Ping(context.Context) error { return nil }

// emptyMeters is a meter.Repository with no data.
type emptyMeters struct{}

func (emptyMeters) List(context.Context, meter.ListParams) ([]meter.Meter, int, error) {
	return nil, 0, nil
}
func (emptyMeters) Get(context.Context, string) (*meter.MeterDetail, error) { return nil, nil }
func (emptyMeters) Exists(context.Context, string) (bool, error)            { return false, nil }
func (emptyMeters) Readings(context.Context, string, meter.ReadingsParams) ([]meter.Reading, error) {
	return nil, nil
}

// emptyDashboard is a dashboard.Repository with no data.
type emptyDashboard struct{}

func (emptyDashboard) Summary(context.Context) (dashboard.Summary, error) {
	return dashboard.Summary{}, nil
}

func testRouter() http.Handler {
	log := slog.New(slog.NewJSONHandler(io.Discard, nil))
	return newRouter(log, []string{"http://localhost:3000"},
		health.NewHandler(okPinger{}, time.Second),
		meter.NewHandler(meter.NewService(emptyMeters{})),
		dashboard.NewHandler(dashboard.NewService(emptyDashboard{})),
	)
}

func TestRouterHealth(t *testing.T) {
	rec := httptest.NewRecorder()

	testRouter().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/health", nil))

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Code)
	}
	if rec.Header().Get("X-Request-ID") == "" {
		t.Error("missing X-Request-ID header")
	}
}

func TestRouterUnknownRoute(t *testing.T) {
	rec := httptest.NewRecorder()

	testRouter().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/does-not-exist", nil))

	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", rec.Code)
	}
	var body struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
		RequestID string `json:"request_id"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid JSON body: %v", err)
	}
	if body.Error.Code != "not_found" {
		t.Errorf("code = %q, want not_found", body.Error.Code)
	}
	if body.RequestID == "" {
		t.Error("error body missing request_id")
	}
}

func TestRouterWrongMethod(t *testing.T) {
	rec := httptest.NewRecorder()

	testRouter().ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/health", nil))

	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("status = %d, want 405", rec.Code)
	}
	if rec.Header().Get("Allow") == "" {
		t.Error("missing Allow header")
	}
	var body struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
		RequestID string `json:"request_id"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid JSON body: %v", err)
	}
	if body.Error.Code != "method_not_allowed" {
		t.Errorf("code = %q, want method_not_allowed", body.Error.Code)
	}
	if body.RequestID == "" {
		t.Error("error body missing request_id")
	}
}

func TestRouterCORSPreflight(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodOptions, "/health", nil)
	req.Header.Set("Origin", "http://localhost:3000")
	req.Header.Set("Access-Control-Request-Method", http.MethodGet)

	testRouter().ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Errorf("status = %d, want 204", rec.Code)
	}
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "http://localhost:3000" {
		t.Errorf("Access-Control-Allow-Origin = %q, want http://localhost:3000", got)
	}
}

func TestRouterDomainRoutes(t *testing.T) {
	tests := []struct {
		path string
		want int
	}{
		{"/meters", http.StatusOK},
		{"/meters/M-999", http.StatusNotFound},
		{"/meters/M-999/readings", http.StatusNotFound},
		{"/dashboard/summary", http.StatusOK},
	}
	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			rec := httptest.NewRecorder()

			testRouter().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, tt.path, nil))

			if rec.Code != tt.want {
				t.Errorf("status = %d, want %d (body %s)", rec.Code, tt.want, rec.Body)
			}
		})
	}
}

func TestRouterMeterNotFoundIsDomainError(t *testing.T) {
	rec := httptest.NewRecorder()

	testRouter().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/meters/M-999", nil))

	var body struct {
		Error struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if body.Error.Message != "meter M-999: not found" {
		t.Errorf("message = %q, want meter-level not found (not a route 404)", body.Error.Message)
	}
}
