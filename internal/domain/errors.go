// Package domain 提供核心领域模型和业务规则
package domain

import (
	"fmt"
)

// ErrorCode 错误代码
type ErrorCode string

const (
	ErrAIService ErrorCode = "AI_ERROR" // AI 服务错误
)

// DomainError 领域错误
type DomainError struct {
	Code    ErrorCode // 错误代码
	Message string    // 错误消息
	Cause   error     // 原始错误
}

// Error 实现 error 接口
func (e *DomainError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("[%s] %s: %v", e.Code, e.Message, e.Cause)
	}
	return fmt.Sprintf("[%s] %s", e.Code, e.Message)
}

// Unwrap 实现错误包装
func (e *DomainError) Unwrap() error {
	return e.Cause
}

// NewDomainError 创建领域错误
func NewDomainError(code ErrorCode, message string, cause error) *DomainError {
	return &DomainError{
		Code:    code,
		Message: message,
		Cause:   cause,
	}
}
