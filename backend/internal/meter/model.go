// Package meter serves the meters list, meter detail and meter readings.
package meter

import "time"

// Sort keys accepted by the meters list.
const (
	SortMeterID     = "meter_id"
	SortConsumption = "consumption"
	SortVariation   = "variation"
	SortSeverity    = "severity"
)

// Meter is one row of the meters list: brief §16 meter fields plus analysis-derived data.
type Meter struct {
	ID                   int64           `json:"id"`
	MeterID              string          `json:"meter_id"`
	Name                 *string         `json:"name"`
	Location             *string         `json:"location"`
	Status               *string         `json:"status"`
	CreatedAt            *time.Time      `json:"created_at"`
	PeriodConsumptionKWh float64         `json:"period_consumption_kwh"`
	Health               *string         `json:"health"`
	Metrics              *Metrics        `json:"metrics"`
	Anomaly              *AnomalySummary `json:"anomaly"`
}

// Metrics are the daily figures engine-ai stored for the meter in the latest completed run.
type Metrics struct {
	CurrentKWh   float64    `json:"current_kwh"`
	BaselineKWh  float64    `json:"baseline_kwh"`
	VariationPct float64    `json:"variation_pct"`
	ChangeStart  *time.Time `json:"change_start"`
}

// AnomalySummary is the meter's top anomaly in the latest completed run.
type AnomalySummary struct {
	ID         int64    `json:"id"`
	MeterID    string   `json:"meter_id"`
	Type       *string  `json:"type"`
	Severity   *string  `json:"severity"`
	Confidence *float64 `json:"confidence"`
	Priority   *int     `json:"priority"`
}

// AnomalyDetail is the full anomaly record shown on the meter detail.
type AnomalyDetail struct {
	AnomalySummary
	Reason            *string    `json:"reason"`
	RecommendedAction *string    `json:"recommended_action"`
	Status            *string    `json:"status"`
	DetectedAt        *time.Time `json:"detected_at"`
}

// Event is a known operational event of a meter.
type Event struct {
	ID          int64     `json:"id"`
	MeterID     string    `json:"meter_id"`
	Timestamp   time.Time `json:"timestamp"`
	Type        *string   `json:"type"`
	Description *string   `json:"description"`
}

// Reading is one hourly measurement of a meter.
type Reading struct {
	ID             int64     `json:"id"`
	MeterID        string    `json:"meter_id"`
	Timestamp      time.Time `json:"timestamp"`
	ConsumptionKWh *float64  `json:"consumption_kwh"`
	VoltageV       *float64  `json:"voltage_v"`
	CurrentA       *float64  `json:"current_a"`
	PowerFactor    *float64  `json:"power_factor"`
	Status         *string   `json:"status"`
}

// MeterDetail is GET /meters/{meterId}: the list fields, the full anomaly and the meter's events.
type MeterDetail struct {
	Meter
	// Anomaly shadows Meter.Anomaly in JSON (the shallower field wins) to expose the full record.
	Anomaly *AnomalyDetail `json:"anomaly"`
	Events  []Event        `json:"events"`
}

// ListQuery is the raw, unvalidated query string of GET /meters.
type ListQuery struct {
	Status, Q, Sort, Order, Limit, Offset string
}

// ListParams is the validated, SQL-ready form of ListQuery.
type ListParams struct {
	Health string // "" = no filter, else OK | ALERT | CRITICAL
	Search string // LIKE-escaped substring of meter_id, "" = no filter
	Sort   string // one of the Sort* keys
	Desc   bool
	Limit  int
	Offset int
}

// ListResult is the GET /meters response envelope.
type ListResult struct {
	Items []Meter `json:"items"`
	Total int     `json:"total"`
}

// ReadingsQuery is the raw query string of GET /meters/{meterId}/readings.
type ReadingsQuery struct {
	From, To string
}

// ReadingsParams are validated, inclusive UTC bounds; nil means unbounded.
type ReadingsParams struct {
	From, To *time.Time
}

// ReadingsResult is the GET /meters/{meterId}/readings response envelope.
type ReadingsResult struct {
	MeterID string    `json:"meter_id"`
	Items   []Reading `json:"items"`
	Total   int       `json:"total"`
}
