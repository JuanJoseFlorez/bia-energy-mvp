package auth

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func post(t *testing.T, body string) *httptest.ResponseRecorder {
	t.Helper()
	mux := http.NewServeMux()
	NewHandler(NewService("demo", "demo")).Register(mux)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/auth/login", strings.NewReader(body)))
	return rec
}

func TestHandlerLogin(t *testing.T) {
	rec := post(t, `{"username":"demo","password":"demo"}`)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body %s", rec.Code, rec.Body)
	}
	var s Session
	if err := json.Unmarshal(rec.Body.Bytes(), &s); err != nil || s.User.Username != "demo" || !hexToken.MatchString(s.Token) {
		t.Errorf("body = %s (%v)", rec.Body, err)
	}
}

func TestHandlerLoginErrors(t *testing.T) {
	tests := []struct {
		name, body string
		want       int
		wantCode   string
	}{
		{"wrong password", `{"username":"demo","password":"x"}`, http.StatusUnauthorized, "unauthorized"},
		{"unknown field", `{"username":"demo","password":"demo","remember":true}`, http.StatusBadRequest, "validation_error"},
		{"empty body", ``, http.StatusBadRequest, "validation_error"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := post(t, tt.body)

			if rec.Code != tt.want {
				t.Fatalf("status = %d, want %d", rec.Code, tt.want)
			}
			var body struct {
				Error struct{ Code string } `json:"error"`
			}
			if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil || body.Error.Code != tt.wantCode {
				t.Errorf("body = %s, want code %s", rec.Body, tt.wantCode)
			}
		})
	}
}
