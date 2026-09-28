package anomaly

import (
	"net/http"

	"github.com/JuanJoseFlorez/bia-energy-mvp/backend/internal/platform/httpx"
)

// Handler serves the anomaly endpoints.
type Handler struct {
	svc *Service
}

// NewHandler returns an anomaly handler backed by svc.
func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

// Register mounts the anomaly routes on mux.
func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /anomalies", h.list)
	mux.HandleFunc("GET /anomalies/{id}", h.get)
	mux.HandleFunc("PATCH /anomalies/{id}", h.updateStatus)
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	res, err := h.svc.List(r.Context(), ListQuery{
		AnalysisID: q.Get("analysis_id"),
		Type:       q.Get("type"),
		Severity:   q.Get("severity"),
		Status:     q.Get("status"),
	})
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, res)
}

func (h *Handler) get(w http.ResponseWriter, r *http.Request) {
	d, err := h.svc.Get(r.Context(), r.PathValue("id"))
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, d)
}

func (h *Handler) updateStatus(w http.ResponseWriter, r *http.Request) {
	var body StatusUpdate
	if err := httpx.Decode(r, &body); err != nil {
		httpx.Error(w, r, err)
		return
	}
	d, err := h.svc.UpdateStatus(r.Context(), r.PathValue("id"), body)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, d)
}
