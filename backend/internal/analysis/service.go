package analysis

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"runtime/debug"
	"strconv"
	"sync"
	"time"

	"github.com/JuanJoseFlorez/bia-energy-mvp/backend/internal/platform/apperr"
)

const (
	// staleGrace is added to the run timeout before an active run counts as abandoned,
	// so a live run finishing its last write is never swept.
	staleGrace = 30 * time.Second
	// failTimeout bounds the write that records a failure (the run's own ctx may be done).
	failTimeout = 5 * time.Second
)

// Client-safe run error messages stored in analysis_runs.error.
const (
	msgTimedOut    = "analysis timed out"
	msgShutdown    = "interrupted by shutdown"
	msgUnavailable = "analysis engine unavailable"
	msgEngine      = "analysis engine failed"
	msgNoResult    = "analysis engine returned no result"
	msgSave        = "could not save analysis results"
	msgInternal    = "internal error"
)

// errSave marks a failure to persist the engine result.
var errSave = errors.New("save results")

// Repository is the data access the analysis service needs.
type Repository interface {
	CreateRun(ctx context.Context) (Run, error)
	MarkRunning(ctx context.Context, id int64) error
	SetStep(ctx context.Context, id int64, step string) error
	Complete(ctx context.Context, id int64, res Result, s Summary) error
	Fail(ctx context.Context, id int64, message string) error
	SweepStale(ctx context.Context, olderThan time.Duration) (int64, error)
	// Get and Latest return nil, nil when there is no such run.
	Get(ctx context.Context, id int64) (*Run, error)
	Latest(ctx context.Context) (*Run, error)
}

// Engine runs one analysis, reporting progress through onStep.
type Engine interface {
	Analyze(ctx context.Context, analysisID int64, onStep func(step string)) (Result, error)
}

// Service starts analysis runs in the background and reads their state.
type Service struct {
	repo    Repository
	engine  Engine
	timeout time.Duration

	baseCtx context.Context // cancelled by Shutdown
	cancel  context.CancelFunc
	wg      sync.WaitGroup
}

// NewService returns a service whose runs each get timeout to finish.
func NewService(repo Repository, engine Engine, timeout time.Duration) *Service {
	ctx, cancel := context.WithCancel(context.Background())
	return &Service{repo: repo, engine: engine, timeout: timeout, baseCtx: ctx, cancel: cancel}
}

// Start creates a PENDING run and executes it in the background; ErrConflict if one is active.
func (s *Service) Start(ctx context.Context) (Run, error) {
	if err := s.SweepStale(ctx); err != nil {
		return Run{}, err
	}
	run, err := s.repo.CreateRun(ctx)
	if err != nil {
		return Run{}, err
	}
	s.wg.Add(1)
	go s.execute(run.ID)
	return run, nil
}

// SweepStale fails active runs abandoned by a crashed or restarted backend.
func (s *Service) SweepStale(ctx context.Context) error {
	n, err := s.repo.SweepStale(ctx, s.timeout+staleGrace)
	if err != nil {
		return err
	}
	if n > 0 {
		slog.WarnContext(ctx, "failed stale analysis runs", "count", n)
	}
	return nil
}

// Get returns one run.
func (s *Service) Get(ctx context.Context, rawID string) (Run, error) {
	id, err := strconv.ParseInt(rawID, 10, 64)
	if err != nil || id < 1 {
		return Run{}, fmt.Errorf("invalid id: must be a positive integer: %w", apperr.ErrValidation)
	}
	run, err := s.repo.Get(ctx, id)
	if err != nil {
		return Run{}, fmt.Errorf("get run: %w", err)
	}
	if run == nil {
		return Run{}, fmt.Errorf("analysis %d: %w", id, apperr.ErrNotFound)
	}
	return *run, nil
}

// Latest returns the most recently started run.
func (s *Service) Latest(ctx context.Context) (Run, error) {
	run, err := s.repo.Latest(ctx)
	if err != nil {
		return Run{}, fmt.Errorf("latest run: %w", err)
	}
	if run == nil {
		return Run{}, fmt.Errorf("no analysis run yet: %w", apperr.ErrNotFound)
	}
	return *run, nil
}

// Shutdown cancels running analyses (they end FAILED) and waits for them or for ctx.
func (s *Service) Shutdown(ctx context.Context) error {
	s.cancel()
	done := make(chan struct{})
	go func() {
		s.wg.Wait()
		close(done)
	}()
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// execute runs one analysis to COMPLETED or FAILED; it never panics.
func (s *Service) execute(id int64) {
	defer s.wg.Done()
	ctx, cancel := context.WithTimeout(s.baseCtx, s.timeout)
	defer cancel()
	defer func() {
		if v := recover(); v != nil {
			slog.Error("analysis panicked", "analysis_id", id, "panic", v, "stack", string(debug.Stack()))
			s.fail(id, msgInternal)
		}
	}()

	started := time.Now()
	if err := s.run(ctx, id); err != nil {
		msg := s.failureMessage(ctx, err)
		slog.Error("analysis failed", "analysis_id", id, "error", err, "message", msg)
		s.fail(id, msg)
		return
	}
	slog.Info("analysis completed", "analysis_id", id, "duration_ms", time.Since(started).Milliseconds())
}

func (s *Service) run(ctx context.Context, id int64) error {
	if err := s.repo.MarkRunning(ctx, id); err != nil {
		return err
	}
	res, err := s.engine.Analyze(ctx, id, func(step string) {
		if err := s.repo.SetStep(ctx, id, step); err != nil {
			slog.Warn("record analysis step", "analysis_id", id, "step", step, "error", err)
		}
	})
	if err != nil {
		return fmt.Errorf("engine: %w", err)
	}
	if err := s.repo.Complete(ctx, id, res, summarize(res.Anomalies)); err != nil {
		return fmt.Errorf("%w: %w", errSave, err)
	}
	return nil
}

// failureMessage maps an internal error to the client-safe text stored on the run.
func (s *Service) failureMessage(ctx context.Context, err error) string {
	switch {
	case s.baseCtx.Err() != nil:
		return msgShutdown
	case errors.Is(ctx.Err(), context.DeadlineExceeded):
		return msgTimedOut
	case errors.Is(err, errEngineUnavailable):
		return msgUnavailable
	case errors.Is(err, errEngineFailed):
		return msgEngine
	case errors.Is(err, errNoResult):
		return msgNoResult
	case errors.Is(err, errSave):
		return msgSave
	default:
		return msgInternal
	}
}

func (s *Service) fail(id int64, msg string) {
	ctx, cancel := context.WithTimeout(context.Background(), failTimeout)
	defer cancel()
	if err := s.repo.Fail(ctx, id, msg); err != nil {
		slog.Error("record analysis failure", "analysis_id", id, "error", err)
	}
}

// summarize counts all anomalies and the high-priority ones (HIGH and not a false positive),
// the same rule as the dashboard's high-priority KPI.
func summarize(anomalies []AnomalyResult) Summary {
	s := Summary{AnomaliesDetected: len(anomalies)}
	for _, a := range anomalies {
		if a.Severity == "HIGH" && a.Type != "FALSE_POSITIVE" {
			s.HighPriority++
		}
	}
	return s
}
