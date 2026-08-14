package payment

import (
	"context"
	"errors"
	"testing"
)

type ledgerStub struct {
	record    Record
	found     bool
	recordErr error
}

func (s *ledgerStub) FindByIdempotencyID(context.Context, string) (Record, bool, error) {
	return s.record, s.found, nil
}

func (s *ledgerStub) RecordSuccess(context.Context, Record) error {
	return s.recordErr
}

type gatewayStub struct {
	calls int
}

func (s *gatewayStub) Charge(context.Context, Charge) (string, error) {
	s.calls++
	return "provider-1", nil
}

func TestCaptureReusesMatchingOutcome(t *testing.T) {
	charge := Charge{PaymentID: "payment-1", OrderID: "order-1", Amount: 1200, IdempotencyID: "key-1"}
	ledger := &ledgerStub{record: Record{Charge: charge, ProviderID: "provider-1"}, found: true}
	gateway := &gatewayStub{}

	outcome, err := NewService(ledger, gateway).Capture(context.Background(), charge)

	if err != nil || !outcome.Reused || gateway.calls != 0 {
		t.Fatalf("unexpected result: outcome=%+v err=%v calls=%d", outcome, err, gateway.calls)
	}
}

func TestCaptureReportsUncertainOutcome(t *testing.T) {
	ledger := &ledgerStub{recordErr: errors.New("store unavailable")}
	gateway := &gatewayStub{}
	charge := Charge{PaymentID: "payment-1", OrderID: "order-1", Amount: 1200, IdempotencyID: "key-1"}

	outcome, err := NewService(ledger, gateway).Capture(context.Background(), charge)

	if !errors.Is(err, ErrOutcomeUncertain) || outcome.ProviderID == "" {
		t.Fatalf("expected uncertain provider outcome, outcome=%+v err=%v", outcome, err)
	}
}
