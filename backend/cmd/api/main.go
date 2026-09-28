// Command api runs the BIA Energy REST API.
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/JuanJoseFlorez/bia-energy-mvp/backend/internal/analysis"
	"github.com/JuanJoseFlorez/bia-energy-mvp/backend/internal/anomaly"
	"github.com/JuanJoseFlorez/bia-energy-mvp/backend/internal/auth"
	"github.com/JuanJoseFlorez/bia-energy-mvp/backend/internal/config"
	"github.com/JuanJoseFlorez/bia-energy-mvp/backend/internal/dashboard"
	"github.com/JuanJoseFlorez/bia-energy-mvp/backend/internal/health"
	"github.com/JuanJoseFlorez/bia-energy-mvp/backend/internal/meter"
	"github.com/JuanJoseFlorez/bia-energy-mvp/backend/internal/platform/database"
	"github.com/JuanJoseFlorez/bia-energy-mvp/backend/internal/platform/logger"
)

const (
	shutdownTimeout = 10 * time.Second
	healthTimeout   = 2 * time.Second
)

func main() {
	if err := run(); err != nil {
		slog.Error("api stopped with error", "error", err)
		os.Exit(1)
	}
}

// run wires config → logger → database → router → server and blocks until shutdown.
func run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	log := logger.New(cfg.LogLevel)
	slog.SetDefault(log)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	pool, err := database.Connect(ctx, cfg.DatabaseURL())
	if err != nil {
		return err
	}
	defer pool.Close()

	analyses := analysis.NewService(analysis.NewRepository(pool), analysis.NewEngineClient(cfg.AIEngineURL), cfg.AIEngineTimeout)
	// Runs left active by a previous process that died are failed before serving.
	if err := analyses.SweepStale(ctx); err != nil {
		return err
	}

	srv := &http.Server{
		Addr: fmt.Sprintf(":%d", cfg.HTTPPort),
		Handler: newRouter(log, cfg.CORSAllowedOrigins,
			health.NewHandler(pool, healthTimeout),
			meter.NewHandler(meter.NewService(meter.NewRepository(pool))),
			dashboard.NewHandler(dashboard.NewService(dashboard.NewRepository(pool))),
			analysis.NewHandler(analyses),
			anomaly.NewHandler(anomaly.NewService(anomaly.NewRepository(pool))),
			auth.NewHandler(auth.NewService(cfg.DemoUser, cfg.DemoPassword)),
		),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	serverErr := make(chan error, 1)
	go func() {
		log.Info("server listening", "addr", srv.Addr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErr <- err
		}
		close(serverErr)
	}()

	select {
	case err := <-serverErr:
		return fmt.Errorf("server: %w", err)
	case <-ctx.Done():
		log.Info("shutdown signal received")
		stop() // restore default signal behavior so a second Ctrl+C kills the process
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("shutdown: %w", err)
	}
	// Cancel in-flight analyses (they end FAILED) while the pool can still record it.
	if err := analyses.Shutdown(shutdownCtx); err != nil {
		log.Warn("analysis still running at shutdown", "error", err)
	}
	pool.Close() // idempotent; also deferred above for early-return paths
	log.Info("shutdown complete")
	return nil
}
