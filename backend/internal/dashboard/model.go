// Package dashboard serves the platform-wide KPI summary.
package dashboard

import "time"

// Summary is GET /dashboard/summary.
type Summary struct {
	MetersCount         int           `json:"meters_count"`
	Period              *Period       `json:"period"`
	TotalConsumptionKWh float64       `json:"total_consumption_kwh"`
	LastAnalysis        *LastAnalysis `json:"last_analysis"`
	Anomalies           *AnomalyStats `json:"anomalies"`
}

// Period is the time span covered by the readings.
type Period struct {
	From time.Time `json:"from"`
	To   time.Time `json:"to"`
}

// LastAnalysis is the most recent analysis run of any status.
type LastAnalysis struct {
	ID         int64      `json:"id"`
	Status     string     `json:"status"`
	StartedAt  *time.Time `json:"started_at"`
	FinishedAt *time.Time `json:"finished_at"`
}

// AnomalyStats are the KPIs of the latest completed run, identified by AnalysisID.
type AnomalyStats struct {
	AnalysisID         int64          `json:"analysis_id"`
	AnalysisFinishedAt time.Time      `json:"analysis_finished_at"`
	Detected           int            `json:"detected"`
	HighPriority       int            `json:"high_priority"`
	AvgConfidence      *float64       `json:"avg_confidence"`
	ByType             map[string]int `json:"by_type"`
}
