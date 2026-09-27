package httpx

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
)

var okHandler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
})

func discardLogger() *slog.Logger {
	return slog.New(slog.NewJSONHandler(io.Discard, nil))
}

func TestChainOrder(t *testing.T) {
	var calls []string
	mark := func(name string) Middleware {
		return func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls = append(calls, name)
				next.ServeHTTP(w, r)
			})
		}
	}

	h := Chain(okHandler, mark("outer"), mark("inner"))
	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil))

	if want := []string{"outer", "inner"}; !slices.Equal(calls, want) {
		t.Errorf("calls = %v, want %v", calls, want)
	}
}

func TestRequestIDGenerated(t *testing.T) {
	var ctxID string
	h := RequestID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctxID = RequestIDFrom(r.Context())
	}))
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))

	id := rec.Header().Get(RequestIDHeader)
	if _, err := hex.DecodeString(id); err != nil || len(id) != 32 {
		t.Errorf("generated id = %q, want 32 hex chars", id)
	}
	if ctxID != id {
		t.Errorf("context id = %q, header id = %q", ctxID, id)
	}
}

func TestRequestIDPropagated(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set(RequestIDHeader, "abc-123")

	RequestID(okHandler).ServeHTTP(rec, req)

	if got := rec.Header().Get(RequestIDHeader); got != "abc-123" {
		t.Errorf("id = %q, want abc-123", got)
	}
}

func TestRequestIDReplacesOversizedValue(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set(RequestIDHeader, strings.Repeat("x", 129))

	RequestID(okHandler).ServeHTTP(rec, req)

	if got := rec.Header().Get(RequestIDHeader); len(got) != 32 {
		t.Errorf("id = %q, want a freshly generated 32-char id", got)
	}
}

func TestRecover(t *testing.T) {
	panicking := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { panic("boom") })
	rec := httptest.NewRecorder()

	Recover(discardLogger())(panicking).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", rec.Code)
	}
	var body errorResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid JSON body: %v", err)
	}
	if body.Error.Code != "internal_error" {
		t.Errorf("code = %q, want internal_error", body.Error.Code)
	}
}

// loggedStatus runs h through Logger and returns the "status" field of the log line.
func loggedStatus(t *testing.T, h http.Handler) float64 {
	t.Helper()
	var buf bytes.Buffer
	log := slog.New(slog.NewJSONHandler(&buf, nil))

	Logger(log)(h).ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/health", nil))

	var line map[string]any
	if err := json.Unmarshal(buf.Bytes(), &line); err != nil {
		t.Fatalf("invalid log line %q: %v", buf.String(), err)
	}
	if line["path"] != "/health" || line["method"] != "GET" {
		t.Errorf("log line missing method/path: %v", line)
	}
	status, _ := line["status"].(float64)
	return status
}

func TestLoggerRecordsStatus(t *testing.T) {
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusTeapot) })

	if got := loggedStatus(t, h); got != http.StatusTeapot {
		t.Errorf("logged status = %v, want 418", got)
	}
}

func TestLoggerImplicitStatus200(t *testing.T) {
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte("ok")) })

	if got := loggedStatus(t, h); got != http.StatusOK {
		t.Errorf("logged status = %v, want 200", got)
	}
}

func TestCORSAllowedOrigin(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Origin", "http://localhost:3000")

	CORS([]string{"http://localhost:3000"})(okHandler).ServeHTTP(rec, req)

	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "http://localhost:3000" {
		t.Errorf("Allow-Origin = %q, want http://localhost:3000", got)
	}
	if got := rec.Header().Get("Vary"); got != "Origin" {
		t.Errorf("Vary = %q, want Origin", got)
	}
	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200 (request passed through)", rec.Code)
	}
}

func TestCORSVaryOriginAlwaysSet(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)

	CORS([]string{"http://localhost:3000"})(okHandler).ServeHTTP(rec, req)

	if got := rec.Header().Get("Vary"); got != "Origin" {
		t.Errorf("Vary = %q, want Origin (even without an Origin header)", got)
	}
}

func TestCORSDisallowedOrigin(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Origin", "http://evil.test")

	CORS([]string{"http://localhost:3000"})(okHandler).ServeHTTP(rec, req)

	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Errorf("Allow-Origin = %q, want empty", got)
	}
}

func TestCORSPreflight(t *testing.T) {
	called := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { called = true })
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodOptions, "/health", nil)
	req.Header.Set("Origin", "http://localhost:3000")
	req.Header.Set("Access-Control-Request-Method", "GET")

	CORS([]string{"http://localhost:3000"})(next).ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Errorf("status = %d, want 204", rec.Code)
	}
	if called {
		t.Error("preflight reached the next handler")
	}
	if got := rec.Header().Get("Access-Control-Allow-Methods"); got != "GET, POST, PATCH, OPTIONS" {
		t.Errorf("Allow-Methods = %q", got)
	}
}
