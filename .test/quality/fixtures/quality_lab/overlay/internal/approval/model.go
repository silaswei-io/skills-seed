package approval

import "errors"

type State string

const (
	// StatePendingManager 表示等待直属负责人审批。
	StatePendingManager State = "pending_manager"
	// StatePendingFinance 表示高额申请等待财务复核。
	StatePendingFinance State = "pending_finance"
	// StateApproved 表示审批完成。
	StateApproved State = "approved"
	// StateRejected 表示申请已被拒绝。
	StateRejected State = "rejected"
)

var (
	ErrWrongReviewer     = errors.New("reviewer cannot act on the current stage")
	ErrTerminalRequest   = errors.New("approval request is already terminal")
	ErrRejectionRequired = errors.New("rejection reason is required")
)

type Request struct {
	ID              string
	Amount          int64
	State           State
	RejectionReason string
}

// Approve 根据金额和当前阶段推进审批，高额申请必须经过财务复核。
func (r *Request) Approve(role string, financeThreshold int64) error {
	if r.State == StateApproved || r.State == StateRejected {
		return ErrTerminalRequest
	}
	switch r.State {
	case StatePendingManager:
		if role != "manager" {
			return ErrWrongReviewer
		}
		if r.Amount >= financeThreshold {
			r.State = StatePendingFinance
			return nil
		}
		r.State = StateApproved
		return nil
	case StatePendingFinance:
		if role != "finance" {
			return ErrWrongReviewer
		}
		r.State = StateApproved
		return nil
	default:
		return ErrWrongReviewer
	}
}

// Reject 只允许当前阶段的处理人拒绝，并保留明确原因。
func (r *Request) Reject(role, reason string) error {
	if r.State == StateApproved || r.State == StateRejected {
		return ErrTerminalRequest
	}
	if reason == "" {
		return ErrRejectionRequired
	}
	if (r.State == StatePendingManager && role != "manager") || (r.State == StatePendingFinance && role != "finance") {
		return ErrWrongReviewer
	}
	r.State = StateRejected
	r.RejectionReason = reason
	return nil
}
