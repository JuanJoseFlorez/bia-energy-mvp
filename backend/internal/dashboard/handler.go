package dashboard

import (
	"net/http"

	"github.com/JuanJoseFlorez/bia-energy-mvp/backend/internal/platform/httpx"
)

// Handler serves the dashboard endpoint.
type Handler struct {
	svc *Service
}

// NewHandler returns a dashboard handler backed by svc.
func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

// Register mounts the dashboard routes on mux.
func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /dashboard/summary", h.summary)
}

func (h *Handler) summary(w http.ResponseWriter, r *http.Request) {
	s, err := h.svc.Summary(r.Context())
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, s)
}
