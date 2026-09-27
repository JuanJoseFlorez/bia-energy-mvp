package meter

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/JuanJoseFlorez/bia-energy-mvp/backend/internal/platform/apperr"
)

// fakeRepo is an in-memory Repository used by service and handler tests.
type fakeRepo struct {
	items    []Meter
	total    int
	gotList  ListParams
	detail   *MeterDetail
	exists   bool
	readings []Reading
	gotRead  ReadingsParams
	err      error
}

func (f *fakeRepo) List(_ context.Context, p ListParams) ([]Meter, int, error) {
	f.gotList = p
	return f.items, f.total, f.err
}

func (f *fakeRepo) Get(context.Context, string) (*MeterDetail, error) { return f.detail, f.err }

func (f *fakeRepo) Exists(context.Context, string) (bool, error) { return f.exists, f.err }

func (f *fakeRepo) Readings(_ context.Context, _ string, p ReadingsParams) ([]Reading, error) {
	f.gotRead = p
	return f.readings, f.err
}

func TestParseListQueryDefaults(t *testing.T) {
	p, err := parseListQuery(ListQuery{})
	if err != nil {
		t.Fatalf("parseListQuery() error = %v", err)
	}
	want := ListParams{Health: "", Search: "", Sort: SortMeterID, Desc: false, Limit: 50, Offset: 0}
	if p != want {
		t.Errorf("params = %+v, want %+v", p, want)
	}
}

func TestParseListQueryValues(t *testing.T) {
	tests := []struct {
		name string
		q    ListQuery
		want ListParams
	}{
		{"status is case-insensitive", ListQuery{Status: "Critical"}, ListParams{Health: "CRITICAL", Sort: SortMeterID, Limit: 50}},
		{"all means no filter", ListQuery{Status: "all"}, ListParams{Health: "", Sort: SortMeterID, Limit: 50}},
		{"non-id sort defaults to desc", ListQuery{Sort: "variation"}, ListParams{Sort: SortVariation, Desc: true, Limit: 50}},
		{"explicit asc wins", ListQuery{Sort: "severity", Order: "asc"}, ListParams{Sort: SortSeverity, Desc: false, Limit: 50}},
		{"explicit desc on meter_id", ListQuery{Order: "desc"}, ListParams{Sort: SortMeterID, Desc: true, Limit: 50}},
		{"limit and offset", ListQuery{Limit: "5", Offset: "10"}, ListParams{Sort: SortMeterID, Limit: 5, Offset: 10}},
		{"sort is case-insensitive", ListQuery{Sort: "Variation"}, ListParams{Sort: SortVariation, Desc: true, Limit: 50}},
		{"order is case-insensitive", ListQuery{Order: "DESC"}, ListParams{Sort: SortMeterID, Desc: true, Limit: 50}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p, err := parseListQuery(tt.q)
			if err != nil {
				t.Fatalf("parseListQuery() error = %v", err)
			}
			if p != tt.want {
				t.Errorf("params = %+v, want %+v", p, tt.want)
			}
		})
	}
}

func TestParseListQueryEscapesSearch(t *testing.T) {
	p, err := parseListQuery(ListQuery{Q: `a\b%c_d`})
	if err != nil {
		t.Fatalf("parseListQuery() error = %v", err)
	}
	if want := `a\\b\%c\_d`; p.Search != want {
		t.Errorf("Search = %q, want %q", p.Search, want)
	}
}

func TestParseListQueryInvalid(t *testing.T) {
	tests := []struct {
		name  string
		q     ListQuery
		param string
	}{
		{"unknown status", ListQuery{Status: "broken"}, "status"},
		{"search too long", ListQuery{Q: strings.Repeat("x", 51)}, "q"},
		{"unknown sort", ListQuery{Sort: "bogus"}, "sort"},
		{"unknown order", ListQuery{Order: "up"}, "order"},
		{"limit not a number", ListQuery{Limit: "abc"}, "limit"},
		{"limit zero", ListQuery{Limit: "0"}, "limit"},
		{"limit too big", ListQuery{Limit: "201"}, "limit"},
		{"offset negative", ListQuery{Offset: "-1"}, "offset"},
		{"offset not a number", ListQuery{Offset: "x"}, "offset"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := parseListQuery(tt.q)
			if !errors.Is(err, apperr.ErrValidation) {
				t.Fatalf("error = %v, want ErrValidation", err)
			}
			if !strings.Contains(err.Error(), tt.param) {
				t.Errorf("error %q does not name %q", err, tt.param)
			}
		})
	}
}

func TestParseReadingsQuery(t *testing.T) {
	day := time.Date(2026, 9, 14, 0, 0, 0, 0, time.UTC)
	endOfDay := time.Date(2026, 9, 14, 23, 59, 59, 999999000, time.UTC)

	p, err := parseReadingsQuery(ReadingsQuery{From: "2026-09-14", To: "2026-09-14"})
	if err != nil {
		t.Fatalf("parseReadingsQuery() error = %v", err)
	}
	if p.From == nil || !p.From.Equal(day) {
		t.Errorf("From = %v, want %v", p.From, day)
	}
	if p.To == nil || !p.To.Equal(endOfDay) {
		t.Errorf("To = %v, want %v", p.To, endOfDay)
	}

	p, err = parseReadingsQuery(ReadingsQuery{From: "2026-09-14T05:00:00-05:00"})
	if err != nil {
		t.Fatalf("parseReadingsQuery() error = %v", err)
	}
	if want := time.Date(2026, 9, 14, 10, 0, 0, 0, time.UTC); p.From == nil || !p.From.Equal(want) || p.From.Location() != time.UTC {
		t.Errorf("From = %v, want %v in UTC", p.From, want)
	}
	if p.To != nil {
		t.Errorf("To = %v, want nil", p.To)
	}
}

func TestParseReadingsQueryInvalid(t *testing.T) {
	tests := []struct {
		name  string
		q     ReadingsQuery
		param string
	}{
		{"bad from", ReadingsQuery{From: "yesterday"}, "from"},
		{"bad to", ReadingsQuery{To: "2026-13-01"}, "to"},
		{"from after to", ReadingsQuery{From: "2026-09-10", To: "2026-09-09"}, "from"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := parseReadingsQuery(tt.q)
			if !errors.Is(err, apperr.ErrValidation) {
				t.Fatalf("error = %v, want ErrValidation", err)
			}
			if !strings.Contains(err.Error(), tt.param) {
				t.Errorf("error %q does not name %q", err, tt.param)
			}
		})
	}
}

func TestServiceListEmptyIsNotNil(t *testing.T) {
	svc := NewService(&fakeRepo{})

	res, err := svc.List(context.Background(), ListQuery{})
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if res.Items == nil {
		t.Error("Items = nil, want empty slice")
	}
}

func TestServiceListRepoErrorIsInternal(t *testing.T) {
	svc := NewService(&fakeRepo{err: errors.New("db down")})

	_, err := svc.List(context.Background(), ListQuery{})
	if err == nil || errors.Is(err, apperr.ErrValidation) || errors.Is(err, apperr.ErrNotFound) {
		t.Errorf("error = %v, want plain wrapped error", err)
	}
}

func TestServiceGetNotFound(t *testing.T) {
	svc := NewService(&fakeRepo{detail: nil})

	_, err := svc.Get(context.Background(), "M-999")
	if !errors.Is(err, apperr.ErrNotFound) {
		t.Fatalf("error = %v, want ErrNotFound", err)
	}
	if !strings.Contains(err.Error(), "M-999") {
		t.Errorf("error %q does not name the meter", err)
	}
}

func TestServiceGetEventsNeverNil(t *testing.T) {
	svc := NewService(&fakeRepo{detail: &MeterDetail{Meter: Meter{MeterID: "M-101"}}})

	d, err := svc.Get(context.Background(), "M-101")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if d.Events == nil {
		t.Error("Events = nil, want empty slice")
	}
}

func TestServiceReadingsUnknownMeter(t *testing.T) {
	svc := NewService(&fakeRepo{exists: false})

	_, err := svc.Readings(context.Background(), "M-999", ReadingsQuery{})
	if !errors.Is(err, apperr.ErrNotFound) {
		t.Errorf("error = %v, want ErrNotFound", err)
	}
}

func TestServiceReadingsResult(t *testing.T) {
	repo := &fakeRepo{exists: true, readings: []Reading{{ID: 1}, {ID: 2}}}
	svc := NewService(repo)

	res, err := svc.Readings(context.Background(), "M-101", ReadingsQuery{From: "2026-09-14"})
	if err != nil {
		t.Fatalf("Readings() error = %v", err)
	}
	if res.MeterID != "M-101" || res.Total != 2 || len(res.Items) != 2 {
		t.Errorf("result = %+v", res)
	}
	if repo.gotRead.From == nil {
		t.Error("From bound not passed to repository")
	}
}
