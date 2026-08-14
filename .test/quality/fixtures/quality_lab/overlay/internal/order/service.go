package order

import (
	"context"
	"errors"
	"fmt"

	"example.com/quality-lab/internal/tenant"
)

var ErrInconsistentReservation = errors.New("inventory reservation may require reconciliation")

type Inventory interface {
	Reserve(context.Context, string, string, int64) error
	Release(context.Context, string, string, int64) error
}

type Repository interface {
	Create(context.Context, Order) error
}

type Publisher interface {
	OrderPlaced(context.Context, Order) error
}

type IDSource interface {
	NewID() string
}

type Service struct {
	inventory Inventory
	orders    Repository
	events    Publisher
	ids       IDSource
}

func NewService(inventory Inventory, orders Repository, events Publisher, ids IDSource) *Service {
	return &Service{inventory: inventory, orders: orders, events: events, ids: ids}
}

// Place validates tenant ownership, reserves inventory, records the order, and publishes its event.
// Inventory and order persistence are independent; compensation failure is returned as an inconsistency.
func (s *Service) Place(ctx context.Context, itemID string, quantity int64) (Order, error) {
	tenantID, err := tenant.RequireID(ctx)
	if err != nil {
		return Order{}, err
	}
	if itemID == "" || quantity <= 0 {
		return Order{}, errors.New("item and positive quantity are required")
	}
	if err := s.inventory.Reserve(ctx, tenantID, itemID, quantity); err != nil {
		return Order{}, fmt.Errorf("reserve inventory: %w", err)
	}

	placed := Order{ID: s.ids.NewID(), TenantID: tenantID, ItemID: itemID, Quantity: quantity, Status: StatusPending}
	if err := s.orders.Create(ctx, placed); err != nil {
		if releaseErr := s.inventory.Release(ctx, tenantID, itemID, quantity); releaseErr != nil {
			return Order{}, fmt.Errorf("%w: create order: %v; release inventory: %v", ErrInconsistentReservation, err, releaseErr)
		}
		return Order{}, fmt.Errorf("create order after compensated reservation: %w", err)
	}
	if err := placed.Transition(StatusConfirmed); err != nil {
		return Order{}, err
	}
	if err := s.events.OrderPlaced(ctx, placed); err != nil {
		return placed, fmt.Errorf("publish placed order: %w", err)
	}
	return placed, nil
}
