package anomaly

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/JuanJoseFlorez/bia-energy-mvp/backend/internal/platform/apperr"
)

// windowRadius is how far around the anomaly's anchor the readings window reaches.
const windowRadius = 7 * 24 * time.Hour

// Repository is the data access the anomaly service needs.
type Repository interface {
	// LatestCompletedRun returns nil when no run has completed yet.
	LatestCompletedRun(ctx context.Context) (*int64, error)
	RunExists(ctx context.Context, id int64) (bool, error)
	List(ctx context.Context, runID int64, f Filters) ([]Item, error)
	// Get returns nil, nil when the anomaly does not exist.
	Get(ctx context.Context, id int64) (*Detail, error)
	RelatedEvents(ctx context.Context, anomalyID int64) ([]Event, error)
	// ReadingsBounds returns nil bounds when the meter has no readings.
	ReadingsBounds(ctx context.Context, meterID string) (first, last *time.Time, err error)
	UpdateStatus(ctx context.Context, id int64, from, to string) (bool, error)
}

// Service validates anomaly requests and enforces the action workflow.
type Service struct {
	repo Repository
}

// NewService returns an anomaly service backed by repo.
func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

// transitions is the action workflow: allowed next statuses per status, in display order.
// RESOLVED and DISMISSED are terminal.
var transitions = map[string][]string{
	StatusPending:       {StatusInvestigating, StatusValidated, StatusDismissed},
	StatusInvestigating: {StatusValidated, StatusResolved, StatusDismissed},
	StatusValidated:     {StatusResolved},
	StatusResolved:      {},
	StatusDismissed:     {},
}

var (
	types      = []string{"REAL_ANOMALY", "EXPLAINABLE_ANOMALY", "FALSE_POSITIVE", "DATA_QUALITY"}
	severities = []string{"LOW", "MEDIUM", "HIGH"}
	statuses   = []string{StatusPending, StatusInvestigating, StatusValidated, StatusDismissed, StatusResolved}
)

// List returns the anomalies of one run (default: the latest completed run) in priority order.
func (s *Service) List(ctx context.Context, q ListQuery) (ListResult, error) {
	f, err := parseFilters(q)
	if err != nil {
		return ListResult{}, err
	}
	runID, err := s.resolveRun(ctx, q.AnalysisID)
	if err != nil {
		return ListResult{}, err
	}
	if runID == nil {
		return ListResult{Items: []Item{}}, nil
	}
	items, err := s.repo.List(ctx, *runID, f)
	if err != nil {
		return ListResult{}, fmt.Errorf("list anomalies: %w", err)
	}
	if items == nil {
		items = []Item{}
	}
	return ListResult{AnalysisID: runID, Items: items, Total: len(items)}, nil
}

// Get returns one anomaly with its evidence, related events, readings window and next statuses.
func (s *Service) Get(ctx context.Context, rawID string) (*Detail, error) {
	id, err := parseID("id", rawID)
	if err != nil {
		return nil, err
	}
	return s.detail(ctx, id)
}

// UpdateStatus moves the anomaly along the action workflow and returns the updated detail.
func (s *Service) UpdateStatus(ctx context.Context, rawID string, u StatusUpdate) (*Detail, error) {
	id, err := parseID("id", rawID)
	if err != nil {
		return nil, err
	}
	to := strings.ToUpper(u.Status)
	if !slices.Contains(statuses, to) {
		return nil, invalid("status", "must be one of "+strings.Join(statuses, ", "))
	}
	cur, err := s.repo.Get(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("get anomaly: %w", err)
	}
	if cur == nil {
		return nil, notFound(id)
	}
	if !slices.Contains(transitions[cur.Status], to) {
		return nil, fmt.Errorf("cannot change status from %s to %s: %w", cur.Status, to, apperr.ErrConflict)
	}
	ok, err := s.repo.UpdateStatus(ctx, id, cur.Status, to)
	if err != nil {
		return nil, fmt.Errorf("update anomaly status: %w", err)
	}
	if !ok {
		// Another request changed the status between the read and the update.
		return nil, fmt.Errorf("anomaly %d status changed concurrently, reload and retry: %w", id, apperr.ErrConflict)
	}
	return s.detail(ctx, id)
}

func (s *Service) detail(ctx context.Context, id int64) (*Detail, error) {
	d, err := s.repo.Get(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("get anomaly: %w", err)
	}
	if d == nil {
		return nil, notFound(id)
	}
	if d.RelatedEvents, err = s.repo.RelatedEvents(ctx, id); err != nil {
		return nil, fmt.Errorf("related events: %w", err)
	}
	if d.RelatedEvents == nil {
		d.RelatedEvents = []Event{}
	}
	first, last, err := s.repo.ReadingsBounds(ctx, d.MeterID)
	if err != nil {
		return nil, fmt.Errorf("readings bounds: %w", err)
	}
	if first != nil && last != nil {
		d.ReadingsWindow = readingsWindow(anchor(d.Evidence, *last), *first, *last)
	}
	d.NextStatuses = slices.Clone(transitions[d.Status])
	if d.NextStatuses == nil {
		d.NextStatuses = []string{}
	}
	return d, nil
}

func (s *Service) resolveRun(ctx context.Context, rawID string) (*int64, error) {
	if rawID == "" {
		id, err := s.repo.LatestCompletedRun(ctx)
		if err != nil {
			return nil, fmt.Errorf("latest completed run: %w", err)
		}
		return id, nil
	}
	id, err := parseID("analysis_id", rawID)
	if err != nil {
		return nil, err
	}
	ok, err := s.repo.RunExists(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("check run: %w", err)
	}
	if !ok {
		return nil, fmt.Errorf("analysis %d: %w", id, apperr.ErrNotFound)
	}
	return &id, nil
}

// anchorEvidence holds the only evidence fields the backend reads.
type anchorEvidence struct {
	ChangeStart *time.Time `json:"change_start"`
	Transient   *struct {
		Start *time.Time `json:"start"`
	} `json:"transient"`
	OutlierTimestamps []time.Time `json:"outlier_timestamps"`
}

// anchor picks the moment the investigation chart centres on: the change start, else the
// transient start, else the earliest outlier, else the meter's last reading.
func anchor(evidence json.RawMessage, lastReading time.Time) time.Time {
	var ev anchorEvidence
	if len(evidence) == 0 || json.Unmarshal(evidence, &ev) != nil {
		return lastReading // unreadable evidence only loses the centring, not the window
	}
	switch {
	case ev.ChangeStart != nil:
		return *ev.ChangeStart
	case ev.Transient != nil && ev.Transient.Start != nil:
		return *ev.Transient.Start
	case len(ev.OutlierTimestamps) > 0:
		return slices.MinFunc(ev.OutlierTimestamps, func(a, b time.Time) int { return a.Compare(b) })
	default:
		return lastReading
	}
}

// readingsWindow spans windowRadius on each side of anchor, clamped to [first, last].
func readingsWindow(anchor, first, last time.Time) *ReadingsWindow {
	anchor = clamp(anchor, first, last)
	return &ReadingsWindow{
		From: clamp(anchor.Add(-windowRadius), first, last),
		To:   clamp(anchor.Add(windowRadius), first, last),
	}
}

func clamp(t, lo, hi time.Time) time.Time {
	if t.Before(lo) {
		return lo
	}
	if t.After(hi) {
		return hi
	}
	return t
}

func parseFilters(q ListQuery) (Filters, error) {
	f := Filters{Type: strings.ToUpper(q.Type), Severity: strings.ToUpper(q.Severity), Status: strings.ToUpper(q.Status)}
	for _, c := range []struct {
		param, value string
		allowed      []string
	}{
		{"type", f.Type, types},
		{"severity", f.Severity, severities},
		{"status", f.Status, statuses},
	} {
		if c.value != "" && !slices.Contains(c.allowed, c.value) {
			return Filters{}, invalid(c.param, "must be one of "+strings.Join(c.allowed, ", "))
		}
	}
	return f, nil
}

func parseID(param, raw string) (int64, error) {
	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || id < 1 {
		return 0, invalid(param, "must be a positive integer")
	}
	return id, nil
}

func invalid(param, rule string) error {
	return fmt.Errorf("invalid %s: %s: %w", param, rule, apperr.ErrValidation)
}

func notFound(id int64) error {
	return fmt.Errorf("anomaly %d: %w", id, apperr.ErrNotFound)
}
