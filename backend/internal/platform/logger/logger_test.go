package logger

import (
	"context"
	"log/slog"
	"testing"
)

func TestNewLevel(t *testing.T) {
	tests := []struct {
		level   string
		debugOn bool
		infoOn  bool
		errorOn bool
	}{
		{"debug", true, true, true},
		{"info", false, true, true},
		{"warn", false, false, true},
		{"error", false, false, true},
	}
	ctx := context.Background()
	for _, tt := range tests {
		t.Run(tt.level, func(t *testing.T) {
			log := New(tt.level)
			if got := log.Enabled(ctx, slog.LevelDebug); got != tt.debugOn {
				t.Errorf("debug enabled = %v, want %v", got, tt.debugOn)
			}
			if got := log.Enabled(ctx, slog.LevelInfo); got != tt.infoOn {
				t.Errorf("info enabled = %v, want %v", got, tt.infoOn)
			}
			if got := log.Enabled(ctx, slog.LevelError); got != tt.errorOn {
				t.Errorf("error enabled = %v, want %v", got, tt.errorOn)
			}
		})
	}
}
