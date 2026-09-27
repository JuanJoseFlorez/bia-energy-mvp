package meter

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

// querier is the subset of *pgxpool.Pool and pgx.Tx the repository needs.
type querier interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// PostgresRepository reads meters, readings and events from PostgreSQL.
type PostgresRepository struct {
	db querier
}

// NewRepository returns a repository over a pool or a transaction.
func NewRepository(db querier) *PostgresRepository {
	return &PostgresRepository{db: db}
}

// overviewSQL builds one row per meter with period consumption, the latest completed run's
// metrics and top anomaly, and the derived health. Callers append a SELECT over "overview".
const overviewSQL = `
WITH latest AS (
    SELECT id FROM analysis_runs
    WHERE status = 'COMPLETED' AND finished_at IS NOT NULL
    ORDER BY finished_at DESC, id DESC LIMIT 1
),
period AS (
    SELECT meter_id, ROUND(SUM(consumption_kwh), 2) AS period_consumption_kwh
    FROM readings GROUP BY meter_id
),
top_anomaly AS (
    SELECT DISTINCT ON (a.meter_id) a.meter_id, a.id, a.type, a.severity, a.confidence, a.priority
    FROM anomalies a JOIN latest l ON a.analysis_id = l.id
    ORDER BY a.meter_id, a.priority ASC NULLS LAST, a.id
),
overview AS (
    SELECT m.id, m.meter_id, m.name, m.location, m.status, m.created_at,
           COALESCE(p.period_consumption_kwh, 0) AS period_consumption_kwh,
           ROUND(mm.current_kwh, 2) AS current_kwh, ROUND(mm.baseline_kwh, 2) AS baseline_kwh,
           ROUND(mm.variation_pct, 1) AS variation_pct, mm.change_start,
           ta.id AS anomaly_id, ta.type AS anomaly_type, ta.severity AS anomaly_severity,
           ROUND(ta.confidence, 2) AS anomaly_confidence, ta.priority AS anomaly_priority,
           CASE
               WHEN NOT EXISTS (SELECT 1 FROM latest)                   THEN NULL
               WHEN ta.type = 'REAL_ANOMALY' AND ta.severity = 'HIGH'   THEN 'CRITICAL'
               WHEN ta.type IS NOT NULL AND ta.type <> 'FALSE_POSITIVE' THEN 'ALERT'
               ELSE 'OK'
           END AS health,
           CASE ta.severity WHEN 'HIGH' THEN 3 WHEN 'MEDIUM' THEN 2 WHEN 'LOW' THEN 1 ELSE 0 END AS severity_rank
    FROM meters m
    LEFT JOIN period p ON p.meter_id = m.meter_id
    LEFT JOIN meter_metrics mm ON mm.meter_id = m.meter_id AND mm.analysis_id = (SELECT id FROM latest)
    LEFT JOIN top_anomaly ta ON ta.meter_id = m.meter_id
)
`

// overviewColumns must match the scan order in scanMeter.
const overviewColumns = `id, meter_id, name, location, status, created_at, period_consumption_kwh,
       current_kwh, baseline_kwh, variation_pct, change_start,
       anomaly_id, anomaly_type, anomaly_severity, anomaly_confidence, anomaly_priority, health`

// listFilters uses $1 = health filter ("" = all) and $2 = LIKE-escaped search ("" = none).
const listFilters = `
WHERE ($1 = '' OR health = $1)
  AND ($2 = '' OR meter_id ILIKE '%' || $2 || '%' ESCAPE '\')`

// sortColumns maps validated sort keys to fixed SQL expressions; user input never reaches ORDER BY.
var sortColumns = map[string]string{
	SortMeterID:     "meter_id",
	SortConsumption: "current_kwh",
	SortVariation:   "variation_pct",
	SortSeverity:    "severity_rank",
}

// List returns one page of meters and the total count after filters.
func (r *PostgresRepository) List(ctx context.Context, p ListParams) ([]Meter, int, error) {
	col, ok := sortColumns[p.Sort]
	if !ok {
		return nil, 0, fmt.Errorf("unknown sort key %q", p.Sort)
	}
	dir := "ASC"
	if p.Desc {
		dir = "DESC"
	}
	query := overviewSQL + `SELECT ` + overviewColumns + `, COUNT(*) OVER () AS total FROM overview` +
		listFilters + ` ORDER BY ` + col + ` ` + dir + ` NULLS LAST, meter_id ASC LIMIT $3 OFFSET $4`

	rows, err := r.db.Query(ctx, query, p.Health, p.Search, p.Limit, p.Offset)
	if err != nil {
		return nil, 0, fmt.Errorf("query meters: %w", err)
	}
	defer rows.Close()

	var (
		items []Meter
		total int
	)
	for rows.Next() {
		m, err := scanMeter(rows, &total)
		if err != nil {
			return nil, 0, fmt.Errorf("scan meter: %w", err)
		}
		items = append(items, m)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate meters: %w", err)
	}

	// COUNT(*) OVER () has no row to ride on when the page is past the end.
	if len(items) == 0 && p.Offset > 0 {
		countQuery := overviewSQL + `SELECT COUNT(*) FROM overview` + listFilters
		if err := r.db.QueryRow(ctx, countQuery, p.Health, p.Search).Scan(&total); err != nil {
			return nil, 0, fmt.Errorf("count meters: %w", err)
		}
	}
	return items, total, nil
}

// Get returns one meter with its full anomaly and events, or nil when it does not exist.
func (r *PostgresRepository) Get(ctx context.Context, meterID string) (*MeterDetail, error) {
	query := overviewSQL + `SELECT ` + overviewColumns + ` FROM overview WHERE meter_id = $1`
	m, err := scanMeter(r.db.QueryRow(ctx, query, meterID))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("query meter: %w", err)
	}

	d := &MeterDetail{Meter: m}
	if d.Events, err = r.events(ctx, meterID); err != nil {
		return nil, err
	}
	if m.Anomaly != nil {
		a := AnomalyDetail{AnomalySummary: *m.Anomaly}
		err := r.db.QueryRow(ctx,
			`SELECT reason, recommended_action, status, detected_at FROM anomalies WHERE id = $1`, m.Anomaly.ID,
		).Scan(&a.Reason, &a.RecommendedAction, &a.Status, &a.DetectedAt)
		if err != nil {
			return nil, fmt.Errorf("query anomaly: %w", err)
		}
		d.Anomaly = &a
	}
	return d, nil
}

// Exists reports whether the meter exists.
func (r *PostgresRepository) Exists(ctx context.Context, meterID string) (bool, error) {
	var ok bool
	err := r.db.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM meters WHERE meter_id = $1)`, meterID).Scan(&ok)
	if err != nil {
		return false, fmt.Errorf("query meter exists: %w", err)
	}
	return ok, nil
}

// Readings returns the meter's readings in ascending time; nil bounds are unbounded.
func (r *PostgresRepository) Readings(ctx context.Context, meterID string, p ReadingsParams) ([]Reading, error) {
	rows, err := r.db.Query(ctx, `
SELECT id, meter_id, timestamp, consumption_kwh, voltage_v, current_a, power_factor, status
FROM readings
WHERE meter_id = $1
  AND ($2::timestamp IS NULL OR timestamp >= $2)
  AND ($3::timestamp IS NULL OR timestamp <= $3)
ORDER BY timestamp`, meterID, p.From, p.To)
	if err != nil {
		return nil, fmt.Errorf("query readings: %w", err)
	}
	defer rows.Close()

	var items []Reading
	for rows.Next() {
		var rd Reading
		if err := rows.Scan(&rd.ID, &rd.MeterID, &rd.Timestamp, &rd.ConsumptionKWh, &rd.VoltageV,
			&rd.CurrentA, &rd.PowerFactor, &rd.Status); err != nil {
			return nil, fmt.Errorf("scan reading: %w", err)
		}
		items = append(items, rd)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate readings: %w", err)
	}
	return items, nil
}

func (r *PostgresRepository) events(ctx context.Context, meterID string) ([]Event, error) {
	rows, err := r.db.Query(ctx, `
SELECT id, meter_id, event_timestamp, event_type, description
FROM events WHERE meter_id = $1
ORDER BY event_timestamp, id`, meterID)
	if err != nil {
		return nil, fmt.Errorf("query events: %w", err)
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

// scanMeter scans one overview row (overviewColumns order) plus any extra trailing columns.
func scanMeter(row pgx.Row, extra ...any) (Meter, error) {
	var (
		m                            Meter
		current, baseline, variation *float64
		changeStart                  *time.Time
		anomalyID                    *int64
		a                            AnomalySummary
	)
	dest := []any{
		&m.ID, &m.MeterID, &m.Name, &m.Location, &m.Status, &m.CreatedAt, &m.PeriodConsumptionKWh,
		&current, &baseline, &variation, &changeStart,
		&anomalyID, &a.Type, &a.Severity, &a.Confidence, &a.Priority, &m.Health,
	}
	if err := row.Scan(append(dest, extra...)...); err != nil {
		return Meter{}, err
	}
	if current != nil && baseline != nil && variation != nil {
		m.Metrics = &Metrics{CurrentKWh: *current, BaselineKWh: *baseline, VariationPct: *variation, ChangeStart: changeStart}
	}
	if anomalyID != nil {
		a.ID = *anomalyID
		a.MeterID = m.MeterID
		m.Anomaly = &a
	}
	return m, nil
}
