package meter

import (
	"net/http"

	"github.com/JuanJoseFlorez/bia-energy-mvp/backend/internal/platform/httpx"
)

// Handler serves the meter endpoints.
type Handler struct {
	svc *Service
}

// NewHandler returns a meter handler backed by svc.
func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

// Register mounts the meter routes on mux.
func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /meters", h.list)
	mux.HandleFunc("GET /meters/{meterId}", h.get)
	mux.HandleFunc("GET /meters/{meterId}/readings", h.readings)
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	res, err := h.svc.List(r.Context(), ListQuery{
		Status: q.Get("status"),
		Q:      q.Get("q"),
		Sort:   q.Get("sort"),
		Order:  q.Get("order"),
		Limit:  q.Get("limit"),
		Offset: q.Get("offset"),
	})
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, res)
}

func (h *Handler) get(w http.ResponseWriter, r *http.Request) {
	d, err := h.svc.Get(r.Context(), r.PathValue("meterId"))
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, d)
}

func (h *Handler) readings(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	res, err := h.svc.Readings(r.Context(), r.PathValue("meterId"), ReadingsQuery{From: q.Get("from"), To: q.Get("to")})
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, res)
}
