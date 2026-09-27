package meter

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/JuanJoseFlorez/bia-energy-mvp/backend/internal/platform/apperr"
)

const (
	defaultLimit = 50
	maxLimit     = 200
	maxSearchLen = 50
	dateLayout   = "2006-01-02"
)

// Repository is the data access the meter service needs.
type Repository interface {
	List(ctx context.Context, p ListParams) ([]Meter, int, error)
	// Get returns nil, nil when the meter does not exist.
	Get(ctx context.Context, meterID string) (*MeterDetail, error)
	Exists(ctx context.Context, meterID string) (bool, error)
	Readings(ctx context.Context, meterID string, p ReadingsParams) ([]Reading, error)
}

// Service validates meter requests and maps repository results to domain errors.
type Service struct {
	repo Repository
}

// NewService returns a meter service backed by repo.
func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

var (
	healthFilters = map[string]string{"all": "", "ok": "OK", "alert": "ALERT", "critical": "CRITICAL"}
	sortKeys      = map[string]bool{SortMeterID: true, SortConsumption: true, SortVariation: true, SortSeverity: true}
	likeEscaper   = strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)
)

// List returns one page of meters.
func (s *Service) List(ctx context.Context, q ListQuery) (ListResult, error) {
	p, err := parseListQuery(q)
	if err != nil {
		return ListResult{}, err
	}
	items, total, err := s.repo.List(ctx, p)
	if err != nil {
		return ListResult{}, fmt.Errorf("list meters: %w", err)
	}
	if items == nil {
		items = []Meter{}
	}
	return ListResult{Items: items, Total: total}, nil
}

// Get returns one meter with its full anomaly and events.
func (s *Service) Get(ctx context.Context, meterID string) (*MeterDetail, error) {
	d, err := s.repo.Get(ctx, meterID)
	if err != nil {
		return nil, fmt.Errorf("get meter: %w", err)
	}
	if d == nil {
		return nil, notFound(meterID)
	}
	if d.Events == nil {
		d.Events = []Event{}
	}
	return d, nil
}

// Readings returns the meter's readings within the optional bounds.
func (s *Service) Readings(ctx context.Context, meterID string, q ReadingsQuery) (ReadingsResult, error) {
	p, err := parseReadingsQuery(q)
	if err != nil {
		return ReadingsResult{}, err
	}
	ok, err := s.repo.Exists(ctx, meterID)
	if err != nil {
		return ReadingsResult{}, fmt.Errorf("check meter: %w", err)
	}
	if !ok {
		return ReadingsResult{}, notFound(meterID)
	}
	items, err := s.repo.Readings(ctx, meterID, p)
	if err != nil {
		return ReadingsResult{}, fmt.Errorf("list readings: %w", err)
	}
	if items == nil {
		items = []Reading{}
	}
	return ReadingsResult{MeterID: meterID, Items: items, Total: len(items)}, nil
}

func parseListQuery(q ListQuery) (ListParams, error) {
	p := ListParams{Sort: SortMeterID, Limit: defaultLimit}

	status := strings.ToLower(q.Status)
	if status == "" {
		status = "all"
	}
	health, ok := healthFilters[status]
	if !ok {
		return ListParams{}, invalid("status", "must be one of all, ok, alert, critical")
	}
	p.Health = health

	if len(q.Q) > maxSearchLen {
		return ListParams{}, invalid("q", fmt.Sprintf("must be at most %d characters", maxSearchLen))
	}
	p.Search = likeEscaper.Replace(q.Q)

	sort := strings.ToLower(q.Sort)
	if sort != "" {
		if !sortKeys[sort] {
			return ListParams{}, invalid("sort", "must be one of meter_id, consumption, variation, severity")
		}
		p.Sort = sort
	}

	p.Desc = p.Sort != SortMeterID
	switch strings.ToLower(q.Order) {
	case "":
	case "asc":
		p.Desc = false
	case "desc":
		p.Desc = true
	default:
		return ListParams{}, invalid("order", "must be asc or desc")
	}

	if q.Limit != "" {
		n, err := strconv.Atoi(q.Limit)
		if err != nil || n < 1 || n > maxLimit {
			return ListParams{}, invalid("limit", fmt.Sprintf("must be an integer between 1 and %d", maxLimit))
		}
		p.Limit = n
	}
	if q.Offset != "" {
		n, err := strconv.Atoi(q.Offset)
		if err != nil || n < 0 {
			return ListParams{}, invalid("offset", "must be a non-negative integer")
		}
		p.Offset = n
	}
	return p, nil
}

func parseReadingsQuery(q ReadingsQuery) (ReadingsParams, error) {
	from, err := parseBound("from", q.From, false)
	if err != nil {
		return ReadingsParams{}, err
	}
	to, err := parseBound("to", q.To, true)
	if err != nil {
		return ReadingsParams{}, err
	}
	if from != nil && to != nil && from.After(*to) {
		return ReadingsParams{}, invalid("from", "must not be after to")
	}
	return ReadingsParams{From: from, To: to}, nil
}

// parseBound accepts RFC3339 or YYYY-MM-DD; a date-only upper bound covers the whole day.
func parseBound(param, raw string, endOfDay bool) (*time.Time, error) {
	if raw == "" {
		return nil, nil
	}
	if t, err := time.Parse(time.RFC3339, raw); err == nil {
		u := t.UTC()
		return &u, nil
	}
	d, err := time.Parse(dateLayout, raw)
	if err != nil {
		return nil, invalid(param, "must be RFC3339 or YYYY-MM-DD")
	}
	if endOfDay {
		d = d.Add(24*time.Hour - time.Microsecond)
	}
	return &d, nil
}

func invalid(param, rule string) error {
	return fmt.Errorf("invalid %s: %s: %w", param, rule, apperr.ErrValidation)
}

func notFound(meterID string) error {
	return fmt.Errorf("meter %s: %w", meterID, apperr.ErrNotFound)
}
