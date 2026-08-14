package pricing

import "testing"

func TestQuoteBestDoesNotStackBenefits(t *testing.T) {
	quote, err := QuoteBest(QuoteRequest{
		BasePrice:       1000,
		Quantity:        2,
		MemberBasis:     1000,
		CampaignBasis:   1500,
		CouponAmount:    500,
		MaximumDiscount: 400,
	})

	if err != nil {
		t.Fatal(err)
	}
	if quote.AppliedBenefit != "coupon" || quote.Discount != 400 || quote.Payable != 1600 {
		t.Fatalf("unexpected quote: %+v", quote)
	}
}
