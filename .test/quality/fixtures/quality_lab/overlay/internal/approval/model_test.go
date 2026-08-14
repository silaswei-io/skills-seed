package approval

import (
	"errors"
	"testing"
)

func TestHighAmountRequiresFinanceReview(t *testing.T) {
	request := Request{ID: "request-1", Amount: 5000, State: StatePendingManager}

	if err := request.Approve("manager", 3000); err != nil {
		t.Fatal(err)
	}
	if request.State != StatePendingFinance {
		t.Fatalf("got %s, want %s", request.State, StatePendingFinance)
	}
	if err := request.Approve("manager", 3000); !errors.Is(err, ErrWrongReviewer) {
		t.Fatalf("got %v, want %v", err, ErrWrongReviewer)
	}
	if err := request.Approve("finance", 3000); err != nil || request.State != StateApproved {
		t.Fatalf("unexpected final state: state=%s err=%v", request.State, err)
	}
}
