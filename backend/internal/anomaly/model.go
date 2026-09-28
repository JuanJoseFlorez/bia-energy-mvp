// Package anomaly serves the anomalies of analysis runs and their action workflow.
package anomaly

import (
	"encoding/json"
	"time"
)

// Action workflow statuses.
const (
	StatusPending       = "PENDING"
	StatusInvestigating = "INVESTIGATING"
	StatusValidated     = "VALIDATED"
	StatusDismissed     = "DISMISSED"
	StatusResolved      = "RESOLVED"
)

// typeFalsePositive is the only anomaly type that is not an anomaly (brief §10 "anomaly": false).
const typeFalsePositive = "FALSE_POSITIVE"

// Item is one row of the anomalies list.
type Item struct {
	ID                int64      `json:"id"`
	AnalysisID        int64      `json:"analysis_id"`
	MeterID           string     `json:"meter_id"`
	DetectedAt        *time.Time `json:"detected_at"`
	IsAnomaly         bool       `json:"anomaly"`
	Type              *string    `json:"type"`
	Severity          *string    `json:"severity"`
	Confidence        *float64   `json:"confidence"`
	Priority          *int       `json:"priority"`
	Reason            *string    `json:"reason"`
	RecommendedAction *string    `json:"recommended_action"`
	ExplanationSource *string    `json:"explanation_source"`
	Status            string     `json:"status"`
}

// Detail is GET /anomalies/{id}: the list fields plus explanation, evidence and investigation aids.
type Detail struct {
	Item
	Explanation    *string         `json:"explanation"`
	Evidence       json.RawMessage `json:"evidence"`
	RelatedEvents  []Event         `json:"related_events"`
	ReadingsWindow *ReadingsWindow `json:"readings_window"`
	NextStatuses   []string        `json:"next_statuses"`
}

// Event is a known operational event referenced by the anomaly's evidence.
type Event struct {
	ID          int64     `json:"id"`
	MeterID     string    `json:"meter_id"`
	Timestamp   time.Time `json:"timestamp"`
	Type        *string   `json:"type"`
	Description *string   `json:"description"`
}

// ReadingsWindow bounds the readings worth charting; fetch them from GET /meters/{meterId}/readings.
type ReadingsWindow struct {
	From time.Time `json:"from"`
	To   time.Time `json:"to"`
}

// ListQuery is the raw, unvalidated query string of GET /anomalies.
type ListQuery struct {
	AnalysisID, Type, Severity, Status string
}

// Filters are validated list filters; "" means no filter.
type Filters struct {
	Type, Severity, Status string
}

// ListResult is the GET /anomalies response envelope; AnalysisID is null before any completed run.
type ListResult struct {
	AnalysisID *int64 `json:"analysis_id"`
	Items      []Item `json:"items"`
	Total      int    `json:"total"`
}

// StatusUpdate is the PATCH /anomalies/{id} body.
type StatusUpdate struct {
	Status string `json:"status"`
}
