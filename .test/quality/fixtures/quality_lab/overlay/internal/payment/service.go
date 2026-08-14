package payment

import (
	"context"
	"errors"
	"fmt"
)

var (
	ErrIdempotencyConflict = errors.New("idempotency key belongs to another payment")
	ErrOutcomeUncertain    = errors.New("payment succeeded but its local outcome is uncertain")
)

type Charge struct {
	PaymentID     string
	OrderID       string
	Amount        int64
	IdempotencyID string
}

type Outcome struct {
	ProviderID string
	Reused     bool
}

type Record struct {
	Charge     Charge
	ProviderID string
}

type Ledger interface {
	FindByIdempotencyID(context.Context, string) (Record, bool, error)
	RecordSuccess(context.Context, Record) error
}

type Gateway interface {
	Charge(context.Context, Charge) (string, error)
}

type Service struct {
	ledger  Ledger
	gateway Gateway
}

func NewService(ledger Ledger, gateway Gateway) *Service {
	return &Service{ledger: ledger, gateway: gateway}
}

// Capture 使用幂等键复用已完成结果，并显式暴露外部扣款成功但本地记录失败的不确定状态。
func (s *Service) Capture(ctx context.Context, charge Charge) (Outcome, error) {
	if charge.PaymentID == "" || charge.OrderID == "" || charge.Amount <= 0 || charge.IdempotencyID == "" {
		return Outcome{}, errors.New("payment, order, positive amount, and idempotency key are required")
	}

	existing, found, err := s.ledger.FindByIdempotencyID(ctx, charge.IdempotencyID)
	if err != nil {
		return Outcome{}, fmt.Errorf("find payment outcome: %w", err)
	}
	if found {
		if existing.Charge.PaymentID != charge.PaymentID || existing.Charge.OrderID != charge.OrderID || existing.Charge.Amount != charge.Amount {
			return Outcome{}, ErrIdempotencyConflict
		}
		return Outcome{ProviderID: existing.ProviderID, Reused: true}, nil
	}

	providerID, err := s.gateway.Charge(ctx, charge)
	if err != nil {
		return Outcome{}, fmt.Errorf("charge payment: %w", err)
	}
	if err := s.ledger.RecordSuccess(ctx, Record{Charge: charge, ProviderID: providerID}); err != nil {
		return Outcome{ProviderID: providerID}, fmt.Errorf("%w: record payment success: %v", ErrOutcomeUncertain, err)
	}
	return Outcome{ProviderID: providerID}, nil
}
