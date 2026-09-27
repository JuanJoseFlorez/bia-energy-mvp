package main

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/JuanJoseFlorez/bia-energy-mvp/backend/internal/health"
	"github.com/JuanJoseFlorez/bia-energy-mvp/backend/internal/platform/httpx"
)

const healthTimeout = 2 * time.Second

// newRouter mounts every domain's routes and wraps them with the middleware chain.
// New domains register here: <domain>.NewHandler(...).Register(mux).
func newRouter(log *slog.Logger, allowedOrigins []string, db health.Pinger) http.Handler {
	mux := http.NewServeMux()

	health.NewHandler(db, healthTimeout).Register(mux)

	return httpx.Chain(httpx.Routes(mux),
		httpx.RequestID,
		httpx.Logger(log),
		httpx.Recover(log),
		httpx.CORS(allowedOrigins),
	)
}
