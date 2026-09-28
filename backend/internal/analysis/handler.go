package analysis

import (
	"fmt"
	"net/http"

	"github.com/JuanJoseFlorez/bia-energy-mvp/backend/internal/platform/httpx"
)

// Handler serves the analysis run endpoints.
type Handler struct {
	svc *Service
}

// NewHandler returns an analysis handler backed by svc.
func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

// Register mounts the analysis routes on mux.
func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("POST /ai/analyze", h.start)
	mux.HandleFunc("GET /ai/analysis/latest", h.latest)
	mux.HandleFunc("GET /ai/analysis/{id}", h.get)
}

func (h *Handler) start(w http.ResponseWriter, r *http.Request) {
	run, err := h.svc.Start(r.Context())
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	w.Header().Set("Location", fmt.Sprintf("/ai/analysis/%d", run.ID))
	httpx.JSON(w, http.StatusAccepted, run)
}

func (h *Handler) latest(w http.ResponseWriter, r *http.Request) {
	run, err := h.svc.Latest(r.Context())
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, run)
}

func (h *Handler) get(w http.ResponseWriter, r *http.Request) {
	run, err := h.svc.Get(r.Context(), r.PathValue("id"))
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, run)
}
