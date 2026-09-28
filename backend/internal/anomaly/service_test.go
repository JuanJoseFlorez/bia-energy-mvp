package anomaly

import (
	"context"
	"encoding/json"
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/JuanJoseFlorez/bia-energy-mvp/backend/internal/platform/apperr"
)

// fakeRepo is an in-memory Repository used by service and handler tests.
type fakeRepo struct {
	latest      *int64
	runExists   bool
	items       []Item
	gotRunID    int64
	gotFilters  Filters
	detail      *Detail // returned by Get; UpdateStatus changes its Status
	events      []Event
	first, last *time.Time
	updateOK    bool
	gotUpdate   [2]string
	err         error
}

func (f *fakeRepo) LatestCompletedRun(context.Context) (*int64, error) { return f.latest, f.err }
func (f *fakeRepo) RunExists(context.Context, int64) (bool, error)     { return f.runExists, f.err }

func (f *fakeRepo) List(_ context.Context, runID int64, fl Filters) ([]Item, error) {
	f.gotRunID, f.gotFilters = runID, fl
	return f.items, f.err
}

func (f *fakeRepo) Get(context.Context, int64) (*Detail, error) {
	if f.detail == nil {
		return nil, f.err
	}
	d := *f.detail // callers mutate the result
	return &d, f.err
}

func (f *fakeRepo) RelatedEvents(context.Context, int64) ([]Event, error) { return f.events, f.err }

func (f *fakeRepo) ReadingsBounds(context.Context, string) (*time.Time, *time.Time, error) {
	return f.first, f.last, f.err
}

func (f *fakeRepo) UpdateStatus(_ context.Context, _ int64, from, to string) (bool, error) {
	f.gotUpdate = [2]string{from, to}
	if f.updateOK {
		f.detail.Status = to
	}
	return f.updateOK, f.err
}

var (
	periodStart = time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	periodEnd   = time.Date(2026, 9, 14, 23, 0, 0, 0, time.UTC)
)

func int64Ptr(n int64) *int64 { return &n }

func detailWith(status, evidence string) *Detail {
	return &Detail{Item: Item{ID: 31, MeterID: "M-109", Status: status}, Evidence: json.RawMessage(evidence)}
}

func TestTransitions(t *testing.T) {
	allowed := map[[2]string]bool{
		{StatusPending, StatusInvestigating}: true, {StatusPending, StatusValidated}: true, {StatusPending, StatusDismissed}: true,
		{StatusInvestigating, StatusValidated}: true, {StatusInvestigating, StatusResolved}: true, {StatusInvestigating, StatusDismissed}: true,
		{StatusValidated, StatusResolved}: true,
	}
	for _, from := range statuses {
		for _, to := range statuses {
			t.Run(from+"→"+to, func(t *testing.T) {
				repo := &fakeRepo{detail: detailWith(from, `{}`), updateOK: true}

				d, err := NewService(repo).UpdateStatus(context.Background(), "31", StatusUpdate{Status: to})

				if allowed[[2]string{from, to}] {
					if err != nil || d.Status != to || repo.gotUpdate != [2]string{from, to} {
						t.Errorf("UpdateStatus() = %+v, %v; update %v", d, err, repo.gotUpdate)
					}
					return
				}
				if !errors.Is(err, apperr.ErrConflict) {
					t.Errorf("error = %v, want ErrConflict", err)
				}
				if repo.gotUpdate != [2]string{} {
					t.Errorf("repository updated %v on a rejected transition", repo.gotUpdate)
				}
			})
		}
	}
}

func TestUpdateStatusErrors(t *testing.T) {
	tests := []struct {
		name   string
		rawID  string
		status string
		repo   *fakeRepo
		want   error
	}{
		{"unknown status", "31", "DONE", &fakeRepo{detail: detailWith(StatusPending, `{}`)}, apperr.ErrValidation},
		{"empty status", "31", "", &fakeRepo{detail: detailWith(StatusPending, `{}`)}, apperr.ErrValidation},
		{"invalid id", "x", StatusInvestigating, &fakeRepo{}, apperr.ErrValidation},
		{"missing anomaly", "31", StatusInvestigating, &fakeRepo{}, apperr.ErrNotFound},
		{"concurrent change", "31", StatusInvestigating, &fakeRepo{detail: detailWith(StatusPending, `{}`), updateOK: false}, apperr.ErrConflict},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewService(tt.repo).UpdateStatus(context.Background(), tt.rawID, StatusUpdate{Status: tt.status})

			if !errors.Is(err, tt.want) {
				t.Errorf("error = %v, want %v", err, tt.want)
			}
		})
	}
}

func TestUpdateStatusIsCaseInsensitive(t *testing.T) {
	repo := &fakeRepo{detail: detailWith(StatusPending, `{}`), updateOK: true}

	d, err := NewService(repo).UpdateStatus(context.Background(), "31", StatusUpdate{Status: "investigating"})

	if err != nil || d.Status != StatusInvestigating {
		t.Errorf("UpdateStatus() = %+v, %v", d, err)
	}
}

func TestGetNextStatuses(t *testing.T) {
	want := map[string][]string{
		StatusPending:       {StatusInvestigating, StatusValidated, StatusDismissed},
		StatusInvestigating: {StatusValidated, StatusResolved, StatusDismissed},
		StatusValidated:     {StatusResolved},
		StatusResolved:      {},
		StatusDismissed:     {},
	}
	for status, next := range want {
		repo := &fakeRepo{detail: detailWith(status, `{}`)}

		d, err := NewService(repo).Get(context.Background(), "31")

		if err != nil || d.NextStatuses == nil || !slices.Equal(d.NextStatuses, next) {
			t.Errorf("%s: next_statuses = %v (%v), want %v", status, d.NextStatuses, err, next)
		}
	}
}

func TestGetReadingsWindow(t *testing.T) {
	tests := []struct {
		name     string
		evidence string
		from, to time.Time
	}{
		{"change start in the middle", `{"change_start":"2026-09-08T00:00:00Z","transient":{"start":"2026-09-02T00:00:00Z"}}`,
			time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC), periodEnd},
		{"change start near the end", `{"change_start":"2026-09-12T14:00:00Z"}`,
			time.Date(2026, 9, 5, 14, 0, 0, 0, time.UTC), periodEnd},
		{"transient start when no change", `{"change_start":null,"transient":{"start":"2026-09-10T06:00:00Z"}}`,
			time.Date(2026, 9, 3, 6, 0, 0, 0, time.UTC), periodEnd},
		{"earliest outlier", `{"outlier_timestamps":["2026-09-13T05:00:00Z","2026-09-12T02:00:00Z"]}`,
			time.Date(2026, 9, 5, 2, 0, 0, 0, time.UTC), periodEnd},
		{"no anchor: last reading", `{"change_start":null,"transient":null,"outlier_timestamps":[]}`,
			time.Date(2026, 9, 7, 23, 0, 0, 0, time.UTC), periodEnd},
		{"null evidence", ``, time.Date(2026, 9, 7, 23, 0, 0, 0, time.UTC), periodEnd},
		{"anchor before the data", `{"change_start":"2026-08-01T00:00:00Z"}`,
			periodStart, time.Date(2026, 9, 8, 0, 0, 0, 0, time.UTC)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &fakeRepo{detail: detailWith(StatusPending, tt.evidence), first: &periodStart, last: &periodEnd}

			d, err := NewService(repo).Get(context.Background(), "31")

			if err != nil || d.ReadingsWindow == nil {
				t.Fatalf("Get() = %+v, %v", d, err)
			}
			if w := d.ReadingsWindow; !w.From.Equal(tt.from) || !w.To.Equal(tt.to) {
				t.Errorf("window = %s → %s, want %s → %s", w.From, w.To, tt.from, tt.to)
			}
		})
	}
}

func TestGetWithoutReadings(t *testing.T) {
	d, err := NewService(&fakeRepo{detail: detailWith(StatusPending, `{}`)}).Get(context.Background(), "31")

	if err != nil || d.ReadingsWindow != nil || d.RelatedEvents == nil {
		t.Errorf("Get() = %+v, %v; want nil window and empty related_events", d, err)
	}
}

func TestGetErrors(t *testing.T) {
	if _, err := NewService(&fakeRepo{}).Get(context.Background(), "31"); !errors.Is(err, apperr.ErrNotFound) {
		t.Errorf("missing: error = %v, want ErrNotFound", err)
	}
	if _, err := NewService(&fakeRepo{}).Get(context.Background(), "0"); !errors.Is(err, apperr.ErrValidation) {
		t.Errorf("id 0: error = %v, want ErrValidation", err)
	}
}

func TestListRunSelection(t *testing.T) {
	tests := []struct {
		name    string
		query   ListQuery
		repo    *fakeRepo
		wantRun *int64
		wantErr error
	}{
		{"default latest completed", ListQuery{}, &fakeRepo{latest: int64Ptr(7)}, int64Ptr(7), nil},
		{"no completed run", ListQuery{}, &fakeRepo{}, nil, nil},
		{"explicit run", ListQuery{AnalysisID: "5"}, &fakeRepo{latest: int64Ptr(7), runExists: true}, int64Ptr(5), nil},
		{"unknown run", ListQuery{AnalysisID: "5"}, &fakeRepo{}, nil, apperr.ErrNotFound},
		{"invalid run id", ListQuery{AnalysisID: "-1"}, &fakeRepo{}, nil, apperr.ErrValidation},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res, err := NewService(tt.repo).List(context.Background(), tt.query)

			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Errorf("error = %v, want %v", err, tt.wantErr)
				}
				return
			}
			if err != nil || res.Items == nil {
				t.Fatalf("List() = %+v, %v", res, err)
			}
			if (res.AnalysisID == nil) != (tt.wantRun == nil) || (res.AnalysisID != nil && *res.AnalysisID != *tt.wantRun) {
				t.Errorf("analysis_id = %v, want %v", res.AnalysisID, tt.wantRun)
			}
			if tt.wantRun != nil && tt.repo.gotRunID != *tt.wantRun {
				t.Errorf("listed run %d, want %d", tt.repo.gotRunID, *tt.wantRun)
			}
		})
	}
}

func TestListFilters(t *testing.T) {
	repo := &fakeRepo{latest: int64Ptr(7)}

	if _, err := NewService(repo).List(context.Background(), ListQuery{Type: "data_quality", Severity: "high", Status: "pending"}); err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if want := (Filters{Type: "DATA_QUALITY", Severity: "HIGH", Status: StatusPending}); repo.gotFilters != want {
		t.Errorf("filters = %+v, want %+v", repo.gotFilters, want)
	}

	for _, q := range []ListQuery{{Type: "BROKEN"}, {Severity: "CRITICAL"}, {Status: "OPEN"}} {
		if _, err := NewService(repo).List(context.Background(), q); !errors.Is(err, apperr.ErrValidation) {
			t.Errorf("List(%+v) error = %v, want ErrValidation", q, err)
		}
	}
}
