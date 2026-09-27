// Package health exposes the service liveness/readiness endpoint.
package health

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/JuanJoseFlorez/bia-energy-mvp/backend/internal/platform/httpx"
)

// Pinger is the only capability health needs from the database.
type Pinger interface {
	Ping(ctx context.Context) error
}

// Handler serves GET /health.
type Handler struct {
	db      Pinger
	timeout time.Duration
}

type response struct {
	Status   string `json:"status"`
	Database string `json:"database"`
}

// NewHandler returns a health handler that pings db with the given timeout.
func NewHandler(db Pinger, timeout time.Duration) *Handler {
	return &Handler{db: db, timeout: timeout}
}

// Register mounts the health routes on mux.
func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /health", h.check)
}

func (h *Handler) check(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), h.timeout)
	defer cancel()

	if err := h.db.Ping(ctx); err != nil {
		slog.WarnContext(ctx, "health check failed", "request_id", httpx.RequestIDFrom(ctx), "error", err)
		httpx.JSON(w, http.StatusServiceUnavailable, response{Status: "degraded", Database: "down"})
		return
	}
	httpx.JSON(w, http.StatusOK, response{Status: "ok", Database: "up"})
}
