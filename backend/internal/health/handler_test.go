package health

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

type fakePinger struct {
	err   error
	block bool // wait until ctx is done
}

func (f fakePinger) Ping(ctx context.Context) error {
	if f.block {
		<-ctx.Done()
		return ctx.Err()
	}
	return f.err
}

func TestHealth(t *testing.T) {
	tests := []struct {
		name         string
		pinger       fakePinger
		wantStatus   int
		wantStatusJS string
		wantDB       string
	}{
		{"db up", fakePinger{}, http.StatusOK, "ok", "up"},
		{"db error", fakePinger{err: errors.New("connection refused")}, http.StatusServiceUnavailable, "degraded", "down"},
		{"db timeout", fakePinger{block: true}, http.StatusServiceUnavailable, "degraded", "down"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mux := http.NewServeMux()
			NewHandler(tt.pinger, 20*time.Millisecond).Register(mux)
			rec := httptest.NewRecorder()

			mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/health", nil))

			if rec.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d", rec.Code, tt.wantStatus)
			}
			var body struct {
				Status   string `json:"status"`
				Database string `json:"database"`
			}
			if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
				t.Fatalf("invalid JSON body: %v", err)
			}
			if body.Status != tt.wantStatusJS || body.Database != tt.wantDB {
				t.Errorf("body = %+v, want status=%s database=%s", body, tt.wantStatusJS, tt.wantDB)
			}
		})
	}
}
