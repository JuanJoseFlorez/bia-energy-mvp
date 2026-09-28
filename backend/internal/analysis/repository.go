package analysis

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/JuanJoseFlorez/bia-energy-mvp/backend/internal/platform/apperr"
)

// errRunNotActive means the run is no longer PENDING/RUNNING (e.g. a sweep already failed it).
var errRunNotActive = errors.New("analysis run is no longer active")

// db is the subset of *pgxpool.Pool and pgx.Tx the repository needs (Begin on a pgx.Tx is a savepoint).
type db interface {
	Begin(ctx context.Context) (pgx.Tx, error)
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// PostgresRepository stores analysis runs and their results in PostgreSQL.
type PostgresRepository struct {
	db db
}

// NewRepository returns a repository over a pool or a transaction.
func NewRepository(db db) *PostgresRepository {
	return &PostgresRepository{db: db}
}

// runColumns must match the scan order in scanRun.
const runColumns = `id, status, current_step, started_at, updated_at, finished_at, summary, error`

func scanRun(row pgx.Row) (Run, error) {
	var r Run
	err := row.Scan(&r.ID, &r.Status, &r.CurrentStep, &r.StartedAt, &r.UpdatedAt, &r.FinishedAt, &r.Summary, &r.Error)
	return r, err
}

// CreateRun inserts a PENDING run. The partial unique index uq_analysis_runs_active allows one
// active run; ON CONFLICT DO NOTHING turns a concurrent second insert into "no row" without an error.
func (r *PostgresRepository) CreateRun(ctx context.Context) (Run, error) {
	run, err := scanRun(r.db.QueryRow(ctx, `
INSERT INTO analysis_runs DEFAULT VALUES
ON CONFLICT ((1)) WHERE status IN ('PENDING', 'RUNNING') DO NOTHING
RETURNING `+runColumns))
	if errors.Is(err, pgx.ErrNoRows) {
		return Run{}, fmt.Errorf("an analysis is already running: %w", apperr.ErrConflict)
	}
	if err != nil {
		return Run{}, fmt.Errorf("insert run: %w", err)
	}
	return run, nil
}

// MarkRunning moves a PENDING run to RUNNING.
func (r *PostgresRepository) MarkRunning(ctx context.Context, id int64) error {
	return r.update(ctx, "mark running", `
UPDATE analysis_runs SET status = 'RUNNING', updated_at = now()
WHERE id = $1 AND status = 'PENDING'`, id)
}

// SetStep records the step engine-ai is working on and bumps the heartbeat.
func (r *PostgresRepository) SetStep(ctx context.Context, id int64, step string) error {
	return r.update(ctx, "set step", `
UPDATE analysis_runs SET current_step = $2, updated_at = now()
WHERE id = $1 AND status = 'RUNNING'`, id, step)
}

// Fail marks an active run FAILED with a client-safe message; an inactive run is left as is.
func (r *PostgresRepository) Fail(ctx context.Context, id int64, message string) error {
	_, err := r.db.Exec(ctx, `
UPDATE analysis_runs SET status = 'FAILED', error = $2, finished_at = now(), updated_at = now()
WHERE id = $1 AND status IN ('PENDING', 'RUNNING')`, id, message)
	if err != nil {
		return fmt.Errorf("fail run: %w", err)
	}
	return nil
}

// Complete stores the run's metrics, anomalies and summary and marks it COMPLETED, atomically.
func (r *PostgresRepository) Complete(ctx context.Context, id int64, res Result, s Summary) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }() // no-op after Commit

	for _, m := range res.Metrics {
		_, err := tx.Exec(ctx, `
INSERT INTO meter_metrics (analysis_id, meter_id, current_kwh, baseline_kwh, variation_pct, change_start)
VALUES ($1, $2, $3, $4, $5, $6)`,
			id, m.MeterID, m.CurrentKWh, m.BaselineKWh, m.VariationPct, m.ChangeStart)
		if err != nil {
			return fmt.Errorf("insert metrics for %s: %w", m.MeterID, err)
		}
	}
	for _, a := range res.Anomalies {
		_, err := tx.Exec(ctx, `
INSERT INTO anomalies (analysis_id, meter_id, type, severity, confidence, priority,
                       reason, explanation, recommended_action, explanation_source, evidence)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)`,
			id, a.MeterID, a.Type, a.Severity, a.Confidence, a.Priority,
			a.Reason, a.Explanation, a.RecommendedAction, a.ExplanationSource, a.Evidence)
		if err != nil {
			return fmt.Errorf("insert anomaly for %s: %w", a.MeterID, err)
		}
	}
	tag, err := tx.Exec(ctx, `
UPDATE analysis_runs SET status = 'COMPLETED', summary = $2, finished_at = now(), updated_at = now()
WHERE id = $1 AND status = 'RUNNING'`, id, s)
	if err != nil {
		return fmt.Errorf("complete run: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("complete run %d: %w", id, errRunNotActive)
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit: %w", err)
	}
	return nil
}

// SweepStale fails active runs whose heartbeat is older than olderThan and returns how many.
func (r *PostgresRepository) SweepStale(ctx context.Context, olderThan time.Duration) (int64, error) {
	tag, err := r.db.Exec(ctx, `
UPDATE analysis_runs
SET status = 'FAILED', error = 'interrupted: backend restarted', finished_at = now(), updated_at = now()
WHERE status IN ('PENDING', 'RUNNING') AND updated_at < now() - make_interval(secs => $1)`,
		olderThan.Seconds())
	if err != nil {
		return 0, fmt.Errorf("sweep stale runs: %w", err)
	}
	return tag.RowsAffected(), nil
}

// Get returns the run, or nil when it does not exist.
func (r *PostgresRepository) Get(ctx context.Context, id int64) (*Run, error) {
	return r.one(ctx, `SELECT `+runColumns+` FROM analysis_runs WHERE id = $1`, id)
}

// Latest returns the most recently started run of any status, or nil when there is none.
func (r *PostgresRepository) Latest(ctx context.Context) (*Run, error) {
	return r.one(ctx, `SELECT `+runColumns+` FROM analysis_runs ORDER BY started_at DESC NULLS LAST, id DESC LIMIT 1`)
}

func (r *PostgresRepository) one(ctx context.Context, query string, args ...any) (*Run, error) {
	run, err := scanRun(r.db.QueryRow(ctx, query, args...))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("query run: %w", err)
	}
	return &run, nil
}

// update runs a single-row state UPDATE; zero affected rows means the run is not active any more.
func (r *PostgresRepository) update(ctx context.Context, op, query string, args ...any) error {
	tag, err := r.db.Exec(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("%s: %w", op, err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("%s: %w", op, errRunNotActive)
	}
	return nil
}
