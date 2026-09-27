package dashboard

import (
	"context"
	"fmt"
)

// Repository is the data access the dashboard service needs.
type Repository interface {
	Summary(ctx context.Context) (Summary, error)
}

// Service builds the dashboard summary.
type Service struct {
	repo Repository
}

// NewService returns a dashboard service backed by repo.
func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

// Summary returns the platform-wide KPIs.
func (s *Service) Summary(ctx context.Context) (Summary, error) {
	sum, err := s.repo.Summary(ctx)
	if err != nil {
		return Summary{}, fmt.Errorf("dashboard summary: %w", err)
	}
	return sum, nil
}
