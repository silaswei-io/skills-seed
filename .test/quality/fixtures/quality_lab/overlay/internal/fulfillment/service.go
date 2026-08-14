package fulfillment

import (
	"context"
	"errors"
	"fmt"
)

var ErrNothingAllocated = errors.New("no order line could be allocated")

type Line struct {
	ItemID   string
	Quantity int64
}

type Allocation struct {
	ItemID    string
	Requested int64
	Allocated int64
}

type Stock interface {
	Allocate(context.Context, string, int64) (int64, error)
}

type Repository interface {
	SavePlan(context.Context, string, []Allocation) error
}

type Service struct {
	stock Stock
	plans Repository
}

func NewService(stock Stock, plans Repository) *Service {
	return &Service{stock: stock, plans: plans}
}

// Plan 保留逐行分配结果；部分缺货不会撤销已经成功的分配。
func (s *Service) Plan(ctx context.Context, orderID string, lines []Line) ([]Allocation, error) {
	if orderID == "" || len(lines) == 0 {
		return nil, errors.New("order and lines are required")
	}
	allocations := make([]Allocation, 0, len(lines))
	allocatedTotal := int64(0)
	for _, line := range lines {
		if line.ItemID == "" || line.Quantity <= 0 {
			return nil, errors.New("line item and positive quantity are required")
		}
		quantity, err := s.stock.Allocate(ctx, line.ItemID, line.Quantity)
		if err != nil {
			return allocations, fmt.Errorf("allocate %s: %w", line.ItemID, err)
		}
		allocations = append(allocations, Allocation{ItemID: line.ItemID, Requested: line.Quantity, Allocated: quantity})
		allocatedTotal += quantity
	}
	if allocatedTotal == 0 {
		return allocations, ErrNothingAllocated
	}
	if err := s.plans.SavePlan(ctx, orderID, allocations); err != nil {
		return allocations, fmt.Errorf("save partial fulfillment plan: %w", err)
	}
	return allocations, nil
}
