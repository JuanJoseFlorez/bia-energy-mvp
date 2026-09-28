package analysis

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/JuanJoseFlorez/bia-energy-mvp/backend/internal/platform/apperr"
)

// fakeRepo is an in-memory Repository; execute runs in a goroutine, so every access is locked.
type fakeRepo struct {
	mu          sync.Mutex
	createErr   error
	sweepErr    error
	completeErr error
	run         *Run // returned by Get/Latest
	getErr      error

	sweptOlderThan time.Duration
	calls          []string // "create", "running", "step:X", "complete", "fail:msg"
	completed      Summary
}

func (f *fakeRepo) record(c string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, c)
}

func (f *fakeRepo) Calls() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return slices.Clone(f.calls)
}

func (f *fakeRepo) CreateRun(context.Context) (Run, error) {
	f.record("create")
	if f.createErr != nil {
		return Run{}, f.createErr
	}
	return Run{ID: 7, Status: StatusPending}, nil
}

func (f *fakeRepo) MarkRunning(context.Context, int64) error { f.record("running"); return nil }

func (f *fakeRepo) SetStep(_ context.Context, _ int64, step string) error {
	f.record("step:" + step)
	return nil
}

func (f *fakeRepo) Complete(_ context.Context, _ int64, _ Result, s Summary) error {
	f.record("complete")
	f.mu.Lock()
	f.completed = s
	f.mu.Unlock()
	return f.completeErr
}

func (f *fakeRepo) Fail(_ context.Context, _ int64, msg string) error {
	f.record("fail:" + msg)
	return nil
}

func (f *fakeRepo) SweepStale(_ context.Context, olderThan time.Duration) (int64, error) {
	f.mu.Lock()
	f.sweptOlderThan = olderThan
	f.mu.Unlock()
	return 0, f.sweepErr
}

func (f *fakeRepo) Get(context.Context, int64) (*Run, error) { return f.run, f.getErr }
func (f *fakeRepo) Latest(context.Context) (*Run, error)     { return f.run, f.getErr }

// fakeEngine calls onStep for steps, then returns result/err; block makes it wait for ctx.
type fakeEngine struct {
	steps  []string
	result Result
	err    error
	block  bool
	panics bool
}

func (e *fakeEngine) Analyze(ctx context.Context, _ int64, onStep func(string)) (Result, error) {
	for _, s := range e.steps {
		onStep(s)
	}
	if e.panics {
		panic("engine exploded")
	}
	if e.block {
		<-ctx.Done()
		return Result{}, ctx.Err()
	}
	return e.result, e.err
}

// startAndWait starts a run and waits for its goroutine to finish.
func startAndWait(t *testing.T, repo *fakeRepo, engine *fakeEngine, timeout time.Duration) Run {
	t.Helper()
	svc := NewService(repo, engine, timeout)
	run, err := svc.Start(context.Background())
	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	svc.wg.Wait()
	return run
}

func TestServiceStartCompletesRun(t *testing.T) {
	repo := &fakeRepo{}
	engine := &fakeEngine{
		steps: []string{"READINGS", "BASELINE"},
		result: Result{Anomalies: []AnomalyResult{
			{Type: "REAL_ANOMALY", Severity: "HIGH"},
			{Type: "DATA_QUALITY", Severity: "HIGH"},
			{Type: "EXPLAINABLE_ANOMALY", Severity: "MEDIUM"},
			{Type: "FALSE_POSITIVE", Severity: "HIGH"},
		}},
	}

	run := startAndWait(t, repo, engine, time.Minute)

	if run.ID != 7 || run.Status != StatusPending {
		t.Errorf("run = %+v, want id 7 PENDING", run)
	}
	want := []string{"create", "running", "step:READINGS", "step:BASELINE", "complete"}
	if got := repo.Calls(); !slices.Equal(got, want) {
		t.Errorf("calls = %v, want %v", got, want)
	}
	if repo.completed != (Summary{AnomaliesDetected: 4, HighPriority: 2}) {
		t.Errorf("summary = %+v, want {4 2}", repo.completed)
	}
	if repo.sweptOlderThan != time.Minute+staleGrace {
		t.Errorf("sweep threshold = %v, want %v", repo.sweptOlderThan, time.Minute+staleGrace)
	}
}

func TestServiceFailureMessages(t *testing.T) {
	tests := []struct {
		name        string
		engine      *fakeEngine
		completeErr error
		want        string
	}{
		{"engine unavailable", &fakeEngine{err: fmt.Errorf("%w: status 503", errEngineUnavailable)}, nil, msgUnavailable},
		{"engine error line", &fakeEngine{err: fmt.Errorf("%w: boom", errEngineFailed)}, nil, msgEngine},
		{"no result", &fakeEngine{err: errNoResult}, nil, msgNoResult},
		{"save fails", &fakeEngine{}, errors.New("fk violation"), msgSave},
		{"unexpected engine error", &fakeEngine{err: errors.New("weird")}, nil, msgInternal},
		{"timeout", &fakeEngine{block: true}, nil, msgTimedOut},
		{"panic", &fakeEngine{panics: true}, nil, msgInternal},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &fakeRepo{completeErr: tt.completeErr}

			startAndWait(t, repo, tt.engine, 50*time.Millisecond)

			calls := repo.Calls()
			if last := calls[len(calls)-1]; last != "fail:"+tt.want {
				t.Errorf("calls = %v, want last fail:%s", calls, tt.want)
			}
		})
	}
}

func TestServiceStartConflict(t *testing.T) {
	repo := &fakeRepo{createErr: fmt.Errorf("an analysis is already running: %w", apperr.ErrConflict)}
	svc := NewService(repo, &fakeEngine{}, time.Minute)

	_, err := svc.Start(context.Background())

	if !errors.Is(err, apperr.ErrConflict) {
		t.Errorf("Start() error = %v, want ErrConflict", err)
	}
	if got := repo.Calls(); !slices.Equal(got, []string{"create"}) {
		t.Errorf("calls = %v, want only create (no background run)", got)
	}
}

func TestServiceStartSweepError(t *testing.T) {
	repo := &fakeRepo{sweepErr: errors.New("db down")}

	_, err := NewService(repo, &fakeEngine{}, time.Minute).Start(context.Background())

	if err == nil || len(repo.Calls()) != 0 {
		t.Errorf("Start() error = %v, calls = %v; want error and no run", err, repo.Calls())
	}
}

func TestServiceShutdownInterruptsRun(t *testing.T) {
	repo := &fakeRepo{}
	svc := NewService(repo, &fakeEngine{steps: []string{"READINGS"}, block: true}, time.Minute)
	if _, err := svc.Start(context.Background()); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	for !slices.Contains(repo.Calls(), "step:READINGS") {
		time.Sleep(time.Millisecond)
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := svc.Shutdown(ctx); err != nil {
		t.Fatalf("Shutdown() error = %v", err)
	}

	calls := repo.Calls()
	if last := calls[len(calls)-1]; last != "fail:"+msgShutdown {
		t.Errorf("calls = %v, want last fail:%s", calls, msgShutdown)
	}
}

func TestServiceShutdownTimesOut(t *testing.T) {
	svc := NewService(&fakeRepo{}, &fakeEngine{}, time.Minute)
	svc.wg.Add(1) // a run that never finishes
	defer svc.wg.Done()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	if err := svc.Shutdown(ctx); !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("Shutdown() error = %v, want DeadlineExceeded", err)
	}
}

func TestServiceGet(t *testing.T) {
	tests := []struct {
		name  string
		rawID string
		repo  *fakeRepo
		want  error
	}{
		{"found", "7", &fakeRepo{run: &Run{ID: 7}}, nil},
		{"not found", "7", &fakeRepo{}, apperr.ErrNotFound},
		{"not a number", "abc", &fakeRepo{}, apperr.ErrValidation},
		{"zero", "0", &fakeRepo{}, apperr.ErrValidation},
		{"repository error", "7", &fakeRepo{getErr: errors.New("db down")}, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			run, err := NewService(tt.repo, &fakeEngine{}, time.Minute).Get(context.Background(), tt.rawID)

			switch {
			case tt.repo.getErr != nil:
				if err == nil || errors.Is(err, apperr.ErrNotFound) || errors.Is(err, apperr.ErrValidation) {
					t.Errorf("error = %v, want internal error", err)
				}
			case tt.want != nil:
				if !errors.Is(err, tt.want) {
					t.Errorf("error = %v, want %v", err, tt.want)
				}
			default:
				if err != nil || run.ID != 7 {
					t.Errorf("Get() = %+v, %v", run, err)
				}
			}
		})
	}
}

func TestServiceLatestNotFound(t *testing.T) {
	_, err := NewService(&fakeRepo{}, &fakeEngine{}, time.Minute).Latest(context.Background())

	if !errors.Is(err, apperr.ErrNotFound) {
		t.Errorf("Latest() error = %v, want ErrNotFound", err)
	}
}
