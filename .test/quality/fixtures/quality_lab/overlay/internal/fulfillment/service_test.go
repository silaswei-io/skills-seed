package fulfillment

import (
	"context"
	"errors"
	"testing"
)

type stockStub struct {
	allocations map[string]int64
}

func (s stockStub) Allocate(_ context.Context, itemID string, _ int64) (int64, error) {
	return s.allocations[itemID], nil
}

type repositoryStub struct {
	err error
}

func (s repositoryStub) SavePlan(context.Context, string, []Allocation) error {
	return s.err
}

func TestPlanPreservesPartialAllocationOnSaveFailure(t *testing.T) {
	service := NewService(
		stockStub{allocations: map[string]int64{"item-1": 2, "item-2": 0}},
		repositoryStub{err: errors.New("store unavailable")},
	)

	allocations, err := service.Plan(context.Background(), "order-1", []Line{
		{ItemID: "item-1", Quantity: 3},
		{ItemID: "item-2", Quantity: 1},
	})

	if err == nil || len(allocations) != 2 || allocations[0].Allocated != 2 {
		t.Fatalf("expected preserved partial result, allocations=%+v err=%v", allocations, err)
	}
}
