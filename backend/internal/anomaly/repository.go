package anomaly

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// querier is the subset of *pgxpool.Pool and pgx.Tx the repository needs.
type querier interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// PostgresRepository reads and updates anomalies in PostgreSQL.
type PostgresRepository struct {
	db querier
}

// NewRepository returns a repository over a pool or a transaction.
func NewRepository(db querier) *PostgresRepository {
	return &PostgresRepository{db: db}
}

// itemColumns must match the scan order in scanItem.
const itemColumns = `id, analysis_id, meter_id, detected_at, type, severity, ROUND(confidence, 2), priority,
       reason, recommended_action, explanation_source, status`

func scanItem(row pgx.Row, extra ...any) (Item, error) {
	var it Item
	dest := []any{&it.ID, &it.AnalysisID, &it.MeterID, &it.DetectedAt, &it.Type, &it.Severity, &it.Confidence,
		&it.Priority, &it.Reason, &it.RecommendedAction, &it.ExplanationSource, &it.Status}
	if err := row.Scan(append(dest, extra...)...); err != nil {
		return Item{}, err
	}
	it.IsAnomaly = it.Type != nil && *it.Type != typeFalsePositive
	return it, nil
}

// LatestCompletedRun returns the id of the latest completed run, or nil when there is none.
func (r *PostgresRepository) LatestCompletedRun(ctx context.Context) (*int64, error) {
	var id int64
	err := r.db.QueryRow(ctx, `
SELECT id FROM analysis_runs
WHERE status = 'COMPLETED' AND finished_at IS NOT NULL
ORDER BY finished_at DESC, id DESC LIMIT 1`).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("query latest completed run: %w", err)
	}
	return &id, nil
}

// RunExists reports whether the analysis run exists.
func (r *PostgresRepository) RunExists(ctx context.Context, id int64) (bool, error) {
	var ok bool
	if err := r.db.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM analysis_runs WHERE id = $1)`, id).Scan(&ok); err != nil {
		return false, fmt.Errorf("query run exists: %w", err)
	}
	return ok, nil
}

// List returns the run's anomalies matching f, in investigation order.
func (r *PostgresRepository) List(ctx context.Context, runID int64, f Filters) ([]Item, error) {
	rows, err := r.db.Query(ctx, `
SELECT `+itemColumns+`
FROM anomalies
WHERE analysis_id = $1
  AND ($2 = '' OR type = $2)
  AND ($3 = '' OR severity = $3)
  AND ($4 = '' OR status = $4)
ORDER BY priority ASC NULLS LAST, id ASC`, runID, f.Type, f.Severity, f.Status)
	if err != nil {
		return nil, fmt.Errorf("query anomalies: %w", err)
	}
	defer rows.Close()

	var items []Item
	for rows.Next() {
		it, err := scanItem(rows)
		if err != nil {
			return nil, fmt.Errorf("scan anomaly: %w", err)
		}
		items = append(items, it)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate anomalies: %w", err)
	}
	return items, nil
}

// Get returns the anomaly with explanation and evidence, or nil when it does not exist.
// RelatedEvents, ReadingsWindow and NextStatuses are left for the service to fill.
func (r *PostgresRepository) Get(ctx context.Context, id int64) (*Detail, error) {
	var d Detail
	it, err := scanItem(r.db.QueryRow(ctx,
		`SELECT `+itemColumns+`, explanation, evidence FROM anomalies WHERE id = $1`, id),
		&d.Explanation, &d.Evidence)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("query anomaly: %w", err)
	}
	d.Item = it
	return &d, nil
}

// RelatedEvents returns the events of the anomaly's meter listed in evidence.related_event_ids.
func (r *PostgresRepository) RelatedEvents(ctx context.Context, anomalyID int64) ([]Event, error) {
	rows, err := r.db.Query(ctx, `
SELECT e.id, e.meter_id, e.event_timestamp, e.event_type, e.description
FROM anomalies a
JOIN events e ON e.meter_id = a.meter_id AND a.evidence->'related_event_ids' @> to_jsonb(e.id)
WHERE a.id = $1
ORDER BY e.event_timestamp, e.id`, anomalyID)
	if err != nil {
		return nil, fmt.Errorf("query related events: %w", err)
	}
	defer rows.Close()

	events := []Event{}
	for rows.Next() {
		var e Event
		if err := rows.Scan(&e.ID, &e.MeterID, &e.Timestamp, &e.Type, &e.Description); err != nil {
			return nil, fmt.Errorf("scan event: %w", err)
		}
		events = append(events, e)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate events: %w", err)
	}
	return events, nil
}

// ReadingsBounds returns the first and last reading timestamps of the meter (nil when it has none).
func (r *PostgresRepository) ReadingsBounds(ctx context.Context, meterID string) (first, last *time.Time, err error) {
	err = r.db.QueryRow(ctx, `SELECT MIN(timestamp), MAX(timestamp) FROM readings WHERE meter_id = $1`, meterID).
		Scan(&first, &last)
	if err != nil {
		return nil, nil, fmt.Errorf("query readings bounds: %w", err)
	}
	return first, last, nil
}

// UpdateStatus sets the status only if it still equals from; false means it changed meanwhile.
func (r *PostgresRepository) UpdateStatus(ctx context.Context, id int64, from, to string) (bool, error) {
	tag, err := r.db.Exec(ctx, `UPDATE anomalies SET status = $3 WHERE id = $1 AND status = $2`, id, from, to)
	if err != nil {
		return false, fmt.Errorf("update anomaly status: %w", err)
	}
	return tag.RowsAffected() == 1, nil
}
