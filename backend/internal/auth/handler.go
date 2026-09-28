package auth

import (
	"net/http"

	"github.com/JuanJoseFlorez/bia-energy-mvp/backend/internal/platform/httpx"
)

// Handler serves the auth endpoints.
type Handler struct {
	svc *Service
}

// NewHandler returns an auth handler backed by svc.
func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

// Register mounts the auth routes on mux.
func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("POST /auth/login", h.login)
}

func (h *Handler) login(w http.ResponseWriter, r *http.Request) {
	var c Credentials
	if err := httpx.Decode(r, &c); err != nil {
		httpx.Error(w, r, err)
		return
	}
	s, err := h.svc.Login(c)
	if err != nil {
		httpx.Error(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, s)
}
