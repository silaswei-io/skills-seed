package subscription

import (
	"context"
	"errors"
	"fmt"
	"time"
)

type State string

const (
	// StateActive 表示订阅处于正常服务期。
	StateActive State = "active"
	// StatePastDue 表示续费失败但仍处于宽限期。
	StatePastDue State = "past_due"
	// StateSuspended 表示宽限期结束后服务已暂停。
	StateSuspended State = "suspended"
)

var ErrInvalidTerm = errors.New("renewal term must be positive")

type Subscription struct {
	ID         string
	State      State
	ExpiresAt  time.Time
	GraceUntil time.Time
}

type Charger interface {
	ChargeRenewal(context.Context, string) error
}

type Repository interface {
	Save(context.Context, Subscription) error
}

type Service struct {
	charger Charger
	repo    Repository
	now     func() time.Time
}

func NewService(charger Charger, repo Repository, now func() time.Time) *Service {
	return &Service{charger: charger, repo: repo, now: now}
}

// Renew 成功时从当前到期日续期；失败时仅在宽限期内保留服务，逾期则暂停。
func (s *Service) Renew(ctx context.Context, subscription Subscription, term time.Duration) (Subscription, error) {
	if subscription.ID == "" || term <= 0 {
		return subscription, ErrInvalidTerm
	}
	now := s.now()
	if err := s.charger.ChargeRenewal(ctx, subscription.ID); err != nil {
		if now.Before(subscription.GraceUntil) || now.Equal(subscription.GraceUntil) {
			subscription.State = StatePastDue
		} else {
			subscription.State = StateSuspended
		}
		if saveErr := s.repo.Save(ctx, subscription); saveErr != nil {
			return subscription, fmt.Errorf("save failed renewal state: %w", saveErr)
		}
		return subscription, fmt.Errorf("charge renewal: %w", err)
	}
	base := subscription.ExpiresAt
	if base.Before(now) {
		base = now
	}
	subscription.ExpiresAt = base.Add(term)
	subscription.GraceUntil = time.Time{}
	subscription.State = StateActive
	if err := s.repo.Save(ctx, subscription); err != nil {
		return subscription, fmt.Errorf("save renewed subscription: %w", err)
	}
	return subscription, nil
}
