// Package domain 提供领域错误的单元测试
package domain

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNewDomainError(t *testing.T) {
	cause := errors.New("underlying error")
	err := NewDomainError(ErrAIService, "ai failed", cause)

	require.Equal(t, ErrAIService, err.Code)
	require.Equal(t, "ai failed", err.Message)
	require.Equal(t, cause, err.Cause)
	require.Equal(t, "[AI_ERROR] ai failed: underlying error", err.Error())
	require.ErrorIs(t, err, cause)
}

func TestDomainErrorWithoutCause(t *testing.T) {
	err := NewDomainError(ErrAIService, "ai failed", nil)

	require.Equal(t, "[AI_ERROR] ai failed", err.Error())
	require.NoError(t, err.Unwrap())
}
