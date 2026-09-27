package dashboard

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

// querier is the subset of *pgxpool.Pool and pgx.Tx the repository needs.
type querier interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// PostgresRepository reads dashboard KPIs from PostgreSQL.
type PostgresRepository struct {
	db querier
}

// NewRepository returns a repository over a pool or a transaction.
func NewRepository(db querier) *PostgresRepository {
	return &PostgresRepository{db: db}
}

// summarySQL returns one row: readings aggregates, the latest run of any status, and the
// anomaly KPIs of the latest completed run (NULL columns when those runs do not exist).
const summarySQL = `
WITH latest AS (
    SELECT id, finished_at FROM analysis_runs
    WHERE status = 'COMPLETED' AND finished_at IS NOT NULL
    ORDER BY finished_at DESC, id DESC LIMIT 1
),
last_run AS (
    SELECT id, status, started_at, finished_at FROM analysis_runs
    ORDER BY started_at DESC NULLS LAST, id DESC LIMIT 1
),
stats AS (
    SELECT COUNT(*) AS detected,
           COUNT(*) FILTER (WHERE severity = 'HIGH' AND type <> 'FALSE_POSITIVE') AS high_priority,
           ROUND(AVG(confidence), 2) AS avg_confidence,
           COUNT(*) FILTER (WHERE type = 'REAL_ANOMALY') AS real_anomaly,
           COUNT(*) FILTER (WHERE type = 'EXPLAINABLE_ANOMALY') AS explainable_anomaly,
           COUNT(*) FILTER (WHERE type = 'FALSE_POSITIVE') AS false_positive,
           COUNT(*) FILTER (WHERE type = 'DATA_QUALITY') AS data_quality
    FROM anomalies WHERE analysis_id = (SELECT id FROM latest)
)
SELECT (SELECT COUNT(*) FROM meters),
       (SELECT MIN(timestamp) FROM readings),
       (SELECT MAX(timestamp) FROM readings),
       (SELECT COALESCE(ROUND(SUM(consumption_kwh), 2), 0) FROM readings),
       lr.id, lr.status, lr.started_at, lr.finished_at,
       l.id, l.finished_at,
       s.detected, s.high_priority, s.avg_confidence,
       s.real_anomaly, s.explainable_anomaly, s.false_positive, s.data_quality
FROM (SELECT 1) AS one
LEFT JOIN last_run lr ON TRUE
LEFT JOIN latest l ON TRUE
LEFT JOIN stats s ON l.id IS NOT NULL
`

// Summary returns the platform-wide KPIs.
func (r *PostgresRepository) Summary(ctx context.Context) (Summary, error) {
	var (
		s                            Summary
		periodFrom, periodTo         *time.Time
		lastID                       *int64
		lastStatus                   *string
		lastStarted, lastFinished    *time.Time
		latestID                     *int64
		latestFinished               *time.Time
		detected, high               *int
		avgConfidence                *float64
		realN, explainN, falseN, dqN *int
	)
	err := r.db.QueryRow(ctx, summarySQL).Scan(
		&s.MetersCount, &periodFrom, &periodTo, &s.TotalConsumptionKWh,
		&lastID, &lastStatus, &lastStarted, &lastFinished,
		&latestID, &latestFinished,
		&detected, &high, &avgConfidence,
		&realN, &explainN, &falseN, &dqN,
	)
	if err != nil {
		return Summary{}, fmt.Errorf("query summary: %w", err)
	}

	if periodFrom != nil && periodTo != nil {
		s.Period = &Period{From: *periodFrom, To: *periodTo}
	}
	if lastID != nil {
		s.LastAnalysis = &LastAnalysis{ID: *lastID, Status: *lastStatus, StartedAt: lastStarted, FinishedAt: lastFinished}
	}
	if latestID != nil {
		s.Anomalies = &AnomalyStats{
			AnalysisID:         *latestID,
			AnalysisFinishedAt: *latestFinished,
			Detected:           *detected,
			HighPriority:       *high,
			AvgConfidence:      avgConfidence,
			ByType: map[string]int{
				"REAL_ANOMALY":        *realN,
				"EXPLAINABLE_ANOMALY": *explainN,
				"FALSE_POSITIVE":      *falseN,
				"DATA_QUALITY":        *dqN,
			},
		}
	}
	return s, nil
}
