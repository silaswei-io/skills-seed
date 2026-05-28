// Package domain 提供领域错误的单元测试
package domain

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

// ==================== DomainError 测试 ====================

func TestNewDomainError(t *testing.T) {
	cause := errors.New("underlying error")
	err := NewDomainError(ErrNotFound, "resource not found", cause)

	assert.Equal(t, ErrNotFound, err.Code, "Code should be ErrNotFound")
	assert.Equal(t, "resource not found", err.Message, "Message should be set")
	assert.Equal(t, cause, err.Cause, "Cause should be set")
	assert.NotNil(t, err.Context, "Context should be initialized (not nil)")
	assert.Equal(t, 0, len(err.Context), "Context should be empty map")
}

func TestNewDomainError_NilCause(t *testing.T) {
	err := NewDomainError(ErrInternal, "something failed", nil)

	assert.Equal(t, ErrInternal, err.Code)
	assert.Equal(t, "something failed", err.Message)
	assert.Nil(t, err.Cause, "Cause should be nil")
	assert.NotNil(t, err.Context, "Context should be initialized")
}

func TestDomainError_Error(t *testing.T) {
	tests := []struct {
		name    string
		err     *DomainError
		wantMsg string
	}{
		{
			name:    "with cause",
			err:     NewDomainError(ErrNotFound, "user not found", errors.New("db timeout")),
			wantMsg: "[NOT_FOUND] user not found: db timeout",
		},
		{
			name:    "without cause",
			err:     NewDomainError(ErrInvalid, "invalid input", nil),
			wantMsg: "[INVALID] invalid input",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.wantMsg, tt.err.Error())
		})
	}
}

func TestDomainError_Unwrap(t *testing.T) {
	t.Run("returns cause when present", func(t *testing.T) {
		cause := errors.New("root cause")
		err := NewDomainError(ErrInternal, "wrapped", cause)
		assert.Equal(t, cause, err.Unwrap())
	})

	t.Run("returns nil when no cause", func(t *testing.T) {
		err := NewDomainError(ErrInternal, "no cause", nil)
		assert.Nil(t, err.Unwrap())
	})
}

func TestDomainError_WithContext(t *testing.T) {
	err := NewDomainError(ErrInternal, "test error", nil)

	result := err.WithContext("key1", "value1").WithContext("key2", 42)

	assert.Equal(t, "value1", result.Context["key1"], "Context key1 should be set")
	assert.Equal(t, 42, result.Context["key2"], "Context key2 should be set")
	assert.Equal(t, 2, len(result.Context), "Context should have 2 entries")
}

func TestDomainError_WithContext_NilMap(t *testing.T) {
	err := &DomainError{
		Code:    ErrInternal,
		Message: "test",
		Context: nil,
	}

	result := err.WithContext("key", "value")

	assert.NotNil(t, result.Context, "Context map should be created")
	assert.Equal(t, "value", result.Context["key"], "Context key should be set")
}

func TestDomainError_WithContext_Chaining(t *testing.T) {
	err := NewDomainError(ErrConflict, "duplicate entry", nil)

	// WithContext 返回同一个指针，便于链式调用。
	result := err.WithContext("user_id", "abc123")
	assert.Same(t, err, result, "WithContext should return the same error pointer for chaining")
}

// ==================== 错误类型检查函数测试 ====================

func TestIsNotFound(t *testing.T) {
	tests := []struct {
		name   string
		err    error
		result bool
	}{
		{
			name:   "DomainError with ErrNotFound",
			err:    NewDomainError(ErrNotFound, "not found", nil),
			result: true,
		},
		{
			name:   "DomainError with ErrInternal",
			err:    NewDomainError(ErrInternal, "internal error", nil),
			result: false,
		},
		{
			name:   "nil error",
			err:    nil,
			result: false,
		},
		{
			name:   "standard error",
			err:    errors.New("some error"),
			result: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.result, IsNotFound(tt.err))
		})
	}
}

func TestIsTimeout(t *testing.T) {
	tests := []struct {
		name   string
		err    error
		result bool
	}{
		{
			name:   "DomainError with ErrTimeout",
			err:    NewDomainError(ErrTimeout, "operation timed out", nil),
			result: true,
		},
		{
			name:   "DomainError with ErrInternal",
			err:    NewDomainError(ErrInternal, "internal error", nil),
			result: false,
		},
		{
			name:   "nil error",
			err:    nil,
			result: false,
		},
		{
			name:   "standard error",
			err:    errors.New("some error"),
			result: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.result, IsTimeout(tt.err))
		})
	}
}

func TestIsInvalid(t *testing.T) {
	tests := []struct {
		name   string
		err    error
		result bool
	}{
		{
			name:   "DomainError with ErrInvalid",
			err:    NewDomainError(ErrInvalid, "bad input", nil),
			result: true,
		},
		{
			name:   "DomainError with ErrNotFound",
			err:    NewDomainError(ErrNotFound, "not found", nil),
			result: false,
		},
		{
			name:   "nil error",
			err:    nil,
			result: false,
		},
		{
			name:   "standard error",
			err:    errors.New("some error"),
			result: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.result, IsInvalid(tt.err))
		})
	}
}

func TestIsConflict(t *testing.T) {
	tests := []struct {
		name   string
		err    error
		result bool
	}{
		{
			name:   "DomainError with ErrConflict",
			err:    NewDomainError(ErrConflict, "conflict", nil),
			result: true,
		},
		{
			name:   "DomainError with ErrInternal",
			err:    NewDomainError(ErrInternal, "internal error", nil),
			result: false,
		},
		{
			name:   "nil error",
			err:    nil,
			result: false,
		},
		{
			name:   "standard error",
			err:    errors.New("some error"),
			result: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.result, IsConflict(tt.err))
		})
	}
}

func TestIsInternal(t *testing.T) {
	tests := []struct {
		name   string
		err    error
		result bool
	}{
		{
			name:   "DomainError with ErrInternal",
			err:    NewDomainError(ErrInternal, "internal error", nil),
			result: true,
		},
		{
			name:   "DomainError with ErrNotFound",
			err:    NewDomainError(ErrNotFound, "not found", nil),
			result: false,
		},
		{
			name:   "nil error",
			err:    nil,
			result: false,
		},
		{
			name:   "standard error",
			err:    errors.New("some error"),
			result: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.result, IsInternal(tt.err))
		})
	}
}

func TestIsAIServiceError(t *testing.T) {
	tests := []struct {
		name   string
		err    error
		result bool
	}{
		{
			name:   "DomainError with ErrAIService",
			err:    NewDomainError(ErrAIService, "ai service unavailable", nil),
			result: true,
		},
		{
			name:   "DomainError with ErrInternal",
			err:    NewDomainError(ErrInternal, "internal error", nil),
			result: false,
		},
		{
			name:   "nil error",
			err:    nil,
			result: false,
		},
		{
			name:   "standard error",
			err:    errors.New("some error"),
			result: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.result, IsAIServiceError(tt.err))
		})
	}
}

// ==================== 工具函数测试 ====================

func TestGetErrorCode(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		wantCode ErrorCode
	}{
		{
			name:     "DomainError returns its code",
			err:      NewDomainError(ErrNotFound, "not found", nil),
			wantCode: ErrNotFound,
		},
		{
			name:     "DomainError with different code",
			err:      NewDomainError(ErrTimeout, "timeout", nil),
			wantCode: ErrTimeout,
		},
		{
			name:     "standard error returns ErrInternal",
			err:      errors.New("standard error"),
			wantCode: ErrInternal,
		},
		{
			name:     "wrapped DomainError returns correct code",
			err:      fmt.Errorf("wrapped: %w", NewDomainError(ErrConflict, "conflict", nil)),
			wantCode: ErrConflict,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.wantCode, GetErrorCode(tt.err))
		})
	}
}

func TestGetErrorContext(t *testing.T) {
	t.Run("DomainError with context returns map", func(t *testing.T) {
		err := NewDomainError(ErrInternal, "test", nil)
		err.WithContext("request_id", "abc-123")
		err.WithContext("user_id", 42)

		ctx := GetErrorContext(err)
		assert.NotNil(t, ctx, "Context should not be nil")
		assert.Equal(t, "abc-123", ctx["request_id"], "request_id should be in context")
		assert.Equal(t, 42, ctx["user_id"], "user_id should be in context")
	})

	t.Run("DomainError without context returns empty map", func(t *testing.T) {
		err := NewDomainError(ErrInternal, "test", nil)

		ctx := GetErrorContext(err)
		assert.NotNil(t, ctx, "Context should not be nil (empty map)")
		assert.Equal(t, 0, len(ctx), "Context should be empty")
	})

	t.Run("standard error returns nil", func(t *testing.T) {
		err := errors.New("standard error")

		ctx := GetErrorContext(err)
		assert.Nil(t, ctx, "Context should be nil for standard error")
	})
}

// ==================== 边界场景测试 ====================

func TestDomainError_Error_AllErrorCodes(t *testing.T) {
	codes := []ErrorCode{
		ErrNotFound, ErrInvalid, ErrConflict, ErrUnauthorized,
		ErrInternal, ErrTimeout, ErrUnavailable,
		ErrGitOperation, ErrAIService, ErrDatabase,
	}

	for _, code := range codes {
		t.Run(string(code), func(t *testing.T) {
			err := NewDomainError(code, "test message", nil)
			expected := "[" + string(code) + "] test message"
			assert.Equal(t, expected, err.Error())
		})
	}
}

func TestDomainError_Unwrap_WithErrorsAs(t *testing.T) {
	cause := errors.New("root cause")
	err := NewDomainError(ErrInternal, "wrapped", cause)

	var domainErr *DomainError
	assert.True(t, errors.As(err, &domainErr), "errors.As should work with DomainError")
	assert.Equal(t, ErrInternal, domainErr.Code)
}

func TestDomainError_Unwrap_WithErrorsIs(t *testing.T) {
	innerErr := errors.New("inner")
	err := NewDomainError(ErrInternal, "outer", innerErr)

	assert.True(t, errors.Is(err, innerErr), "errors.Is should find the inner error through Unwrap")
}

func TestAllTypeCheckFunctions_WithWrappedDomainError(t *testing.T) {
	inner := NewDomainError(ErrNotFound, "not found", nil)
	wrapped := fmt.Errorf("service call failed: %w", inner)

	assert.True(t, IsNotFound(wrapped), "IsNotFound should work through wrapped error")
	assert.False(t, IsTimeout(wrapped), "IsTimeout should be false for wrapped ErrNotFound")
}

func TestGetErrorCode_WrappedError(t *testing.T) {
	inner := NewDomainError(ErrAIService, "ai failed", nil)
	wrapped := fmt.Errorf("service: %w", inner)

	assert.Equal(t, ErrAIService, GetErrorCode(wrapped), "GetErrorCode should unwrap to find DomainError code")
}
