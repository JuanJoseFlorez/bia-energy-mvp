// Package httpx provides JSON helpers, error mapping and middleware shared by all HTTP handlers.
package httpx

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"

	"github.com/JuanJoseFlorez/bia-energy-mvp/backend/internal/platform/apperr"
)

const maxBodyBytes = 1 << 20 // 1 MB

type errorBody struct {
	Error     errorDetail `json:"error"`
	RequestID string      `json:"request_id,omitempty"`
}

type errorDetail struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// JSON writes v as a JSON response with the given status.
func JSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if v == nil {
		return
	}
	if err := json.NewEncoder(w).Encode(v); err != nil {
		slog.Error("encode response", "error", err)
	}
}

// Error maps err to an HTTP status and writes the standard error body.
// Unknown errors become 500 with a generic message; the real error is only logged.
func Error(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, apperr.ErrValidation):
		writeError(w, r, http.StatusBadRequest, "validation_error", err.Error())
	case errors.Is(err, apperr.ErrNotFound):
		writeError(w, r, http.StatusNotFound, "not_found", err.Error())
	case errors.Is(err, apperr.ErrConflict):
		writeError(w, r, http.StatusConflict, "conflict", err.Error())
	default:
		slog.ErrorContext(r.Context(), "internal error", "request_id", RequestIDFrom(r.Context()), "error", err)
		writeError(w, r, http.StatusInternalServerError, "internal_error", "internal server error")
	}
}

// NotFound is the catch-all handler for unknown routes.
func NotFound(w http.ResponseWriter, r *http.Request) {
	Error(w, r, fmt.Errorf("route %s %s: %w", r.Method, r.URL.Path, apperr.ErrNotFound))
}

// Decode reads a single JSON object from the request body into v.
// Empty, malformed, oversized, multi-object bodies and unknown fields are validation errors.
func Decode(r *http.Request, v any) error {
	dec := json.NewDecoder(http.MaxBytesReader(nil, r.Body, maxBodyBytes))
	dec.DisallowUnknownFields()

	if err := dec.Decode(v); err != nil {
		var tooLarge *http.MaxBytesError
		switch {
		case errors.Is(err, io.EOF):
			return fmt.Errorf("request body is empty: %w", apperr.ErrValidation)
		case errors.As(err, &tooLarge):
			return fmt.Errorf("request body exceeds %d bytes: %w", maxBodyBytes, apperr.ErrValidation)
		default:
			return fmt.Errorf("invalid request body: %s: %w", err.Error(), apperr.ErrValidation)
		}
	}
	if dec.More() {
		return fmt.Errorf("request body must contain a single JSON object: %w", apperr.ErrValidation)
	}
	return nil
}

func writeError(w http.ResponseWriter, r *http.Request, status int, code, message string) {
	JSON(w, status, errorBody{
		Error:     errorDetail{Code: code, Message: message},
		RequestID: RequestIDFrom(r.Context()),
	})
}
