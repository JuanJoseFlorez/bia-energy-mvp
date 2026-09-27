// Package logger builds the structured JSON logger used across the service.
package logger

import (
	"log/slog"
	"os"
)

var levels = map[string]slog.Level{
	"debug": slog.LevelDebug,
	"info":  slog.LevelInfo,
	"warn":  slog.LevelWarn,
	"error": slog.LevelError,
}

// New returns a JSON logger writing to stdout at the given level ("info" if unknown).
func New(level string) *slog.Logger {
	lvl, ok := levels[level]
	if !ok {
		lvl = slog.LevelInfo
	}
	return slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: lvl}))
}
