package refund

import (
	"context"
	"errors"
	"fmt"
)

var (
	// ErrIdempotencyConflict 表示幂等键已绑定到另一笔退款请求。
	ErrIdempotencyConflict = errors.New("idempotency key belongs to another refund")
	// ErrAmountExceeded 表示累计退款金额超过原支付金额。
	ErrAmountExceeded = errors.New("refund amount exceeds remaining captured amount")
	// ErrOutcomeUncertain 表示渠道已退款但本地账本未能确认结果。
	ErrOutcomeUncertain = errors.New("refund succeeded but its local outcome is uncertain")
)

type Request struct {
	RefundID      string
	PaymentID     string
	Amount        int64
	IdempotencyID string
}

type Payment struct {
	ID       string
	Captured int64
}

type Record struct {
	Request    Request
	ProviderID string
}

type Outcome struct {
	ProviderID string
	Reused     bool
}

type Ledger interface {
	Payment(context.Context, string) (Payment, error)
	RefundedAmount(context.Context, string) (int64, error)
	FindByIdempotencyID(context.Context, string) (Record, bool, error)
	RecordSuccess(context.Context, Record) error
}

type Gateway interface {
	Refund(context.Context, Request) (string, error)
}

type Service struct {
	ledger  Ledger
	gateway Gateway
}

func NewService(ledger Ledger, gateway Gateway) *Service {
	return &Service{ledger: ledger, gateway: gateway}
}

// Issue 先复用相同幂等请求，再按原支付的累计已退金额校验本次可退额度。
func (s *Service) Issue(ctx context.Context, request Request) (Outcome, error) {
	if request.RefundID == "" || request.PaymentID == "" || request.Amount <= 0 || request.IdempotencyID == "" {
		return Outcome{}, errors.New("refund, payment, positive amount, and idempotency key are required")
	}
	existing, found, err := s.ledger.FindByIdempotencyID(ctx, request.IdempotencyID)
	if err != nil {
		return Outcome{}, fmt.Errorf("find refund outcome: %w", err)
	}
	if found {
		if existing.Request.RefundID != request.RefundID || existing.Request.PaymentID != request.PaymentID || existing.Request.Amount != request.Amount {
			return Outcome{}, ErrIdempotencyConflict
		}
		return Outcome{ProviderID: existing.ProviderID, Reused: true}, nil
	}
	payment, err := s.ledger.Payment(ctx, request.PaymentID)
	if err != nil {
		return Outcome{}, fmt.Errorf("load captured payment: %w", err)
	}
	refunded, err := s.ledger.RefundedAmount(ctx, request.PaymentID)
	if err != nil {
		return Outcome{}, fmt.Errorf("load refunded amount: %w", err)
	}
	if request.Amount > payment.Captured-refunded {
		return Outcome{}, ErrAmountExceeded
	}
	providerID, err := s.gateway.Refund(ctx, request)
	if err != nil {
		return Outcome{}, fmt.Errorf("refund payment: %w", err)
	}
	if err := s.ledger.RecordSuccess(ctx, Record{Request: request, ProviderID: providerID}); err != nil {
		return Outcome{ProviderID: providerID}, fmt.Errorf("%w: record refund success: %v", ErrOutcomeUncertain, err)
	}
	return Outcome{ProviderID: providerID}, nil
}
