package main

import (
	"log/slog"
	"net/http"

	"github.com/JuanJoseFlorez/bia-energy-mvp/backend/internal/platform/httpx"
)

// registrar is anything that mounts its routes on a mux (every domain Handler).
type registrar interface {
	Register(mux *http.ServeMux)
}

// newRouter mounts every domain's routes and wraps them with the middleware chain.
// Domain handlers are built in main.go and passed in, so tests can use fakes.
func newRouter(log *slog.Logger, allowedOrigins []string, handlers ...registrar) http.Handler {
	mux := http.NewServeMux()
	for _, h := range handlers {
		h.Register(mux)
	}

	return httpx.Chain(httpx.Routes(mux),
		httpx.RequestID,
		httpx.Logger(log),
		httpx.Recover(log),
		httpx.CORS(allowedOrigins),
	)
}
