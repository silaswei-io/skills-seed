package pricing

import "errors"

var ErrInvalidPrice = errors.New("base price and quantity must be positive")

type QuoteRequest struct {
	BasePrice       int64
	Quantity        int64
	MemberBasis     int64
	CampaignBasis   int64
	CouponAmount    int64
	MaximumDiscount int64
}

type Quote struct {
	Subtotal       int64
	Discount       int64
	Payable        int64
	AppliedBenefit string
}

// QuoteBest 只选择会员折扣、活动折扣或优惠券中价值最高的一项，并受总折扣上限约束。
func QuoteBest(request QuoteRequest) (Quote, error) {
	if request.BasePrice <= 0 || request.Quantity <= 0 {
		return Quote{}, ErrInvalidPrice
	}
	subtotal := request.BasePrice * request.Quantity
	benefit, discount := bestBenefit(subtotal, request)
	if request.MaximumDiscount > 0 && discount > request.MaximumDiscount {
		discount = request.MaximumDiscount
	}
	if discount > subtotal {
		discount = subtotal
	}
	return Quote{Subtotal: subtotal, Discount: discount, Payable: subtotal - discount, AppliedBenefit: benefit}, nil
}

func bestBenefit(subtotal int64, request QuoteRequest) (string, int64) {
	benefits := []struct {
		name   string
		amount int64
	}{
		{name: "member", amount: subtotal * request.MemberBasis / 10000},
		{name: "campaign", amount: subtotal * request.CampaignBasis / 10000},
		{name: "coupon", amount: request.CouponAmount},
	}
	best := benefits[0]
	for _, benefit := range benefits[1:] {
		if benefit.amount > best.amount {
			best = benefit
		}
	}
	return best.name, best.amount
}
