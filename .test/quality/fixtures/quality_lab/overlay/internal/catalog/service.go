package catalog

import (
	"context"
	"fmt"

	"example.com/quality-lab/internal/tenant"
)

type Item struct {
	ID    string
	Name  string
	Stock int64
}

type Reader interface {
	Available(context.Context, string) ([]Item, error)
}

type Service struct {
	reader Reader
}

func NewService(reader Reader) *Service {
	return &Service{reader: reader}
}

// ListAvailable returns the tenant-visible catalog after validating request ownership.
func (s *Service) ListAvailable(ctx context.Context) ([]Item, error) {
	tenantID, err := tenant.RequireID(ctx)
	if err != nil {
		return nil, err
	}
	items, err := s.reader.Available(ctx, tenantID)
	if err != nil {
		return nil, fmt.Errorf("list tenant catalog: %w", err)
	}
	return items, nil
}
