package shipment

import (
	"context"
	"errors"
	"testing"
)

type routerStub struct {
	choices map[string]Choice
}

func (s routerStub) Choose(_ context.Context, line Line) (Choice, bool, error) {
	choice, found := s.choices[line.LineID]
	return choice, found, nil
}

type repositoryStub struct {
	err error
}

func (s repositoryStub) Save(context.Context, string, Plan) error {
	return s.err
}

func TestBuildGroupsPackagesAndPreservesRejectedQuantities(t *testing.T) {
	planner := NewPlanner(routerStub{choices: map[string]Choice{
		"line-1": {WarehouseID: "warehouse-a", Quantity: 2},
		"line-2": {WarehouseID: "warehouse-a", Quantity: 1},
	}}, repositoryStub{})

	plan, err := planner.Build(context.Background(), "order-1", []Line{
		{LineID: "line-1", ItemID: "item-1", Quantity: 3},
		{LineID: "line-2", ItemID: "item-2", Quantity: 1},
		{LineID: "line-3", ItemID: "item-3", Quantity: 4},
	})

	if err != nil || len(plan.Packages) != 1 || len(plan.Packages[0].Lines) != 2 || len(plan.Rejected) != 2 {
		t.Fatalf("unexpected shipment plan: plan=%+v err=%v", plan, err)
	}
	if plan.Rejected[0].Line.Quantity != 1 || plan.Rejected[1].Line.Quantity != 4 {
		t.Fatalf("rejected quantities were not preserved: %+v", plan.Rejected)
	}
}

func TestBuildReturnsRejectedLinesWhenNothingCanShip(t *testing.T) {
	planner := NewPlanner(routerStub{}, repositoryStub{})

	plan, err := planner.Build(context.Background(), "order-1", []Line{{LineID: "line-1", ItemID: "item-1", Quantity: 2}})

	if !errors.Is(err, ErrNothingShippable) || len(plan.Rejected) != 1 {
		t.Fatalf("expected rejected line result, plan=%+v err=%v", plan, err)
	}
}

func TestBuildReturnsPlanWhenSaveFails(t *testing.T) {
	planner := NewPlanner(
		routerStub{choices: map[string]Choice{"line-1": {WarehouseID: "warehouse-a", Quantity: 2}}},
		repositoryStub{err: errors.New("store unavailable")},
	)

	plan, err := planner.Build(context.Background(), "order-1", []Line{{LineID: "line-1", ItemID: "item-1", Quantity: 2}})

	if err == nil || len(plan.Packages) != 1 {
		t.Fatalf("expected preserved plan on save failure, plan=%+v err=%v", plan, err)
	}
}
