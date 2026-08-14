package refund

import (
	"context"
	"errors"
	"testing"
)

type ledgerStub struct {
	payment   Payment
	refunded  int64
	record    Record
	found     bool
	recordErr error
}

func (s ledgerStub) Payment(context.Context, string) (Payment, error) {
	return s.payment, nil
}

func (s ledgerStub) RefundedAmount(context.Context, string) (int64, error) {
	return s.refunded, nil
}

func (s ledgerStub) FindByIdempotencyID(context.Context, string) (Record, bool, error) {
	return s.record, s.found, nil
}

func (s ledgerStub) RecordSuccess(context.Context, Record) error {
	return s.recordErr
}

type gatewayStub struct {
	calls int
}

func (s *gatewayStub) Refund(context.Context, Request) (string, error) {
	s.calls++
	return "provider-refund-1", nil
}

func TestIssueRejectsAmountAboveRemainingCapture(t *testing.T) {
	gateway := &gatewayStub{}
	service := NewService(ledgerStub{payment: Payment{ID: "payment-1", Captured: 1000}, refunded: 700}, gateway)

	_, err := service.Issue(context.Background(), Request{RefundID: "refund-2", PaymentID: "payment-1", Amount: 301, IdempotencyID: "key-2"})

	if !errors.Is(err, ErrAmountExceeded) || gateway.calls != 0 {
		t.Fatalf("expected remaining amount rejection, err=%v calls=%d", err, gateway.calls)
	}
}

func TestIssueReusesMatchingResultBeforeAmountCheck(t *testing.T) {
	request := Request{RefundID: "refund-1", PaymentID: "payment-1", Amount: 400, IdempotencyID: "key-1"}
	gateway := &gatewayStub{}
	service := NewService(ledgerStub{
		payment:  Payment{ID: "payment-1", Captured: 1000},
		refunded: 1000,
		record:   Record{Request: request, ProviderID: "provider-refund-1"},
		found:    true,
	}, gateway)

	outcome, err := service.Issue(context.Background(), request)

	if err != nil || !outcome.Reused || gateway.calls != 0 {
		t.Fatalf("unexpected reuse result: outcome=%+v err=%v calls=%d", outcome, err, gateway.calls)
	}
}

func TestIssueReturnsProviderResultWhenLedgerWriteFails(t *testing.T) {
	service := NewService(
		ledgerStub{payment: Payment{ID: "payment-1", Captured: 1000}, recordErr: errors.New("store unavailable")},
		&gatewayStub{},
	)

	outcome, err := service.Issue(context.Background(), Request{RefundID: "refund-1", PaymentID: "payment-1", Amount: 400, IdempotencyID: "key-1"})

	if !errors.Is(err, ErrOutcomeUncertain) || outcome.ProviderID == "" {
		t.Fatalf("expected uncertain provider result, outcome=%+v err=%v", outcome, err)
	}
}
