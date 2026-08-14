package order

import "errors"

type Status string

const (
	StatusPending   Status = "pending"
	StatusConfirmed Status = "confirmed"
	StatusCancelled Status = "cancelled"
)

var ErrInvalidTransition = errors.New("invalid order status transition")

type Order struct {
	ID       string
	TenantID string
	ItemID   string
	Quantity int64
	Status   Status
}

// Transition applies the complete set of allowed order status changes.
func (o *Order) Transition(next Status) error {
	allowed := o.Status == StatusPending && (next == StatusConfirmed || next == StatusCancelled)
	if !allowed {
		return ErrInvalidTransition
	}
	o.Status = next
	return nil
}
