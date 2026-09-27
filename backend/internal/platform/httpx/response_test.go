package httpx

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/JuanJoseFlorez/bia-energy-mvp/backend/internal/platform/apperr"
)

type errorResponse struct {
	Error struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
	RequestID string `json:"request_id"`
}

func TestJSON(t *testing.T) {
	rec := httptest.NewRecorder()

	JSON(rec, http.StatusCreated, map[string]string{"a": "b"})

	if rec.Code != http.StatusCreated {
		t.Errorf("status = %d, want 201", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}
	if body := strings.TrimSpace(rec.Body.String()); body != `{"a":"b"}` {
		t.Errorf("body = %s, want {\"a\":\"b\"}", body)
	}
}

func TestErrorMapping(t *testing.T) {
	tests := []struct {
		name        string
		err         error
		wantStatus  int
		wantCode    string
		wantMessage string
	}{
		{"validation", fmt.Errorf("bad input: %w", apperr.ErrValidation), 400, "validation_error", "bad input: validation failed"},
		{"not found", fmt.Errorf("meter M-999: %w", apperr.ErrNotFound), 404, "not_found", "meter M-999: not found"},
		{"conflict", fmt.Errorf("run already active: %w", apperr.ErrConflict), 409, "conflict", "run already active: conflict"},
		{"unknown hides details", errors.New("db password leaked"), 500, "internal_error", "internal server error"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/x", nil)
			req = req.WithContext(WithRequestID(req.Context(), "req-1"))
			rec := httptest.NewRecorder()

			Error(rec, req, tt.err)

			if rec.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d", rec.Code, tt.wantStatus)
			}
			var body errorResponse
			if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
				t.Fatalf("invalid JSON body: %v", err)
			}
			if body.Error.Code != tt.wantCode {
				t.Errorf("code = %q, want %q", body.Error.Code, tt.wantCode)
			}
			if body.Error.Message != tt.wantMessage {
				t.Errorf("message = %q, want %q", body.Error.Message, tt.wantMessage)
			}
			if body.RequestID != "req-1" {
				t.Errorf("request_id = %q, want req-1", body.RequestID)
			}
		})
	}
}

func TestNotFound(t *testing.T) {
	rec := httptest.NewRecorder()

	NotFound(rec, httptest.NewRequest(http.MethodGet, "/nope", nil))

	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", rec.Code)
	}
	var body errorResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid JSON body: %v", err)
	}
	if body.Error.Code != "not_found" {
		t.Errorf("code = %q, want not_found", body.Error.Code)
	}
}

type payload struct {
	Name string `json:"name"`
}

func TestDecodeValid(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/x", strings.NewReader(`{"name":"M-101"}`))

	var p payload
	if err := Decode(req, &p); err != nil {
		t.Fatalf("Decode() error = %v", err)
	}
	if p.Name != "M-101" {
		t.Errorf("Name = %q, want M-101", p.Name)
	}
}

func TestDecodeRejects(t *testing.T) {
	tests := []struct {
		name, body string
	}{
		{"unknown field", `{"name":"a","extra":1}`},
		{"malformed", `{"name":`},
		{"empty", ``},
		{"trailing data", `{"name":"a"}{"name":"b"}`},
		{"too large", `{"name":"` + strings.Repeat("a", 1<<20) + `"}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/x", strings.NewReader(tt.body))

			var p payload
			err := Decode(req, &p)
			if !errors.Is(err, apperr.ErrValidation) {
				t.Errorf("Decode() error = %v, want ErrValidation", err)
			}
		})
	}
}
