package httpx

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func testMux() *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /widgets", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	return mux
}

func TestRoutes(t *testing.T) {
	tests := []struct {
		name       string
		method     string
		path       string
		wantStatus int
		wantCode   string
	}{
		{"registered route passes through", http.MethodGet, "/widgets", http.StatusOK, ""},
		{"wrong method is 405", http.MethodPost, "/widgets", http.StatusMethodNotAllowed, "method_not_allowed"},
		{"unknown path is 404", http.MethodGet, "/does-not-exist", http.StatusNotFound, "not_found"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			req := httptest.NewRequest(tt.method, tt.path, nil)

			Routes(testMux()).ServeHTTP(rec, req)

			if rec.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d", rec.Code, tt.wantStatus)
			}
			if tt.wantCode == "" {
				return
			}
			var body errorResponse
			if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
				t.Fatalf("invalid JSON body: %v", err)
			}
			if body.Error.Code != tt.wantCode {
				t.Errorf("code = %q, want %q", body.Error.Code, tt.wantCode)
			}
		})
	}
}

func TestRoutesMethodNotAllowedHasAllowHeader(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/widgets", nil)

	Routes(testMux()).ServeHTTP(rec, req)

	allow := rec.Header().Get("Allow")
	if allow == "" {
		t.Fatal("Allow header is empty")
	}
	if !strings.Contains(allow, http.MethodGet) {
		t.Errorf("Allow = %q, want it to contain GET", allow)
	}
}
