package shipment

import (
	"context"
	"errors"
	"fmt"
)

var ErrNothingShippable = errors.New("no order line can be shipped")

type Line struct {
	LineID   string
	ItemID   string
	Quantity int64
}

type Choice struct {
	WarehouseID string
	Quantity    int64
}

type Package struct {
	WarehouseID string
	Lines       []Line
}

type RejectedLine struct {
	Line   Line
	Reason string
}

type Plan struct {
	Packages []Package
	Rejected []RejectedLine
}

type Router interface {
	Choose(context.Context, Line) (Choice, bool, error)
}

type Repository interface {
	Save(context.Context, string, Plan) error
}

type Planner struct {
	router Router
	repo   Repository
}

func NewPlanner(router Router, repo Repository) *Planner {
	return &Planner{router: router, repo: repo}
}

// Build 按仓库合并可发明细，同时显式保留缺货或只能部分发货的明细结果。
func (p *Planner) Build(ctx context.Context, orderID string, lines []Line) (Plan, error) {
	if orderID == "" || len(lines) == 0 {
		return Plan{}, errors.New("order and lines are required")
	}
	plan := Plan{}
	packageIndex := make(map[string]int)
	for _, line := range lines {
		if line.LineID == "" || line.ItemID == "" || line.Quantity <= 0 {
			return plan, errors.New("line, item, and positive quantity are required")
		}
		choice, found, err := p.router.Choose(ctx, line)
		if err != nil {
			return plan, fmt.Errorf("route line %s: %w", line.LineID, err)
		}
		if !found || choice.Quantity <= 0 {
			plan.Rejected = append(plan.Rejected, RejectedLine{Line: line, Reason: "unavailable"})
			continue
		}
		shippable := line
		if choice.Quantity < shippable.Quantity {
			shippable.Quantity = choice.Quantity
			remaining := line
			remaining.Quantity -= choice.Quantity
			plan.Rejected = append(plan.Rejected, RejectedLine{Line: remaining, Reason: "insufficient stock"})
		}
		index, exists := packageIndex[choice.WarehouseID]
		if !exists {
			index = len(plan.Packages)
			packageIndex[choice.WarehouseID] = index
			plan.Packages = append(plan.Packages, Package{WarehouseID: choice.WarehouseID})
		}
		plan.Packages[index].Lines = append(plan.Packages[index].Lines, shippable)
	}
	if len(plan.Packages) == 0 {
		return plan, ErrNothingShippable
	}
	if err := p.repo.Save(ctx, orderID, plan); err != nil {
		return plan, fmt.Errorf("save shipment plan: %w", err)
	}
	return plan, nil
}
