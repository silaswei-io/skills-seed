package subscription

import (
	"context"
	"errors"
	"testing"
	"time"
)

type chargerStub struct {
	err error
}

func (s chargerStub) ChargeRenewal(context.Context, string) error {
	return s.err
}

type repositoryStub struct {
	saved Subscription
}

func (s *repositoryStub) Save(_ context.Context, subscription Subscription) error {
	s.saved = subscription
	return nil
}

func TestRenewExtendsUnexpiredSubscriptionFromExpiry(t *testing.T) {
	now := time.Date(2026, 8, 13, 10, 0, 0, 0, time.UTC)
	expires := now.Add(48 * time.Hour)
	repo := &repositoryStub{}
	service := NewService(chargerStub{}, repo, func() time.Time { return now })

	updated, err := service.Renew(context.Background(), Subscription{ID: "sub-1", State: StateActive, ExpiresAt: expires}, 24*time.Hour)

	if err != nil || !updated.ExpiresAt.Equal(expires.Add(24*time.Hour)) || updated.State != StateActive {
		t.Fatalf("unexpected renewal: subscription=%+v err=%v", updated, err)
	}
}

func TestRenewalFailureKeepsServiceDuringGracePeriod(t *testing.T) {
	now := time.Date(2026, 8, 13, 10, 0, 0, 0, time.UTC)
	service := NewService(chargerStub{err: errors.New("declined")}, &repositoryStub{}, func() time.Time { return now })

	updated, err := service.Renew(context.Background(), Subscription{ID: "sub-1", State: StateActive, GraceUntil: now.Add(time.Hour)}, 24*time.Hour)

	if err == nil || updated.State != StatePastDue {
		t.Fatalf("expected past-due grace state, subscription=%+v err=%v", updated, err)
	}
}

func TestRenewalFailureSuspendsAfterGracePeriod(t *testing.T) {
	now := time.Date(2026, 8, 13, 10, 0, 0, 0, time.UTC)
	service := NewService(chargerStub{err: errors.New("declined")}, &repositoryStub{}, func() time.Time { return now })

	updated, err := service.Renew(context.Background(), Subscription{ID: "sub-1", State: StatePastDue, GraceUntil: now.Add(-time.Second)}, 24*time.Hour)

	if err == nil || updated.State != StateSuspended {
		t.Fatalf("expected suspended state, subscription=%+v err=%v", updated, err)
	}
}
