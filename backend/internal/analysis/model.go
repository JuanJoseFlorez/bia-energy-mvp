// Package analysis runs AI analyses: it starts runs, streams progress from engine-ai
// and persists each run's metrics, anomalies and summary.
package analysis

import (
	"encoding/json"
	"time"
)

// Run statuses.
const (
	StatusPending   = "PENDING"
	StatusRunning   = "RUNNING"
	StatusCompleted = "COMPLETED"
	StatusFailed    = "FAILED"
)

// Run is one "Run AI Analysis" execution.
type Run struct {
	ID          int64      `json:"id"`
	Status      string     `json:"status"`
	CurrentStep *string    `json:"current_step"`
	StartedAt   *time.Time `json:"started_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
	FinishedAt  *time.Time `json:"finished_at"`
	Summary     *Summary   `json:"summary"`
	Error       *string    `json:"error"`
}

// Summary is the headline of a completed run, e.g. "4 anomalies · 2 high priority".
type Summary struct {
	AnomaliesDetected int `json:"anomalies_detected"`
	HighPriority      int `json:"high_priority"`
}

// Result is the payload of engine-ai's "result" stream line.
type Result struct {
	Metrics   []MetricResult  `json:"metrics"`
	Anomalies []AnomalyResult `json:"anomalies"`
}

// MetricResult is one meter's daily figures as computed by engine-ai.
type MetricResult struct {
	MeterID      string     `json:"meter_id"`
	CurrentKWh   float64    `json:"current_kwh"`
	BaselineKWh  float64    `json:"baseline_kwh"`
	VariationPct float64    `json:"variation_pct"`
	ChangeStart  *time.Time `json:"change_start"`
}

// AnomalyResult is one anomaly as computed by engine-ai; Evidence is kept verbatim.
type AnomalyResult struct {
	MeterID           string          `json:"meter_id"`
	Type              string          `json:"type"`
	Severity          string          `json:"severity"`
	Confidence        float64         `json:"confidence"`
	Priority          int             `json:"priority"`
	Reason            string          `json:"reason"`
	Explanation       string          `json:"explanation"`
	RecommendedAction string          `json:"recommended_action"`
	ExplanationSource string          `json:"explanation_source"`
	Evidence          json.RawMessage `json:"evidence"`
}

// streamLine is one NDJSON line of engine-ai's POST /analyze stream:
// {"type":"step","step":…}, {"type":"result","metrics":…,"anomalies":…} or {"type":"error","message":…}.
type streamLine struct {
	Type    string `json:"type"`
	Step    string `json:"step"`
	Message string `json:"message"`
	Result
}
